package service

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"

	"inkdrop/config"
	"inkdrop/model"
	"inkdrop/storage/filesystem"

	"github.com/google/uuid"
)

const (
	// MaxUploadSize is the maximum file upload size in bytes (500 MB).
	MaxUploadSize int64 = 500 * 1024 * 1024
)

var safeNameRegex = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// FileService handles file operations within drops.
type FileService struct {
	store *filesystem.Store
	perm  *PermissionService
}

// NewFileService creates a new FileService.
func NewFileService(store *filesystem.Store, perm *PermissionService) *FileService {
	return &FileService{
		store: store,
		perm:  perm,
	}
}

// ValidateName validates a filename or directory name.
func ValidateName(name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("name is required")
	}
	if strings.ContainsAny(name, "/\\") {
		return errors.New("name cannot contain path separators")
	}
	if name == "." || name == ".." {
		return errors.New("invalid name")
	}
	if len(name) > 255 {
		return errors.New("name is too long (max 255 characters)")
	}
	return nil
}

// ListFiles returns files at a given path within a drop.
// If relPath is empty or "/", returns all files in the drop.
// Otherwise, returns only files directly under the given directory path.
func (s *FileService) ListFiles(dropID, relPath string) ([]*model.File, error) {
	db := config.GetDB()

	relPath = normalizePath(relPath)

	if relPath == "" || relPath == "/" {
		return model.GetFilesByDrop(db, dropID)
	}

	// Get files directly under this path (exact prefix match at directory boundary)
	var files []*model.File
	all, err := model.GetFilesByDrop(db, dropID)
	if err != nil {
		return nil, err
	}
	prefix := strings.TrimSuffix(relPath, "/") + "/"
	for _, f := range all {
		// Match if the file path starts with the directory prefix
		// and the remaining portion doesn't contain a slash (direct child only)
		if strings.HasPrefix(f.Path, prefix) {
			remainder := strings.TrimPrefix(f.Path, prefix)
			if !strings.Contains(remainder, "/") {
				files = append(files, f)
			}
		}
	}
	return files, nil
}

// GetFile retrieves a file by ID.
func (s *FileService) GetFile(fileID string) (*model.File, error) {
	db := config.GetDB()
	return model.GetFileByID(db, fileID)
}

// GetFileInDrop retrieves a file by ID, verifying it belongs to the given drop.
func (s *FileService) GetFileInDrop(dropID, fileID string) (*model.File, error) {
	db := config.GetDB()
	file, err := model.GetFileByID(db, fileID)
	if err != nil {
		return nil, err
	}
	if file.DropID != dropID {
		return nil, errors.New("file not found in this drop")
	}
	return file, nil
}

// CreateDirectory creates a new directory within a drop.
func (s *FileService) CreateDirectory(dropID, name, parentPath string, createdBy int64) (*model.File, error) {
	db := config.GetDB()

	if err := ValidateName(name); err != nil {
		return nil, err
	}

	parentPath = normalizePath(parentPath)
	if parentPath == "/" {
		parentPath = ""
	}

	// Build full path
	fullPath := filepath.ToSlash(filepath.Join(parentPath, name))

	// Check for duplicate
	existing, err := model.GetFileByPath(db, dropID, fullPath)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("an item named %q already exists in this location", name)
	}

	fileID := uuid.New().String()

	// Create the directory in storage
	storageRelPath := filepath.Join("files", fullPath)
	if err := s.store.CreateDirectory(dropID, storageRelPath); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	// Create file record
	dir := &model.File{
		ID:        fileID,
		DropID:    dropID,
		Name:      name,
		Type:      model.FileTypeFolder,
		MimeType:  "inode/directory",
		Size:      0,
		Path:      fullPath,
		CreatedBy: createdBy,
	}

	if err := model.CreateFileRecord(db, dir); err != nil {
		return nil, fmt.Errorf("failed to create directory record: %w", err)
	}

	// Log activity
	model.LogActivity(db, &model.ActivityLog{
		DropID:     dropID,
		UserID:     createdBy,
		EventType:  model.EventFileCreated,
		TargetType: "folder",
		TargetID:   fileID,
	})

	return dir, nil
}

// UploadFile uploads a file to a drop.
func (s *FileService) UploadFile(dropID, name, parentPath string, createdBy int64, reader io.Reader, size int64) (*model.File, error) {
	db := config.GetDB()

	if err := ValidateName(name); err != nil {
		return nil, err
	}

	if size > MaxUploadSize {
		return nil, fmt.Errorf("file is too large (max %d bytes)", MaxUploadSize)
	}

	parentPath = normalizePath(parentPath)
	if parentPath == "/" {
		parentPath = ""
	}

	// Build full path
	fullPath := filepath.ToSlash(filepath.Join(parentPath, name))

	// Check for duplicate
	existing, err := model.GetFileByPath(db, dropID, fullPath)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("a file named %q already exists in this location", name)
	}

	// Read the file data (bounded by MaxUploadSize)
	if size < 0 {
		// Unknown size — read up to MaxUploadSize
		data, err := io.ReadAll(io.LimitReader(reader, MaxUploadSize+1))
		if err != nil {
			return nil, fmt.Errorf("failed to read upload: %w", err)
		}
		if int64(len(data)) > MaxUploadSize {
			return nil, fmt.Errorf("file is too large (max %d bytes)", MaxUploadSize)
		}
		return s.createFileData(dropID, name, parentPath, fullPath, createdBy, data)
	}

	// Known size — stream with limit
	data, err := io.ReadAll(io.LimitReader(reader, size))
	if err != nil {
		return nil, fmt.Errorf("failed to read upload: %w", err)
	}
	if int64(len(data)) != size {
		return nil, fmt.Errorf("upload truncated: expected %d bytes, got %d", size, len(data))
	}
	return s.createFileData(dropID, name, parentPath, fullPath, createdBy, data)
}

func (s *FileService) createFileData(dropID, name, parentPath, fullPath string, createdBy int64, data []byte) (*model.File, error) {
	db := config.GetDB()

	fileID := uuid.New().String()

	// Detect MIME type
	mimeType := http.DetectContentType(data)
	if mimeType == "application/octet-stream" {
		// Try to infer from extension
		ext := strings.ToLower(filepath.Ext(name))
		if inferred := mime.TypeByExtension(ext); inferred != "" {
			mimeType = inferred
		}
	}

	file := &model.File{
		ID:        fileID,
		DropID:    dropID,
		Name:      name,
		Type:      model.FileTypeFile,
		MimeType:  mimeType,
		Size:      int64(len(data)),
		Path:      fullPath,
		CreatedBy: createdBy,
	}

	if err := model.CreateFileRecord(db, file); err != nil {
		return nil, fmt.Errorf("failed to create file record: %w", err)
	}

	// Write data to filesystem
	storageRelPath := filepath.Join("files", fullPath)
	if err := s.store.WriteFile(dropID, storageRelPath, data); err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	// Compute checksum
	checksum, err := s.store.CalculateChecksum(dropID, storageRelPath)
	if err == nil {
		file.Checksum = checksum
		model.UpdateFileRecord(db, file)
	}

	// Log activity
	model.LogActivity(db, &model.ActivityLog{
		DropID:     dropID,
		UserID:     createdBy,
		EventType:  model.EventFileCreated,
		TargetType: "file",
		TargetID:   fileID,
	})

	return file, nil
}

// CreateFile creates a new file record from JSON content (legacy method).
func (s *FileService) CreateFile(dropID, name, fileType, relPath string, createdBy int64, data []byte) (*model.File, error) {
	if err := ValidateName(name); err != nil {
		return nil, err
	}

	relPath = normalizePath(relPath)
	if relPath == "/" {
		relPath = ""
	}

	fullPath := filepath.ToSlash(filepath.Join(relPath, name))
	ft := model.FileType(fileType)
	if ft == "" {
		ft = model.FileTypeFile
	}

	// If it's a folder type, delegate to CreateDirectory
	if ft == model.FileTypeFolder {
		return s.CreateDirectory(dropID, name, relPath, createdBy)
	}

	file := &model.File{
		ID:        uuid.New().String(),
		DropID:    dropID,
		Name:      name,
		Type:      ft,
		MimeType:  http.DetectContentType(data),
		Size:      int64(len(data)),
		Path:      fullPath,
		CreatedBy: createdBy,
	}

	db := config.GetDB()
	if err := model.CreateFileRecord(db, file); err != nil {
		return nil, fmt.Errorf("failed to create file record: %w", err)
	}

	if len(data) > 0 {
		storageRelPath := filepath.Join("files", fullPath)
		if err := s.store.WriteFile(dropID, storageRelPath, data); err != nil {
			return nil, fmt.Errorf("failed to write file: %w", err)
		}
		checksum, err := s.store.CalculateChecksum(dropID, storageRelPath)
		if err == nil {
			file.Checksum = checksum
			model.UpdateFileRecord(db, file)
		}
	}

	model.LogActivity(db, &model.ActivityLog{
		DropID:     dropID,
		UserID:     createdBy,
		EventType:  model.EventFileCreated,
		TargetType: "file",
		TargetID:   file.ID,
	})

	return file, nil
}

// ReadFile reads file content from storage.
func (s *FileService) ReadFile(dropID, fileID string) ([]byte, error) {
	file, err := s.GetFileInDrop(dropID, fileID)
	if err != nil {
		return nil, err
	}
	if file.Type == model.FileTypeFolder {
		return nil, errors.New("cannot read a directory")
	}
	storageRelPath := filepath.Join("files", file.Path)
	return s.store.ReadFile(dropID, storageRelPath)
}

// StreamFile streams file content from storage to the given writer.
func (s *FileService) StreamFile(dropID, fileID string, w io.Writer) error {
	file, err := s.GetFileInDrop(dropID, fileID)
	if err != nil {
		return err
	}
	if file.Type == model.FileTypeFolder {
		return errors.New("cannot stream a directory")
	}
	storageRelPath := filepath.Join("files", file.Path)
	data, err := s.store.ReadFile(dropID, storageRelPath)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

// UpdateFile updates file content and creates a version snapshot if content changed.
func (s *FileService) UpdateFile(dropID, fileID string, data []byte, userID int64, versionMessage string) (*model.File, error) {
	db := config.GetDB()

	file, err := s.GetFileInDrop(dropID, fileID)
	if err != nil {
		return nil, err
	}

	if file.Type == model.FileTypeFolder {
		return nil, errors.New("cannot update a directory")
	}

	storageRelPath := filepath.Join("files", file.Path)

	// Check if content actually changed
	if len(data) > 0 {
		// Save current content as version before overwriting
		existingData, err := s.store.ReadFile(dropID, storageRelPath)
		if err == nil && len(existingData) > 0 {
			// Get latest version number
			latestVersion, _ := model.GetLatestFileVersion(db, fileID)

			// Save version snapshot
			if err := s.store.SaveVersion(dropID, fileID, storageRelPath, latestVersion+1); err == nil {
				model.CreateFileVersion(db, &model.FileVersion{
					FileID:        fileID,
					VersionNumber: latestVersion + 1,
					Checksum:      file.Checksum,
					Size:          file.Size,
					CreatedBy:     userID,
					Message:       versionMessage,
				})
			}
		}

		// Write new content
		if err := s.store.WriteFile(dropID, storageRelPath, data); err != nil {
			return nil, fmt.Errorf("failed to write file: %w", err)
		}

		// Update checksum and size
		checksum, err := s.store.CalculateChecksum(dropID, storageRelPath)
		if err == nil {
			file.Checksum = checksum
		}
		file.Size = int64(len(data))
		file.MimeType = http.DetectContentType(data)
	}

	// Update record
	if err := model.UpdateFileRecord(db, file); err != nil {
		return nil, err
	}

	// Log activity
	model.LogActivity(db, &model.ActivityLog{
		DropID:     dropID,
		UserID:     userID,
		EventType:  model.EventFileModified,
		TargetType: "file",
		TargetID:   fileID,
	})

	return file, nil
}

// TrashFile moves a file or folder to trash.
func (s *FileService) TrashFile(dropID, fileID string, userID int64) error {
	db := config.GetDB()

	file, err := s.GetFileInDrop(dropID, fileID)
	if err != nil {
		return err
	}

	// Check if already in trash
	if strings.HasPrefix(file.Path, ".Trash-1000/") {
		return errors.New("item is already in trash")
	}

	// Move on filesystem
	oldStoragePath := filepath.Join("files", file.Path)
	trashBase := filepath.Join("files", ".Trash-1000", "files")

	// Generate trash path with fallback for name collisions
	trashRelPath, err := s.nextAvailableTrashPath(dropID, trashBase, file.Name)
	if err != nil {
		return err
	}

	if err := s.store.MoveFile(dropID, oldStoragePath, trashRelPath); err != nil {
		return fmt.Errorf("failed to move to trash: %w", err)
	}

	// If it's a folder, also update all child file records
	if file.Type == model.FileTypeFolder {
		return s.updateChildrenPathsForTrash(dropID, file, trashRelPath)
	}

	// Update file record
	oldPath := file.Path
	file.Path = strings.TrimPrefix(filepath.ToSlash(trashRelPath), "files/")
	if err := model.UpdateFileRecord(db, file); err != nil {
		return err
	}

	// Log activity
	model.LogActivity(db, &model.ActivityLog{
		DropID:     dropID,
		UserID:     userID,
		EventType:  model.EventFileDeleted,
		TargetType: "file",
		TargetID:   fileID,
		Metadata:   fmt.Sprintf(`{"trashed": true, "original_path": %q}`, oldPath),
	})

	return nil
}

func (s *FileService) updateChildrenPathsForTrash(dropID string, folder *model.File, newFolderPath string) error {
	db := config.GetDB()
	// Update all child file records
	all, err := model.GetFilesByDrop(db, dropID)
	if err != nil {
		return err
	}
	prefix := folder.Path + "/"
	newPrefix := strings.TrimPrefix(filepath.ToSlash(newFolderPath), "files/") + "/"
	for _, f := range all {
		if strings.HasPrefix(f.Path, prefix) {
			f.Path = newPrefix + strings.TrimPrefix(f.Path, prefix)
			if err := model.UpdateFileRecord(db, f); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *FileService) nextAvailableTrashPath(dropID, trashBase, name string) (string, error) {
	target := filepath.Join(trashBase, name)
	exists, err := s.store.FileExists(dropID, target)
	if err != nil || !exists {
		return target, nil
	}
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	for i := 1; i < 1000; i++ {
		candidate := filepath.Join(trashBase, fmt.Sprintf("%s (%d)%s", base, i, ext))
		exists, err := s.store.FileExists(dropID, candidate)
		if err != nil || !exists {
			return candidate, nil
		}
	}
	return "", errors.New("could not find an available trash path")
}

// RestoreFile restores a file or folder from trash.
func (s *FileService) RestoreFile(dropID, fileID string, userID int64) (*model.File, error) {
	db := config.GetDB()

	file, err := s.GetFileInDrop(dropID, fileID)
	if err != nil {
		return nil, err
	}

	// Check it's in trash
	if !strings.HasPrefix(file.Path, ".Trash-1000/") {
		return nil, errors.New("item is not in trash")
	}

	// Determine original path (strip trash prefix)
	currentTrashPath := file.Path
	originalPath := strings.TrimPrefix(currentTrashPath, ".Trash-1000/files/")

	// Check if original path is available
	if _, err := model.GetFileByPath(db, dropID, originalPath); err == nil {
		// Original location has a conflict — append " (restored)"
		ext := filepath.Ext(originalPath)
		base := strings.TrimSuffix(originalPath, ext)
		for i := 1; i < 1000; i++ {
			candidate := fmt.Sprintf("%s (restored %d)%s", base, i, ext)
			if _, err := model.GetFileByPath(db, dropID, candidate); err != nil {
				originalPath = candidate
				break
			}
		}
	}

	// Move on filesystem
	oldStoragePath := filepath.Join("files", currentTrashPath)
	newStoragePath := filepath.Join("files", originalPath)
	if err := s.store.MoveFile(dropID, oldStoragePath, newStoragePath); err != nil {
		return nil, fmt.Errorf("failed to restore from trash: %w", err)
	}

	// If it's a folder, update children paths
	if file.Type == model.FileTypeFolder {
		all, err := model.GetFilesByDrop(db, dropID)
		if err != nil {
			return nil, err
		}
		oldPrefix := currentTrashPath + "/"
		newPrefix := originalPath + "/"
		for _, f := range all {
			if strings.HasPrefix(f.Path, oldPrefix) {
				f.Path = newPrefix + strings.TrimPrefix(f.Path, oldPrefix)
				model.UpdateFileRecord(db, f)
			}
		}
	}

	// Update file record
	file.Path = originalPath
	if err := model.UpdateFileRecord(db, file); err != nil {
		return nil, err
	}

	// Log activity
	model.LogActivity(db, &model.ActivityLog{
		DropID:     dropID,
		UserID:     userID,
		EventType:  model.EventFileModified,
		TargetType: "file",
		TargetID:   fileID,
		Metadata:   fmt.Sprintf(`{"restored_from_trash": true}`),
	})

	return file, nil
}

// ListTrash returns all trashed items in a drop.
func (s *FileService) ListTrash(dropID string) ([]*model.File, error) {
	db := config.GetDB()
	all, err := model.GetFilesByDrop(db, dropID)
	if err != nil {
		return nil, err
	}
	var trashed []*model.File
	for _, f := range all {
		if strings.HasPrefix(f.Path, ".Trash-1000/") {
			trashed = append(trashed, f)
		}
	}
	return trashed, nil
}

// EmptyTrash permanently deletes all trashed items.
func (s *FileService) EmptyTrash(dropID string, userID int64) error {
	db := config.GetDB()

	trashed, err := s.ListTrash(dropID)
	if err != nil {
		return err
	}

	for _, file := range trashed {
		storageRelPath := filepath.Join("files", file.Path)
		if err := s.store.DeleteFile(dropID, storageRelPath); err != nil {
			return err
		}
		if err := model.DeleteFileRecord(db, file.ID); err != nil {
			return err
		}
	}

	// Log activity
	model.LogActivity(db, &model.ActivityLog{
		DropID:     dropID,
		UserID:     userID,
		EventType:  model.EventFileDeleted,
		TargetType: "trash",
		TargetID:   dropID,
		Metadata:   `{"emptied_trash": true}`,
	})

	return nil
}

// DeleteFile permanently deletes a file or folder.
func (s *FileService) DeleteFile(dropID, fileID string, userID int64) error {
	db := config.GetDB()

	file, err := s.GetFileInDrop(dropID, fileID)
	if err != nil {
		return err
	}

	// If it's a folder, delete all children records too
	if file.Type == model.FileTypeFolder {
		all, err := model.GetFilesByDrop(db, dropID)
		if err == nil {
			prefix := file.Path + "/"
			for _, f := range all {
				if strings.HasPrefix(f.Path, prefix) {
					model.DeleteFileRecord(db, f.ID)
				}
			}
		}
	}

	// Remove from filesystem
	storageRelPath := filepath.Join("files", file.Path)
	s.store.DeleteFile(dropID, storageRelPath)

	// Remove from database
	if err := model.DeleteFileRecord(db, fileID); err != nil {
		return err
	}

	// Log activity
	model.LogActivity(db, &model.ActivityLog{
		DropID:     dropID,
		UserID:     userID,
		EventType:  model.EventFileDeleted,
		TargetType: "file",
		TargetID:   fileID,
	})

	return nil
}

// RenameFile renames a file or folder within a drop.
func (s *FileService) RenameFile(dropID, fileID, newName string, userID int64) (*model.File, error) {
	db := config.GetDB()

	if err := ValidateName(newName); err != nil {
		return nil, err
	}

	file, err := s.GetFileInDrop(dropID, fileID)
	if err != nil {
		return nil, err
	}

	oldPath := file.Path
	parentDir := filepath.Dir(oldPath)
	newPath := filepath.ToSlash(filepath.Join(parentDir, newName))

	// Check for conflicts
	if newPath != oldPath {
		if _, err := model.GetFileByPath(db, dropID, newPath); err == nil {
			return nil, fmt.Errorf("an item named %q already exists in this location", newName)
		}
	}

	oldStoragePath := filepath.Join("files", oldPath)
	newStoragePath := filepath.Join("files", newPath)

	// Move on filesystem
	if err := s.store.MoveFile(dropID, oldStoragePath, newStoragePath); err != nil {
		return nil, fmt.Errorf("failed to rename: %w", err)
	}

	// If it's a folder, update child paths
	if file.Type == model.FileTypeFolder {
		all, err := model.GetFilesByDrop(db, dropID)
		if err == nil {
			oldPrefix := oldPath + "/"
			newPrefix := newPath + "/"
			for _, f := range all {
				if strings.HasPrefix(f.Path, oldPrefix) {
					f.Path = newPrefix + strings.TrimPrefix(f.Path, oldPrefix)
					model.UpdateFileRecord(db, f)
				}
			}
		}
	}

	// Update record
	oldName := file.Name
	file.Name = newName
	file.Path = newPath
	if err := model.UpdateFileRecord(db, file); err != nil {
		return nil, err
	}

	// Log activity
	model.LogActivity(db, &model.ActivityLog{
		DropID:     dropID,
		UserID:     userID,
		EventType:  model.EventFileRenamed,
		TargetType: "file",
		TargetID:   fileID,
		Metadata:   fmt.Sprintf(`{"from": %q, "to": %q}`, oldName, newName),
	})

	return file, nil
}

// MoveFile moves a file or folder to a new directory within a drop.
func (s *FileService) MoveFile(dropID, fileID, newDir string, userID int64) (*model.File, error) {
	db := config.GetDB()

	file, err := s.GetFileInDrop(dropID, fileID)
	if err != nil {
		return nil, err
	}

	newDir = normalizePath(newDir)
	if newDir == "/" {
		newDir = ""
	}

	oldPath := file.Path
	newPath := filepath.ToSlash(filepath.Join(newDir, file.Name))

	// Cannot move into itself
	if newPath == oldPath {
		return file, nil
	}
	// Cannot move a folder into its own subtree
	if file.Type == model.FileTypeFolder && strings.HasPrefix(newPath, oldPath+"/") {
		return nil, errors.New("cannot move a folder into itself")
	}

	// Check for conflicts
	if _, err := model.GetFileByPath(db, dropID, newPath); err == nil {
		return nil, fmt.Errorf("an item named %q already exists in the destination", file.Name)
	}

	oldStoragePath := filepath.Join("files", oldPath)
	newStoragePath := filepath.Join("files", newPath)

	// Move on filesystem
	if err := s.store.MoveFile(dropID, oldStoragePath, newStoragePath); err != nil {
		return nil, fmt.Errorf("failed to move: %w", err)
	}

	// If it's a folder, update child paths
	if file.Type == model.FileTypeFolder {
		all, err := model.GetFilesByDrop(db, dropID)
		if err == nil {
			oldPrefix := oldPath + "/"
			newPrefix := newPath + "/"
			for _, f := range all {
				if strings.HasPrefix(f.Path, oldPrefix) {
					f.Path = newPrefix + strings.TrimPrefix(f.Path, oldPrefix)
					model.UpdateFileRecord(db, f)
				}
			}
		}
	}

	// Update record
	file.Path = newPath
	if err := model.UpdateFileRecord(db, file); err != nil {
		return nil, err
	}

	// Log activity
	model.LogActivity(db, &model.ActivityLog{
		DropID:     dropID,
		UserID:     userID,
		EventType:  model.EventFileMoved,
		TargetType: "file",
		TargetID:   fileID,
		Metadata:   fmt.Sprintf(`{"from": %q, "to": %q}`, oldPath, newPath),
	})

	return file, nil
}

// GetFileMimeType returns the MIME type for a file, for preview purposes.
func (s *FileService) GetFileMimeType(file *model.File) string {
	if file.MimeType != "" {
		return file.MimeType
	}
	ext := strings.ToLower(filepath.Ext(file.Name))
	if inferred := mime.TypeByExtension(ext); inferred != "" {
		return inferred
	}
	return "application/octet-stream"
}

// normalizePath normalizes a relative path string.
func normalizePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || path == "." {
		return "/"
	}
	clean := filepath.ToSlash(filepath.Clean("/" + strings.ReplaceAll(path, "\\", "/")))
	if clean == "." || clean == "/" {
		return "/"
	}
	return clean
}


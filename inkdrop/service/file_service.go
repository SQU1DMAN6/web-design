package service

import (
	"fmt"
	"inkdrop/config"
	"inkdrop/model"
	"inkdrop/storage/filesystem"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

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

// ListFiles returns all files in a drop at a given path.
func (s *FileService) ListFiles(dropID, relPath string) ([]*model.File, error) {
	db := config.GetDB()
	return model.GetFilesByDrop(db, dropID)
}

// GetFile retrieves a file by ID.
func (s *FileService) GetFile(fileID string) (*model.File, error) {
	db := config.GetDB()
	return model.GetFileByID(db, fileID)
}

// CreateFile creates a new file record and optionally writes data.
func (s *FileService) CreateFile(dropID, name, fileType, relPath string, createdBy int64, data []byte) (*model.File, error) {
	db := config.GetDB()

	fileID := uuid.New().String()
	fullPath := filepath.Join(relPath, name)

	file := &model.File{
		ID:        fileID,
		DropID:    dropID,
		Name:      name,
		Type:      model.FileType(fileType),
		Path:      filepath.ToSlash(fullPath),
		CreatedBy: createdBy,
		Size:      int64(len(data)),
	}

	if err := model.CreateFileRecord(db, file); err != nil {
		return nil, fmt.Errorf("failed to create file record: %w", err)
	}

	// Write data to filesystem if provided
	if len(data) > 0 {
		// Compute checksum
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

// ReadFile reads file content from storage.
func (s *FileService) ReadFile(dropID, fileID string) ([]byte, error) {
	file, err := s.GetFile(fileID)
	if err != nil {
		return nil, err
	}
	storageRelPath := filepath.Join("files", file.Path)
	return s.store.ReadFile(dropID, storageRelPath)
}

// UpdateFile updates file content and creates a version snapshot if content changed.
func (s *FileService) UpdateFile(dropID, fileID string, data []byte, userID int64, versionMessage string) (*model.File, error) {
	db := config.GetDB()

	file, err := model.GetFileByID(db, fileID)
	if err != nil {
		return nil, err
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
					CreatedBy:     file.CreatedBy,
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
	}

	// Update record
	file.UpdatedAt = 0 // will be set by UpdateFileRecord
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

// DeleteFile deletes a file and its versions.
func (s *FileService) DeleteFile(dropID, fileID string, userID int64) error {
	db := config.GetDB()

	file, err := model.GetFileByID(db, fileID)
	if err != nil {
		return err
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

// RenameFile renames a file within a drop.
func (s *FileService) RenameFile(dropID, fileID, newName string, userID int64) (*model.File, error) {
	db := config.GetDB()

	file, err := model.GetFileByID(db, fileID)
	if err != nil {
		return nil, err
	}

	oldStoragePath := filepath.Join("files", file.Path)
	newPath := filepath.Join(filepath.Dir(file.Path), newName)
	newStoragePath := filepath.Join("files", newPath)

	// Move on filesystem
	if err := s.store.MoveFile(dropID, oldStoragePath, newStoragePath); err != nil {
		return nil, fmt.Errorf("failed to rename file: %w", err)
	}

	// Update record
	file.Name = newName
	file.Path = filepath.ToSlash(newPath)
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
	})

	return file, nil
}

// MoveFile moves a file to a new directory within a drop.
func (s *FileService) MoveFile(dropID, fileID, newDir string, userID int64) (*model.File, error) {
	db := config.GetDB()

	file, err := model.GetFileByID(db, fileID)
	if err != nil {
		return nil, err
	}

	oldStoragePath := filepath.Join("files", file.Path)
	newPath := filepath.Join(strings.TrimPrefix(newDir, "/"), file.Name)
	newStoragePath := filepath.Join("files", newPath)

	// Move on filesystem
	if err := s.store.MoveFile(dropID, oldStoragePath, newStoragePath); err != nil {
		return nil, fmt.Errorf("failed to move file: %w", err)
	}

	// Update record
	file.Path = filepath.ToSlash(newPath)
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
	})

	return file, nil
}

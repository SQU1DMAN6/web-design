package filesystem

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Store provides filesystem operations for drop storage.
// Controllers must never access this directly — only through services.
type Store struct {
	RootDir string
}

// NewStore creates a new filesystem store rooted at the given directory.
func NewStore(rootDir string) *Store {
	return &Store{RootDir: rootDir}
}

// DropPath returns the absolute path for a drop's storage directory.
func (s *Store) DropPath(dropID string) string {
	return filepath.Clean(filepath.Join(s.RootDir, "drops", dropID))
}

// EnsureDropLayout creates the directory structure for a drop.
func (s *Store) EnsureDropLayout(dropID string) error {
	base := s.DropPath(dropID)
	dirs := []string{
		base,
		filepath.Join(base, "files"),
		filepath.Join(base, "assets"),
		filepath.Join(base, "documents"),
		filepath.Join(base, "presentations"),
		filepath.Join(base, ".inkdrop", "versions"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create %s: %w", dir, err)
		}
	}
	return nil
}

// WriteFile writes data to a file within a drop, creating parent directories.
func (s *Store) WriteFile(dropID, relPath string, data []byte) error {
	fullPath := s.resolvePath(dropID, relPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(fullPath, data, 0644)
}

// ReadFile reads a file from within a drop.
func (s *Store) ReadFile(dropID, relPath string) ([]byte, error) {
	fullPath := s.resolvePath(dropID, relPath)
	return os.ReadFile(fullPath)
}

// DeleteFile removes a file or directory from within a drop.
func (s *Store) DeleteFile(dropID, relPath string) error {
	fullPath := s.resolvePath(dropID, relPath)
	return os.RemoveAll(fullPath)
}

// MoveFile moves/renames a file within a drop.
func (s *Store) MoveFile(dropID, oldRelPath, newRelPath string) error {
	oldPath := s.resolvePath(dropID, oldRelPath)
	newPath := s.resolvePath(dropID, newRelPath)
	if err := os.MkdirAll(filepath.Dir(newPath), 0755); err != nil {
		return err
	}
	return os.Rename(oldPath, newPath)
}

// FileExists checks if a path exists within a drop.
func (s *Store) FileExists(dropID, relPath string) (bool, error) {
	fullPath := s.resolvePath(dropID, relPath)
	_, err := os.Stat(fullPath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// ListDirectory returns sorted directory entries for a path within a drop.
func (s *Store) ListDirectory(dropID, relPath string) ([]os.DirEntry, error) {
	fullPath := s.resolvePath(dropID, relPath)
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})
	return entries, nil
}

// CreateDirectory creates a directory within a drop.
func (s *Store) CreateDirectory(dropID, relPath string) error {
	fullPath := s.resolvePath(dropID, relPath)
	return os.MkdirAll(fullPath, 0755)
}

// WalkFiles walks all files in a drop, calling fn for each file.
func (s *Store) WalkFiles(dropID string, fn func(relPath string, info fs.FileInfo) error) error {
	base := s.DropPath(dropID)
	return filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(base, path)
		if err != nil {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		return fn(filepath.ToSlash(rel), info)
	})
}

// CalculateChecksum returns the SHA-256 checksum of a file.
func (s *Store) CalculateChecksum(dropID, relPath string) (string, error) {
	fullPath := s.resolvePath(dropID, relPath)
	f, err := os.Open(fullPath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// CopyFile copies a file from one drop path to another.
func (s *Store) CopyFile(srcDropID, srcRelPath, dstDropID, dstRelPath string) error {
	srcPath := s.resolvePath(srcDropID, srcRelPath)
	dstPath := s.resolvePath(dstDropID, dstRelPath)
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return err
	}
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dst.Close()
	_, err = io.Copy(dst, src)
	return err
}

// VersionPath returns the path for a specific version of a file.
func (s *Store) VersionPath(dropID, fileID string, versionNumber int) string {
	return filepath.Join(s.DropPath(dropID), ".inkdrop", "versions", fileID, fmt.Sprintf("v%d", versionNumber))
}

// SaveVersion saves a copy of a file as a version snapshot.
func (s *Store) SaveVersion(dropID, fileID, relPath string, versionNumber int) error {
	srcPath := s.resolvePath(dropID, relPath)
	dstPath := s.VersionPath(dropID, fileID, versionNumber)
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return err
	}
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dst.Close()
	_, err = io.Copy(dst, src)
	return err
}

// RestoreVersion restores a file from a version snapshot.
func (s *Store) RestoreVersion(dropID, fileID, relPath string, versionNumber int) error {
	srcPath := s.VersionPath(dropID, fileID, versionNumber)
	dstPath := s.resolvePath(dropID, relPath)
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return err
	}
	dst, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dst.Close()
	_, err = io.Copy(dst, src)
	return err
}

// resolvePath resolves a relative path within a drop, ensuring it doesn't escape.
func (s *Store) resolvePath(dropID, relPath string) string {
	base := s.DropPath(dropID)
	clean := filepath.Clean(filepath.Join(base, relPath))
	// Prevent path traversal
	if !strings.HasPrefix(clean, base) {
		return base
	}
	return clean
}

// EnsureStorageLayout creates the top-level storage directories.
func (s *Store) EnsureStorageLayout() error {
	dirs := []string{
		s.RootDir,
		filepath.Join(s.RootDir, "drops"),
		filepath.Join(s.RootDir, "tmp"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return nil
}

// EnsureDirExists checks if a directory exists and creates it if not.
func EnsureDirExists(path string) error {
	info, err := os.Stat(path)
	if err == nil {
		if info.IsDir() {
			return nil
		}
		return fmt.Errorf("%s exists but is not a directory", path)
	}
	if os.IsNotExist(err) {
		return os.MkdirAll(path, 0755)
	}
	return err
}

// IsWithinRoot checks if a path is within a root directory (prevents traversal).
func IsWithinRoot(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// ErrItemExists is returned when trying to create a file that already exists.
var ErrItemExists = errors.New("item already exists")

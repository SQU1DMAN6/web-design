package service

import (
	"fmt"
	"inkdrop/config"
	"inkdrop/model"
	"inkdrop/storage/filesystem"
)

// VersionService handles file version management.
type VersionService struct {
	store *filesystem.Store
}

// NewVersionService creates a new VersionService.
func NewVersionService(store *filesystem.Store) *VersionService {
	return &VersionService{store: store}
}

// CreateSnapshot creates a manual version snapshot for a file.
func (s *VersionService) CreateSnapshot(dropID, fileID string, userID int64, message string) (*model.FileVersion, error) {
	db := config.GetDB()

	file, err := model.GetFileByID(db, fileID)
	if err != nil {
		return nil, fmt.Errorf("file not found: %w", err)
	}

	// Get latest version number
	latestVersion, err := model.GetLatestFileVersion(db, fileID)
	if err != nil {
		latestVersion = 0
	}

	newVersion := latestVersion + 1
	storageRelPath := fmt.Sprintf("files/%s", file.Path)

	// Save version to filesystem
	if err := s.store.SaveVersion(dropID, fileID, storageRelPath, newVersion); err != nil {
		return nil, fmt.Errorf("failed to save version: %w", err)
	}

	// Create version record
	version := &model.FileVersion{
		FileID:        fileID,
		VersionNumber: newVersion,
		Checksum:      file.Checksum,
		Size:          file.Size,
		CreatedBy:     userID,
		Message:       message,
	}

	if err := model.CreateFileVersion(db, version); err != nil {
		return nil, fmt.Errorf("failed to create version record: %w", err)
	}

	// Log activity
	model.LogActivity(db, &model.ActivityLog{
		DropID:     dropID,
		UserID:     userID,
		EventType:  model.EventVersionCreated,
		TargetType: "file",
		TargetID:   fileID,
	})

	return version, nil
}

// GetVersions returns all versions for a file.
func (s *VersionService) GetVersions(fileID string) ([]*model.FileVersion, error) {
	db := config.GetDB()
	return model.GetFileVersions(db, fileID)
}

// RestoreVersion restores a file to a previous version.
func (s *VersionService) RestoreVersion(dropID, fileID string, versionNumber int, userID int64) (*model.File, error) {
	db := config.GetDB()

	file, err := model.GetFileByID(db, fileID)
	if err != nil {
		return nil, fmt.Errorf("file not found: %w", err)
	}

	// Verify version exists
	versions, err := model.GetFileVersions(db, fileID)
	if err != nil {
		return nil, fmt.Errorf("no versions found: %w", err)
	}

	versionExists := false
	for _, v := range versions {
		if v.VersionNumber == versionNumber {
			versionExists = true
			break
		}
	}
	if !versionExists {
		return nil, fmt.Errorf("version %d not found", versionNumber)
	}

	// Save current state as a new version before restoring
	latestVersion, _ := model.GetLatestFileVersion(db, fileID)
	storageRelPath := fmt.Sprintf("files/%s", file.Path)
	s.store.SaveVersion(dropID, fileID, storageRelPath, latestVersion+1)

	// Restore the requested version
	if err := s.store.RestoreVersion(dropID, fileID, storageRelPath, versionNumber); err != nil {
		return nil, fmt.Errorf("failed to restore version: %w", err)
	}

	// Update file record
	file.Checksum = ""
	file.UpdatedAt = 0
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
		Metadata:   fmt.Sprintf(`{"restored_from_version": %d}`, versionNumber),
	})

	return file, nil
}

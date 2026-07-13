package service

import (
	"errors"
	"fmt"
	"inkdrop/config"
	"inkdrop/model"
	"inkdrop/storage/filesystem"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// DropService handles all drop business logic.
type DropService struct {
	store *filesystem.Store
}

// NewDropService creates a new DropService.
func NewDropService(store *filesystem.Store) *DropService {
	return &DropService{
		store: store,
	}
}

// CreateDrop creates a new drop with the given name and owner.
func (s *DropService) CreateDrop(name, ownerName, description string) (*model.Drop, error) {
	db := config.GetDB()

	// Get the owner user
	owner, err := model.GetUserByName(ownerName, db)
	if err != nil {
		return nil, fmt.Errorf("owner not found: %w", err)
	}

	// Generate a unique ID
	dropID := uuid.New().String()

	// Create the drop record
	drop := &model.Drop{
		ID:          dropID,
		Name:        name,
		OwnerID:     owner.ID,
		Description: description,
		Visibility:  model.DropVisibilityPrivate,
		StoragePath: s.store.DropPath(dropID),
	}

	if err := model.CreateDrop(db, drop); err != nil {
		return nil, fmt.Errorf("failed to create drop record: %w", err)
	}

	// Add the owner as a member with owner role
	member := &model.DropMember{
		DropID: dropID,
		UserID: owner.ID,
		Role:   model.DropRoleOwner,
	}
	if err := model.AddDropMember(db, member); err != nil {
		return nil, fmt.Errorf("failed to add owner: %w", err)
	}

	// Create filesystem layout
	if err := s.store.EnsureDropLayout(dropID); err != nil {
		return nil, fmt.Errorf("failed to create storage: %w", err)
	}

	// Log activity
	model.LogActivity(db, &model.ActivityLog{
		DropID:     dropID,
		UserID:     owner.ID,
		EventType:  model.EventDropCreated,
		TargetType: "drop",
		TargetID:   dropID,
	})

	return drop, nil
}

// GetDrop retrieves a drop by ID.
func (s *DropService) GetDrop(id string) (*model.Drop, error) {
	db := config.GetDB()
	return model.GetDropByID(db, id)
}

// GetUserDrops returns all drops accessible to a user.
func (s *DropService) GetUserDrops(userName string) ([]*model.Drop, error) {
	db := config.GetDB()
	user, err := model.GetUserByName(userName, db)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}
	return model.GetDropsByUser(db, user.ID)
}

// UpdateDropSettings updates drop metadata and settings.
func (s *DropService) UpdateDropSettings(id, description string, visibility model.DropVisibility, settings model.DropSettings) error {
	db := config.GetDB()
	drop, err := model.GetDropByID(db, id)
	if err != nil {
		return err
	}

	drop.Description = description
	if visibility != "" {
		drop.Visibility = visibility
	}
	drop.Settings = drop.MarshalSettings(settings)

	return model.UpdateDrop(db, drop)
}

// DeleteDrop deletes a drop and all its data.
func (s *DropService) DeleteDrop(id string, userID int64) error {
	db := config.GetDB()

	// Verify the user is an owner
	role, err := model.GetUserDropRole(db, id, userID)
	if err != nil || role != model.DropRoleOwner {
		return errors.New("only owners can delete drops")
	}

	// Remove filesystem data
	dropPath := s.store.DropPath(id)
	if err := os.RemoveAll(dropPath); err != nil {
		return fmt.Errorf("failed to remove storage: %w", err)
	}

	// Remove database records
	return model.DeleteDropByID(db, id)
}

// MigrateFromLegacy migrates a legacy drop (from InkDrop 3.x) to the new system.
func (s *DropService) MigrateFromLegacy(userName, dropName string) (*model.Drop, error) {
	db := config.GetDB()

	// Get the owner
	owner, err := model.GetUserByName(userName, db)
	if err != nil {
		return nil, fmt.Errorf("owner not found: %w", err)
	}

	// Check if already migrated
	existingDrops, err := model.GetDropsByUser(db, owner.ID)
	if err == nil {
		for _, d := range existingDrops {
			if d.Name == dropName {
				// Already migrated
				return d, nil
			}
		}
	}

	// Generate new ID
	dropID := uuid.New().String()

	// Create the drop record
	drop := &model.Drop{
		ID:          dropID,
		Name:        dropName,
		OwnerID:     owner.ID,
		Description: "",
		Visibility:  model.DropVisibilityPrivate,
		StoragePath: s.store.DropPath(dropID),
	}

	if err := model.CreateDrop(db, drop); err != nil {
		return nil, fmt.Errorf("failed to create drop record: %w", err)
	}

	// Add owner
	member := &model.DropMember{
		DropID: dropID,
		UserID: owner.ID,
		Role:   model.DropRoleOwner,
	}
	if err := model.AddDropMember(db, member); err != nil {
		return nil, err
	}

	// Create filesystem layout
	if err := s.store.EnsureDropLayout(dropID); err != nil {
		return nil, err
	}

	// Copy files from legacy location
	legacyRoot := filepath.Join("/srv/ftr/userRepositories", userName, dropName)
	if info, err := os.Stat(legacyRoot); err == nil && info.IsDir() {
		filepath.WalkDir(legacyRoot, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(legacyRoot, path)
			if err != nil {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			// Write to new location under files/
			destPath := filepath.Join("files", filepath.ToSlash(rel))
			s.store.WriteFile(dropID, destPath, data)
			return nil
		})
	}

	// Log migration
	model.LogActivity(db, &model.ActivityLog{
		DropID:     dropID,
		UserID:     owner.ID,
		EventType:  model.EventDropCreated,
		TargetType: "drop",
		TargetID:   dropID,
		Metadata:   `{"migrated": true, "legacy_path": "` + legacyRoot + `"}`,
	})

	return drop, nil
}

// GetDropByLegacyPath finds a drop by its legacy user/dropname path.
func (s *DropService) GetDropByLegacyPath(userName, dropName string) (*model.Drop, error) {
	db := config.GetDB()
	user, err := model.GetUserByName(userName, db)
	if err != nil {
		return nil, err
	}
	drops, err := model.GetDropsByUser(db, user.ID)
	if err != nil {
		return nil, err
	}
	for _, d := range drops {
		if strings.EqualFold(d.Name, dropName) {
			return d, nil
		}
	}
	return nil, fmt.Errorf("drop %s/%s not found", userName, dropName)
}

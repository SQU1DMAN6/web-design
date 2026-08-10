package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"inkdrop/config"
	"inkdrop/model"
	"inkdrop/repository"
	"inkdrop/storage/filesystem"
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

	// Get the owner
	owner, err := model.GetUserByName(ownerName, db)
	if err != nil {
		return nil, fmt.Errorf("owner not found: %w", err)
	}

	// Generate a unique ID
	dropID := model.GenerateDropID()

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

// GetDrop retrieves a drop by ID, auto-migrating legacy drops if needed.
func (s *DropService) GetDrop(id string) (*model.Drop, error) {
	db := config.GetDB()
	drop, err := model.GetDropByID(db, id)
	if err == nil {
		return drop, nil
	}
	if !isLegacyMissingError(err) {
		return nil, err
	}

	// Try legacy migration by legacy path if the ID looks like user/drop
	if legacyDrop := s.tryLegacyMigrationByID(id); legacyDrop != nil {
		return legacyDrop, nil
	}
	return nil, err
}

// GetUserDrops returns all drops accessible to a user.
// This includes:
//   - Drops where the user is a member
//   - Public drops
//   - Drops with 'contacts' visibility owned by accepted contacts
func (s *DropService) GetUserDrops(userName string) ([]*model.Drop, error) {
	db := config.GetDB()
	user, err := model.GetUserByName(userName, db)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	// Get drops where the user is a member
	memberDrops, err := model.GetDropsByUser(db, user.ID)
	if err != nil {
		if isLegacyMissingError(err) {
			_ = repository.EnsureDropsSchema()
			memberDrops, err = model.GetDropsByUser(db, user.ID)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	// Get public drops
	publicDrops, err := model.GetPublicDrops(db)
	if err != nil {
		// If the query fails (e.g. missing column), fall back to member drops only
		publicDrops = nil
	}

	// Get contact-visible drops
	contactDrops, err := model.GetContactVisibleDrops(db, user.ID)
	if err != nil {
		contactDrops = nil
	}

	// Merge all drops, deduplicating by ID
	seen := make(map[string]bool)
	var all []*model.Drop
	for _, d := range memberDrops {
		if d != nil && !seen[d.ID] {
			seen[d.ID] = true
			all = append(all, d)
		}
	}
	for _, d := range publicDrops {
		if d != nil && !seen[d.ID] {
			seen[d.ID] = true
			all = append(all, d)
		}
	}
	for _, d := range contactDrops {
		if d != nil && !seen[d.ID] {
			seen[d.ID] = true
			all = append(all, d)
		}
	}

	return all, nil
}

// CanAccess checks whether a user can access a drop.
func (s *DropService) CanAccess(userID int64, dropID string) (bool, error) {
	db := config.GetDB()
	return model.CanAccessDrop(db, dropID, userID)
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
	dropID := model.GenerateDropID()

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

// GetOrMigrateDrop returns an existing 4.0 drop or auto-migrates a legacy drop on first access.
func (s *DropService) GetOrMigrateDrop(userName, dropName string) (*model.Drop, error) {
	db := config.GetDB()
	user, err := model.GetUserByName(userName, db)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	// Check existing 4.0 drops for this user
	drops, err := model.GetDropsByUser(db, user.ID)
	if err == nil {
		for _, d := range drops {
			if strings.EqualFold(d.Name, dropName) {
				return d, nil
			}
		}
	} else if !isLegacyMissingError(err) {
		return nil, err
	}

	// If not found, attempt legacy migration
	return s.MigrateFromLegacy(userName, dropName)
}

func (s *DropService) tryLegacyMigrationByID(id string) *model.Drop {
	parts := strings.SplitN(id, "/", 2)
	if len(parts) != 2 {
		return nil
	}
	userName := parts[0]
	dropName := parts[1]
	if userName == "" || dropName == "" {
		return nil
	}
	drop, err := s.MigrateFromLegacy(userName, dropName)
	if err != nil {
		return nil
	}
	return drop
}

func isLegacyMissingError(err error) bool {
	if err == nil {
		return false
	}
	if strings.Contains(err.Error(), "no such column") ||
		strings.Contains(err.Error(), "no such table") ||
		strings.Contains(err.Error(), "database is closed") {
		return true
	}
	return false
}

package service

import (
	"errors"
	"inkdrop/config"
	"inkdrop/model"
)

// PermissionService handles role-based access control for drops.
type PermissionService struct{}

// NewPermissionService creates a new PermissionService.
func NewPermissionService() *PermissionService {
	return &PermissionService{}
}

// CheckPermission checks if a user has at least the required role for a drop.
// Returns true if the user has sufficient permissions.
// For read-only access (viewer), public drops and contact-visible drops are
// also accessible even without an explicit membership.
func (s *PermissionService) CheckPermission(userID int64, dropID string, requiredRole model.DropRole) bool {
	db := config.GetDB()

	role, err := model.GetUserDropRole(db, dropID, userID)
	if err == nil {
		return roleWeight(role) >= roleWeight(requiredRole)
	}

	// If the user is not a member, check if the drop is publicly accessible
	// for viewer-level access.
	if requiredRole == model.DropRoleViewer {
		canAccess, err := model.CanAccessDrop(db, dropID, userID)
		if err == nil && canAccess {
			return true
		}
	}

	return false
}

// AddMember adds a user to a drop with a specific role.
func (s *PermissionService) AddMember(dropID string, userID int64, role model.DropRole) error {
	db := config.GetDB()
	member := &model.DropMember{
		DropID: dropID,
		UserID: userID,
		Role:   role,
	}
	return model.AddDropMember(db, member)
}

// RemoveMember removes a user from a drop.
func (s *PermissionService) RemoveMember(dropID string, userID int64) error {
	db := config.GetDB()
	return model.RemoveDropMember(db, dropID, userID)
}

// GetMembers returns all members of a drop.
func (s *PermissionService) GetMembers(dropID string) ([]*model.DropMember, error) {
	db := config.GetDB()
	return model.GetDropMembers(db, dropID)
}

// UpdateRole changes a user's role within a drop.
// Only owners can change roles, and cannot demote other owners.
func (s *PermissionService) UpdateRole(dropID string, actorID, targetUserID int64, newRole model.DropRole) error {
	db := config.GetDB()

	// Verify actor is an owner
	actorRole, err := model.GetUserDropRole(db, dropID, actorID)
	if err != nil || actorRole != model.DropRoleOwner {
		return errors.New("only owners can change roles")
	}

	// Cannot change role of another owner (except self-demotion)
	targetRole, err := model.GetUserDropRole(db, dropID, targetUserID)
	if err != nil {
		return errors.New("target user is not a member")
	}

	if targetRole == model.DropRoleOwner && actorID != targetUserID {
		return errors.New("cannot change another owner's role")
	}

	// Cannot demote the last owner
	if targetRole == model.DropRoleOwner && newRole != model.DropRoleOwner {
		// Count other owners
		members, _ := model.GetDropMembers(db, dropID)
		ownerCount := 0
		for _, m := range members {
			if m.Role == model.DropRoleOwner {
				ownerCount++
			}
		}
		if ownerCount <= 1 {
			return errors.New("cannot demote the last owner")
		}
	}

	return model.AddDropMember(db, &model.DropMember{
		DropID: dropID,
		UserID: targetUserID,
		Role:   newRole,
	})
}

// CanModify returns true if the user can modify content in a drop (owner or editor).
func (s *PermissionService) CanModify(userID int64, dropID string) bool {
	return s.CheckPermission(userID, dropID, model.DropRoleEditor)
}

// CanView returns true if the user can view content in a drop.
func (s *PermissionService) CanView(userID int64, dropID string) bool {
	return s.CheckPermission(userID, dropID, model.DropRoleViewer)
}

// IsOwner returns true if the user is an owner of the drop.
func (s *PermissionService) IsOwner(userID int64, dropID string) bool {
	return s.CheckPermission(userID, dropID, model.DropRoleOwner)
}

// roleWeight returns a numeric weight for role comparison.
func roleWeight(role model.DropRole) int {
	switch role {
	case model.DropRoleViewer:
		return 1
	case model.DropRoleEditor:
		return 2
	case model.DropRoleOwner:
		return 3
	default:
		return 0
	}
}

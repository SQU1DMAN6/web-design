package model

import (
	"context"
	"encoding/json"
	"time"

	"github.com/uptrace/bun"
)

type DropVisibility string

const (
	DropVisibilityPrivate DropVisibility = "private"
	DropVisibilityPublic  DropVisibility = "public"
	DropVisibilityShared  DropVisibility = "shared"
)

type Drop struct {
	ID          string         `bun:",pk" json:"id"`
	Name        string         `bun:",notnull" json:"name"`
	OwnerID     int64          `bun:",notnull" json:"owner_id"`
	Description string         `bun:",nullzero" json:"description"`
	Visibility  DropVisibility `bun:",notnull,default:'private'" json:"visibility"`
	StoragePath string         `bun:",notnull" json:"storage_path"`
	Settings    string         `bun:",nullzero" json:"-"` // JSON blob

	CreatedAt int64 `bun:",notnull" json:"created_at"`
	UpdatedAt int64 `bun:",notnull" json:"updated_at"`

	// Parsed settings (not stored directly in DB)
	ParsedSettings DropSettings `bun:"-" json:"settings,omitempty"`
}

type DropSettings struct {
	AllowComments bool `json:"allow_comments"`
	AllowUploads  bool `json:"allow_uploads"`
}

func DefaultDropSettings() DropSettings {
	return DropSettings{
		AllowComments: true,
		AllowUploads:  false,
	}
}

func (d *Drop) ParseSettings() DropSettings {
	ds := DefaultDropSettings()
	if d.Settings == "" {
		return ds
	}
	if err := json.Unmarshal([]byte(d.Settings), &ds); err != nil {
		return DefaultDropSettings()
	}
	return ds
}

func (d *Drop) MarshalSettings(settings DropSettings) string {
	b, err := json.Marshal(settings)
	if err != nil {
		return "{}"
	}
	return string(b)
}

type DropRole string

const (
	DropRoleOwner  DropRole = "owner"
	DropRoleEditor DropRole = "editor"
	DropRoleViewer DropRole = "viewer"
)

type DropMember struct {
	DropID string   `bun:",pk" json:"drop_id"`
	UserID int64    `bun:",pk" json:"user_id"`
	Role   DropRole `bun:",notnull,default:'viewer'" json:"role"`
}

func ModelDrop(db *bun.DB) error {
	ctx := context.Background()

	_, err := db.NewCreateTable().
		Model((*Drop)(nil)).
		IfNotExists().
		Exec(ctx)
	if err != nil {
		return err
	}

	_, err = db.NewCreateTable().
		Model((*DropMember)(nil)).
		IfNotExists().
		Exec(ctx)
	if err != nil {
		return err
	}

	// Create indexes
	if _, err := db.ExecContext(ctx, "CREATE INDEX IF NOT EXISTS idx_drops_owner ON drops(owner_id)"); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, "CREATE INDEX IF NOT EXISTS idx_drops_visibility ON drops(visibility)"); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, "CREATE INDEX IF NOT EXISTS idx_drop_members_user ON drop_members(user_id)"); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, "CREATE INDEX IF NOT EXISTS idx_drop_members_drop ON drop_members(drop_id)"); err != nil {
		return err
	}

	return nil
}

// GetDropByID retrieves a drop by its UUID.
func GetDropByID(db *bun.DB, id string) (*Drop, error) {
	var drop Drop
	ctx := context.Background()
	err := db.NewSelect().Model(&drop).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, err
	}
	drop.ParsedSettings = drop.ParseSettings()
	return &drop, nil
}

// GetDropsByUser returns all drops where the user is an owner or member.
func GetDropsByUser(db *bun.DB, userID int64) ([]*Drop, error) {
	var drops []*Drop
	ctx := context.Background()
	err := db.NewSelect().
		Model(&drops).
		Where("id IN (SELECT drop_id FROM drop_members WHERE user_id = ?)", userID).
		OrderExpr("updated_at DESC").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return drops, nil
}

// CreateDrop inserts a new drop record.
func CreateDrop(db *bun.DB, drop *Drop) error {
	now := time.Now().Unix()
	drop.CreatedAt = now
	drop.UpdatedAt = now
	if drop.Visibility == "" {
		drop.Visibility = DropVisibilityPrivate
	}
	drop.Settings = drop.MarshalSettings(DefaultDropSettings())
	ctx := context.Background()
	_, err := db.NewInsert().Model(drop).Exec(ctx)
	return err
}

// UpdateDrop updates drop metadata.
func UpdateDrop(db *bun.DB, drop *Drop) error {
	drop.UpdatedAt = time.Now().Unix()
	ctx := context.Background()
	_, err := db.NewUpdate().Model(drop).Where("id = ?", drop.ID).Exec(ctx)
	return err
}

// DeleteDropByID removes a drop and its members.
func DeleteDropByID(db *bun.DB, id string) error {
	ctx := context.Background()
	// Delete members first
	if _, err := db.NewDelete().Model((*DropMember)(nil)).Where("drop_id = ?", id).Exec(ctx); err != nil {
		return err
	}
	// Delete the drop
	_, err := db.NewDelete().Model((*Drop)(nil)).Where("id = ?", id).Exec(ctx)
	return err
}

// AddDropMember adds a user to a drop with a specific role.
func AddDropMember(db *bun.DB, member *DropMember) error {
	ctx := context.Background()
	_, err := db.NewInsert().Model(member).On("CONFLICT(drop_id, user_id) DO UPDATE SET role = ?", member.Role).Exec(ctx)
	return err
}

// RemoveDropMember removes a user from a drop.
func RemoveDropMember(db *bun.DB, dropID string, userID int64) error {
	ctx := context.Background()
	_, err := db.NewDelete().
		Model((*DropMember)(nil)).
		Where("drop_id = ? AND user_id = ?", dropID, userID).
		Exec(ctx)
	return err
}

// GetDropMembers returns all members of a drop.
func GetDropMembers(db *bun.DB, dropID string) ([]*DropMember, error) {
	var members []*DropMember
	ctx := context.Background()
	err := db.NewSelect().
		Model(&members).
		Where("drop_id = ?", dropID).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return members, nil
}

// GetUserDropRole returns the role of a user in a drop.
func GetUserDropRole(db *bun.DB, dropID string, userID int64) (DropRole, error) {
	var member DropMember
	ctx := context.Background()
	err := db.NewSelect().
		Model(&member).
		Where("drop_id = ? AND user_id = ?", dropID, userID).
		Scan(ctx)
	if err != nil {
		return "", err
	}
	return member.Role, nil
}

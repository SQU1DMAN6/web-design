package model

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type DropVisibility string

const (
	DropVisibilityPrivate  DropVisibility = "private"
	DropVisibilityShared   DropVisibility = "shared"
	DropVisibilityContacts DropVisibility = "contacts"
	DropVisibilityPublic   DropVisibility = "public"
)

// dropJSON is used for JSON serialization with Unix timestamps.
type dropJSON struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	OwnerID     int64          `json:"owner_id"`
	Description string         `json:"description"`
	Visibility  DropVisibility `json:"visibility"`
	Settings    string         `json:"settings"`
	StoragePath string         `json:"storage_path"`
	CreatedAt   int64          `json:"created_at"`
	UpdatedAt   int64          `json:"updated_at"`
}

// MarshalJSON serializes Drop as dropJSON with Unix timestamps.
func (d *Drop) MarshalJSON() ([]byte, error) {
	return json.Marshal(dropJSON{
		ID:          d.ID,
		Name:        d.Name,
		OwnerID:     d.OwnerID,
		Description: d.Description,
		Visibility:  d.Visibility,
		Settings:    d.Settings,
		StoragePath: d.StoragePath,
		CreatedAt:   d.CreatedAt.Unix(),
		UpdatedAt:   d.UpdatedAt.Unix(),
	})
}

// UnmarshalJSON deserializes Drop from dropJSON with Unix timestamps.
func (d *Drop) UnmarshalJSON(data []byte) error {
	var j dropJSON
	if err := json.Unmarshal(data, &j); err != nil {
		return err
	}
	d.ID = j.ID
	d.Name = j.Name
	d.OwnerID = j.OwnerID
	d.Description = j.Description
	d.Visibility = j.Visibility
	d.Settings = j.Settings
	d.StoragePath = j.StoragePath
	d.CreatedAt = time.Unix(j.CreatedAt, 0)
	d.UpdatedAt = time.Unix(j.UpdatedAt, 0)
	return nil
}

type Drop struct {
	ID          string         `bun:",pk" json:"-"`
	Name        string         `bun:",notnull" json:"-"`
	OwnerID     int64          `bun:",notnull" json:"-"`
	Description string         `bun:",notnull" json:"-"`
	Visibility  DropVisibility `bun:",notnull" json:"-"`
	Settings    string         `bun:",notnull" json:"-"`
	StoragePath string         `bun:",notnull" json:"-"`
	CreatedAt   time.Time      `bun:",nullzero,notnull,default:current_timestamp" json:"-"`
	UpdatedAt   time.Time      `bun:",nullzero,notnull,default:current_timestamp" json:"-"`
}

type DropMember struct {
	ID        int64     `bun:",pk,autoincrement" json:"id"`
	DropID    string    `bun:",notnull" json:"drop_id"`
	UserID    int64     `bun:",notnull" json:"user_id"`
	Role      DropRole  `bun:",notnull" json:"role"`
	CreatedAt time.Time `bun:",nullzero,notnull,default:current_timestamp" json:"created_at"`
}

type DropSettings struct {
	AllowComments bool `json:"allow_comments"`
	AllowUploads  bool `json:"allow_uploads"`
}

type DropRole string

const (
	DropRoleViewer DropRole = "viewer"
	DropRoleEditor DropRole = "editor"
	DropRoleOwner  DropRole = "owner"
)

var ErrDropNotFound = fmt.Errorf("drop not found")

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
	return ensureDropColumns(db.DB)
}

func ensureDropColumns(sqldb *sql.DB) error {
	defs := map[string]map[string]string{
		"drops": {
			"settings": "TEXT NOT NULL DEFAULT ''",
		},
	}
	for table, cols := range defs {
		for col, def := range cols {
			rows, err := sqldb.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
			if err != nil {
				return err
			}
			found := false
			for rows.Next() {
				var cid int
				var name string
				var columnType string
				var notNull int
				var defaultValue interface{}
				var pk int
				if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
					rows.Close()
					return err
				}
				if strings.EqualFold(name, col) {
					found = true
					break
				}
			}
			rows.Close()
			if !found {
				if _, err := sqldb.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, col, def)); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// GenerateDropID returns a new unique drop identifier.
func GenerateDropID() string {
	return "drop_" + uuid.New().String()
}

func CreateDrop(db *bun.DB, d *Drop) error {
	if d.ID == "" {
		d.ID = GenerateDropID()
	}
	now := time.Now()
	d.CreatedAt = now
	d.UpdatedAt = now
	if strings.TrimSpace(d.Settings) == "" {
		d.Settings = "{}"
	}
	ctx := context.Background()
	_, err := db.NewInsert().Model(d).Exec(ctx)
	return err
}

func UpdateDrop(db *bun.DB, d *Drop) error {
	d.UpdatedAt = time.Now()
	ctx := context.Background()
	_, err := db.NewUpdate().Model(d).Where("id = ?", d.ID).Exec(ctx)
	return err
}

func DeleteDropByID(db *bun.DB, id string) error {
	ctx := context.Background()
	_, err := db.NewDelete().Model((*Drop)(nil)).Where("id = ?", id).Exec(ctx)
	return err
}

func GetDropByID(db *bun.DB, id string) (*Drop, error) {
	var drop Drop
	ctx := context.Background()
	err := db.NewSelect().Model(&drop).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &drop, nil
}

func GetDropByName(db *bun.DB, name string) (*Drop, error) {
	var drop Drop
	ctx := context.Background()
	err := db.NewSelect().Model(&drop).Where("name = ?", name).Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &drop, nil
}

func GetDropsByUser(db *bun.DB, userID int64) ([]*Drop, error) {
	ctx := context.Background()
	var drops []*Drop
	// Use raw SQL with explicit aliases to avoid bun's table alias mangling.
	err := db.NewRaw(`
		SELECT d.id, d.name, d.owner_id, d.description, d.visibility,
		       d.settings, d.storage_path, d.created_at, d.updated_at
		FROM drops d
		JOIN drop_members dm ON dm.drop_id = d.id
		WHERE dm.user_id = ?
		ORDER BY d.updated_at DESC
	`, userID).Scan(ctx, &drops)
	if err != nil {
		return nil, err
	}
	return drops, nil
}

// GetPublicDrops returns all drops with public visibility.
func GetPublicDrops(db *bun.DB) ([]*Drop, error) {
	ctx := context.Background()
	var drops []*Drop
	err := db.NewRaw(`
		SELECT d.id, d.name, d.owner_id, d.description, d.visibility,
		       d.settings, d.storage_path, d.created_at, d.updated_at
		FROM drops d
		WHERE d.visibility = 'public'
		ORDER BY d.updated_at DESC
	`).Scan(ctx, &drops)
	if err != nil {
		return nil, err
	}
	return drops, nil
}

// GetContactVisibleDrops returns drops owned by users who have an accepted
// contact relationship with the given user, where the drop visibility is 'contacts'.
func GetContactVisibleDrops(db *bun.DB, userID int64) ([]*Drop, error) {
	ctx := context.Background()
	var drops []*Drop
	err := db.NewRaw(`
		SELECT d.id, d.name, d.owner_id, d.description, d.visibility,
		       d.settings, d.storage_path, d.created_at, d.updated_at
		FROM drops d
		WHERE d.visibility = 'contacts'
		  AND d.owner_id IN (
			SELECT u.id
			FROM contact_requests cr
			JOIN users u ON u.name = CASE
				WHEN cr.requester = ? THEN cr.recipient
				ELSE cr.requester
			END
			WHERE cr.status = 'accepted'
			  AND (cr.requester = ? OR cr.recipient = ?)
		  )
		ORDER BY d.updated_at DESC
	`, userNameForID(db, userID), userNameForID(db, userID), userNameForID(db, userID)).Scan(ctx, &drops)
	if err != nil {
		return nil, err
	}
	return drops, nil
}

// CanAccessDrop determines whether a user can access a drop.
// Access is granted if:
//   - The user is a member of the drop
//   - The drop is public
//   - The drop is 'contacts' visibility and the user has an accepted contact
//     relationship with the drop owner
func CanAccessDrop(db *bun.DB, dropID string, userID int64) (bool, error) {
	ctx := context.Background()

	// Check membership first
	var member DropMember
	err := db.NewSelect().Model(&member).
		Where("drop_id = ?", dropID).
		Where("user_id = ?", userID).
		Limit(1).
		Scan(ctx)
	if err == nil {
		return true, nil
	}
	if err != sql.ErrNoRows {
		// Check if the error is "no rows" from bun
		if !strings.Contains(err.Error(), "no rows") {
			return false, err
		}
	}

	// Check drop visibility
	var drop Drop
	err = db.NewSelect().Model(&drop).Where("id = ?", dropID).Scan(ctx)
	if err != nil {
		return false, err
	}

	// Public drops are accessible to everyone
	if drop.Visibility == DropVisibilityPublic {
		return true, nil
	}

	// Contacts visibility: check accepted contact relationship with owner
	if drop.Visibility == DropVisibilityContacts {
		userName := userNameForID(db, userID)
		ownerName := userNameForID(db, drop.OwnerID)
		if userName == "" || ownerName == "" {
			return false, nil
		}
		var count int
		err := db.NewRaw(`
			SELECT COUNT(*)
			FROM contact_requests cr
			WHERE cr.status = 'accepted'
			  AND ((cr.requester = ? AND cr.recipient = ?)
			    OR (cr.requester = ? AND cr.recipient = ?))
		`, userName, ownerName, ownerName, userName).Scan(ctx, &count)
		if err != nil {
			return false, err
		}
		return count > 0, nil
	}

	return false, nil
}

func AddDropMember(db *bun.DB, member *DropMember) error {
	member.CreatedAt = time.Now()
	ctx := context.Background()
	_, err := db.NewInsert().Model(member).Exec(ctx)
	return err
}

func RemoveDropMember(db *bun.DB, dropID string, userID int64) error {
	ctx := context.Background()
	_, err := db.NewDelete().Model((*DropMember)(nil)).
		Where("drop_id = ?", dropID).
		Where("user_id = ?", userID).
		Exec(ctx)
	return err
}

func UpdateDropMemberRole(db *bun.DB, dropID string, userID int64, role DropRole) error {
	ctx := context.Background()
	_, err := db.NewUpdate().Model((*DropMember)(nil)).
		Set("role = ?", role).
		Where("drop_id = ?", dropID).
		Where("user_id = ?", userID).
		Exec(ctx)
	return err
}

func GetUserDropRole(db *bun.DB, dropID string, userID int64) (DropRole, error) {
	ctx := context.Background()
	var member DropMember
	err := db.NewSelect().Model(&member).
		Where("drop_id = ?", dropID).
		Where("user_id = ?", userID).
		Limit(1).
		Scan(ctx)
	if err != nil {
		return "", err
	}
	return member.Role, nil
}

func GetDropMembers(db *bun.DB, dropID string) ([]*DropMember, error) {
	ctx := context.Background()
	var members []*DropMember
	err := db.NewSelect().Model(&members).
		Where("drop_id = ?", dropID).
		Order("user_id ASC").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return members, nil
}

func (d *Drop) MarshalSettings(in DropSettings) string {
	if in.AllowComments || in.AllowUploads {
		return fmt.Sprintf(`{"allow_comments":%v,"allow_uploads":%v}`, in.AllowComments, in.AllowUploads)
	}
	return "{}"
}

func (d *Drop) UnmarshalSettings() DropSettings {
	out := DropSettings{}
	if strings.TrimSpace(d.Settings) == "" {
		return out
	}
	fmt.Sscanf(d.Settings, `{"allow_comments":%t,"allow_uploads":%t}`, &out.AllowComments, &out.AllowUploads)
	return out
}

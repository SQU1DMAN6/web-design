package model

import (
	"context"
	"database/sql"
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

type Drop struct {
	ID          string        `bun:",pk" json:"id"`
	Name        string        `bun:",notnull" json:"name"`
	OwnerID     int64         `bun:",notnull" json:"owner_id"`
	Description string        `bun:",notnull" json:"description"`
	Visibility  DropVisibility `bun:",notnull" json:"visibility"`
	Settings    string        `bun:",notnull" json:"settings"`
	StoragePath string        `bun:",notnull" json:"storage_path"`
	CreatedAt   int64         `bun:",notnull" json:"created_at"`
	UpdatedAt   int64         `bun:",notnull" json:"updated_at"`
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
	now := time.Now().Unix()
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
	d.UpdatedAt = time.Now().Unix()
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
	err := db.NewSelect().
		Model(&drops).
		ColumnExpr("drops.*").
		Join("JOIN drop_members ON drop_members.drop_id = drops.id").
		Where("drop_members.user_id = ?", userID).
		Order("drops.updated_at DESC").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return drops, nil
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
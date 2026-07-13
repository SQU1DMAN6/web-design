package model

import (
	"context"
	"time"

	"github.com/uptrace/bun"
)

type ActivityLog struct {
	ID         int64  `bun:",pk,autoincrement" json:"id"`
	DropID     string `bun:",notnull" json:"drop_id"`
	UserID     int64  `bun:",nullzero" json:"user_id"`
	EventType  string `bun:",notnull" json:"event_type"`
	TargetType string `bun:",nullzero" json:"target_type"`
	TargetID   string `bun:",nullzero" json:"target_id"`
	Metadata   string `bun:",nullzero" json:"-"` // JSON blob

	CreatedAt int64 `bun:",notnull" json:"created_at"`
}

const (
	EventDropCreated       = "drop.created"
	EventDropUpdated       = "drop.updated"
	EventDropDeleted       = "drop.deleted"
	EventFileCreated       = "file.created"
	EventFileModified      = "file.modified"
	EventFileDeleted       = "file.deleted"
	EventFileRenamed       = "file.renamed"
	EventFileMoved         = "file.moved"
	EventMemberJoined      = "member.joined"
	EventMemberLeft        = "member.left"
	EventPermissionChanged = "permission.changed"
	EventVersionCreated    = "version.created"
)

func ModelActivity(db *bun.DB) error {
	ctx := context.Background()

	_, err := db.NewCreateTable().
		Model((*ActivityLog)(nil)).
		IfNotExists().
		Exec(ctx)
	if err != nil {
		return err
	}

	// Create indexes
	if _, err := db.ExecContext(ctx, "CREATE INDEX IF NOT EXISTS idx_activity_drop ON activity_logs(drop_id)"); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, "CREATE INDEX IF NOT EXISTS idx_activity_user ON activity_logs(user_id)"); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, "CREATE INDEX IF NOT EXISTS idx_activity_event ON activity_logs(event_type)"); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, "CREATE INDEX IF NOT EXISTS idx_activity_created ON activity_logs(created_at)"); err != nil {
		return err
	}

	return nil
}

// LogActivity creates a new activity log entry.
func LogActivity(db *bun.DB, log *ActivityLog) error {
	log.CreatedAt = time.Now().Unix()
	ctx := context.Background()
	_, err := db.NewInsert().Model(log).Exec(ctx)
	return err
}

// GetDropActivity returns activity logs for a drop, ordered by most recent first.
func GetDropActivity(db *bun.DB, dropID string, limit, offset int) ([]*ActivityLog, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	var logs []*ActivityLog
	ctx := context.Background()
	err := db.NewSelect().
		Model(&logs).
		Where("drop_id = ?", dropID).
		OrderExpr("created_at DESC").
		Limit(limit).
		Offset(offset).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return logs, nil
}

// GetUserActivity returns activity logs for a user across all drops.
func GetUserActivity(db *bun.DB, userID int64, limit, offset int) ([]*ActivityLog, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	var logs []*ActivityLog
	ctx := context.Background()
	err := db.NewSelect().
		Model(&logs).
		Where("user_id = ?", userID).
		OrderExpr("created_at DESC").
		Limit(limit).
		Offset(offset).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return logs, nil
}

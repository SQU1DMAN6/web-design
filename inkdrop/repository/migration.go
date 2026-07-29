package repository

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"inkdrop/config"
)

// EnsureDropsSchema ensures the drops table exists with required columns.
func EnsureDropsSchema() error {
	db := config.GetDB()
	sqldb := db.DB

	// Create table if missing
	_, err := sqldb.Exec(`CREATE TABLE IF NOT EXISTS drops (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		owner_id INTEGER NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		visibility TEXT NOT NULL DEFAULT 'private',
		settings TEXT NOT NULL DEFAULT '{}',
		storage_path TEXT NOT NULL,
		created_at INTEGER NOT NULL DEFAULT 0,
		updated_at INTEGER NOT NULL DEFAULT 0
	)`)
	if err != nil {
		return fmt.Errorf("create drops table: %w", err)
	}

	// Ensure required columns exist
	required := map[string]string{
		"id":           "TEXT NOT NULL DEFAULT ''",
		"name":         "TEXT NOT NULL DEFAULT ''",
		"owner_id":     "INTEGER NOT NULL DEFAULT 0",
		"description":  "TEXT NOT NULL DEFAULT ''",
		"visibility":   "TEXT NOT NULL DEFAULT 'private'",
		"settings":     "TEXT NOT NULL DEFAULT '{}'",
		"storage_path": "TEXT NOT NULL DEFAULT ''",
		"created_at":   "INTEGER NOT NULL DEFAULT 0",
		"updated_at":   "INTEGER NOT NULL DEFAULT 0",
	}

	rows, err := sqldb.Query("PRAGMA table_info(drops)")
	if err != nil {
		return fmt.Errorf("table_info drops: %w", err)
	}
	defer rows.Close()

	existing := map[string]bool{}
	for rows.Next() {
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue interface{}
		var pk int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		existing[name] = true
	}
	rows.Close()

	for col, def := range required {
		if !existing[col] {
			if _, err := sqldb.Exec(fmt.Sprintf("ALTER TABLE drops ADD COLUMN %s %s", col, def)); err != nil {
				return fmt.Errorf("add column %s: %w", col, err)
			}
		}
	}

	// Create index on owner_id if missing
	_, err = sqldb.Exec("CREATE INDEX IF NOT EXISTS idx_drops_owner_id ON drops(owner_id)")
	if err != nil {
		return fmt.Errorf("create owner_id index: %w", err)
	}

	// Create drop_members table if missing
	_, err = sqldb.Exec(`CREATE TABLE IF NOT EXISTS drop_members (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		drop_id TEXT NOT NULL,
		user_id INTEGER NOT NULL,
		role TEXT NOT NULL DEFAULT 'viewer',
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		return fmt.Errorf("create drop_members table: %w", err)
	}

	_, err = sqldb.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_drop_members_unique ON drop_members(drop_id, user_id)")
	if err != nil {
		return fmt.Errorf("create drop_members unique index: %w", err)
	}

	_, err = sqldb.Exec("CREATE INDEX IF NOT EXISTS idx_drop_members_user ON drop_members(user_id)")
	if err != nil {
		return fmt.Errorf("create drop_members user index: %w", err)
	}

	// Ensure drop_members columns exist (in case table was created by bun with different schema)
	memberRequired := map[string]string{
		"id":         "INTEGER PRIMARY KEY AUTOINCREMENT",
		"drop_id":    "TEXT NOT NULL",
		"user_id":    "INTEGER NOT NULL",
		"role":       "TEXT NOT NULL DEFAULT 'viewer'",
		"created_at": "TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP",
	}
	memberRows, err := sqldb.Query("PRAGMA table_info(drop_members)")
	if err != nil {
		return fmt.Errorf("table_info drop_members: %w", err)
	}
	defer memberRows.Close()

	memberExisting := map[string]bool{}
	for memberRows.Next() {
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue interface{}
		var pk int
		if err := memberRows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		memberExisting[name] = true
	}
	memberRows.Close()

	for col, def := range memberRequired {
		if !memberExisting[col] {
			if _, err := sqldb.Exec(fmt.Sprintf("ALTER TABLE drop_members ADD COLUMN %s %s", col, def)); err != nil {
				return fmt.Errorf("add drop_members column %s: %w", col, err)
			}
		}
	}

	return nil
}

// AutoMigrateLegacyDrop ensures a legacy drop directory is migrated to 4.0 if needed.
func AutoMigrateLegacyDrop(userName, dropName string) error {
	metaDir := filepath.Join(DropMetaDir, userName, dropName)
	dropV4Path := filepath.Join(metaDir, "drop.json")

	// If already 4.0, skip
	if _, err := os.Stat(dropV4Path); err == nil {
		return nil
	}

	// Load legacy meta using repository helper.
	meta, err := LoadRepoMeta(userName, dropName)
	if err != nil || meta == nil {
		return fmt.Errorf("legacy drop metadata not found")
	}

	// Write 4.0 marker
	v4 := map[string]interface{}{
		"version":     "4.0",
		"migrated":    true,
		"legacy": map[string]interface{}{
			"owners":      meta.Owners,
			"description": meta.Description,
			"public":      meta.Public,
			"created_at":  meta.CreatedAt,
			"updated_at":  meta.UpdatedAt,
		},
	}
	b, err := json.MarshalIndent(v4, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(metaDir, 0755); err != nil {
		return err
	}
	if err := os.WriteFile(dropV4Path, b, 0644); err != nil {
		return err
	}

	return nil
}
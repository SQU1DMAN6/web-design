package model

import (
	"context"
	"time"

	"github.com/uptrace/bun"
)

type FileType string

const (
	FileTypeFile         FileType = "file"
	FileTypeDocument     FileType = "document"
	FileTypePresentation FileType = "presentation"
	FileTypeFolder       FileType = "folder"
)

type File struct {
	ID        string   `bun:",pk" json:"id"`
	DropID    string   `bun:",notnull" json:"drop_id"`
	Name      string   `bun:",notnull" json:"name"`
	Type      FileType `bun:",notnull,default:'file'" json:"type"`
	MimeType  string   `bun:",nullzero" json:"mime_type"`
	Size      int64    `bun:",default:0" json:"size"`
	Path      string   `bun:",notnull" json:"path"`
	CreatedBy int64    `bun:",nullzero" json:"created_by"`
	Checksum  string   `bun:",nullzero" json:"checksum"`

	CreatedAt int64 `bun:",notnull" json:"created_at"`
	UpdatedAt int64 `bun:",notnull" json:"updated_at"`
}

type FileVersion struct {
	ID            int64  `bun:",pk,autoincrement" json:"id"`
	FileID        string `bun:",notnull" json:"file_id"`
	VersionNumber int    `bun:",notnull" json:"version_number"`
	Checksum      string `bun:",notnull" json:"checksum"`
	Size          int64  `bun:",default:0" json:"size"`
	CreatedBy     int64  `bun:",nullzero" json:"created_by"`
	Message       string `bun:",nullzero" json:"message"`

	CreatedAt int64 `bun:",notnull" json:"created_at"`
}

func ModelFile(db *bun.DB) error {
	ctx := context.Background()

	_, err := db.NewCreateTable().
		Model((*File)(nil)).
		IfNotExists().
		Exec(ctx)
	if err != nil {
		return err
	}

	_, err = db.NewCreateTable().
		Model((*FileVersion)(nil)).
		IfNotExists().
		Exec(ctx)
	if err != nil {
		return err
	}

	// Create indexes
	if _, err := db.ExecContext(ctx, "CREATE INDEX IF NOT EXISTS idx_files_drop ON files(drop_id)"); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, "CREATE INDEX IF NOT EXISTS idx_files_path ON files(drop_id, path)"); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, "CREATE INDEX IF NOT EXISTS idx_file_versions_file ON file_versions(file_id)"); err != nil {
		return err
	}

	return nil
}

// GetFileByID retrieves a file by its UUID.
func GetFileByID(db *bun.DB, id string) (*File, error) {
	var file File
	ctx := context.Background()
	err := db.NewSelect().Model(&file).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &file, nil
}

// GetFilesByDrop returns all files in a drop.
func GetFilesByDrop(db *bun.DB, dropID string) ([]*File, error) {
	var files []*File
	ctx := context.Background()
	err := db.NewSelect().
		Model(&files).
		Where("drop_id = ?", dropID).
		OrderExpr("path ASC, name ASC").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return files, nil
}

// GetFileByPath retrieves a file by its drop ID and relative path.
func GetFileByPath(db *bun.DB, dropID, path string) (*File, error) {
	var file File
	ctx := context.Background()
	err := db.NewSelect().
		Model(&file).
		Where("drop_id = ? AND path = ?", dropID, path).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &file, nil
}

// CreateFile inserts a new file record.
func CreateFileRecord(db *bun.DB, file *File) error {
	now := time.Now().Unix()
	file.CreatedAt = now
	file.UpdatedAt = now
	if file.Type == "" {
		file.Type = FileTypeFile
	}
	ctx := context.Background()
	_, err := db.NewInsert().Model(file).Exec(ctx)
	return err
}

// UpdateFile updates file metadata.
func UpdateFileRecord(db *bun.DB, file *File) error {
	file.UpdatedAt = time.Now().Unix()
	ctx := context.Background()
	_, err := db.NewUpdate().Model(file).Where("id = ?", file.ID).Exec(ctx)
	return err
}

// DeleteFileRecord deletes a file record.
func DeleteFileRecord(db *bun.DB, id string) error {
	ctx := context.Background()
	// Delete versions first
	if _, err := db.NewDelete().Model((*FileVersion)(nil)).Where("file_id = ?", id).Exec(ctx); err != nil {
		return err
	}
	_, err := db.NewDelete().Model((*File)(nil)).Where("id = ?", id).Exec(ctx)
	return err
}

// CreateFileVersion inserts a new version record.
func CreateFileVersion(db *bun.DB, version *FileVersion) error {
	version.CreatedAt = time.Now().Unix()
	ctx := context.Background()
	_, err := db.NewInsert().Model(version).Exec(ctx)
	return err
}

// GetFileVersions returns all versions for a file, ordered by version number desc.
func GetFileVersions(db *bun.DB, fileID string) ([]*FileVersion, error) {
	var versions []*FileVersion
	ctx := context.Background()
	err := db.NewSelect().
		Model(&versions).
		Where("file_id = ?", fileID).
		OrderExpr("version_number DESC").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return versions, nil
}

// GetLatestFileVersion returns the latest version number for a file.
func GetLatestFileVersion(db *bun.DB, fileID string) (int, error) {
	var version FileVersion
	ctx := context.Background()
	err := db.NewSelect().
		Model(&version).
		Where("file_id = ?", fileID).
		OrderExpr("version_number DESC").
		Limit(1).
		Scan(ctx)
	if err != nil {
		return 0, err
	}
	return version.VersionNumber, nil
}

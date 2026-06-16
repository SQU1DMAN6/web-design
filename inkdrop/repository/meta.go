package repository

import (
	"encoding/json"
	"fmt"
	"inkdrop/config"
	"inkdrop/model"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RepoMeta represents drop metadata stored on disk as JSON.
type RepoMeta struct {
	Owners      []string               `json:"owners"`
	Description string                 `json:"description"`
	Public      bool                   `json:"public"`
	CreatedAt   int64                  `json:"created_at"`
	UpdatedAt   int64                  `json:"updated_at"`
	Extra       map[string]interface{} `json:"extra,omitempty"`
}

// metaPath returns path to the metadata file for a drop.
func metaPath(userName, dropName string) string {
	return filepath.Join(DropMetaDir, userName, dropName, "meta.json")
}

// LoadRepoMeta loads metadata for a drop. If not present, returns nil, nil.
func LoadRepoMeta(userName, dropName string) (*RepoMeta, error) {
	p := metaPath(userName, dropName)
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var m RepoMeta
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// SaveRepoMeta writes drop metadata to disk, creating directories as needed.
func SaveRepoMeta(userName, dropName string, m *RepoMeta) error {
	dir := filepath.Join(DropMetaDir, userName, dropName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	now := time.Now().Unix()
	if m.CreatedAt == 0 {
		m.CreatedAt = now
	}
	m.UpdatedAt = now
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "meta.json"), b, 0644)
}

// ValidateOwners returns a cleaned list of owners where each owner exists in the DB.
// Non-existent usernames are ignored.
func ValidateOwners(raw string) ([]string, error) {
	out := []string{}
	parts := strings.Split(raw, ",")
	db := config.GetDB()
	for _, p := range parts {
		name := strings.TrimSpace(p)
		if name == "" {
			continue
		}
		if _, err := model.GetUserByName(name, db); err == nil {
			out = append(out, name)
		}
	}
	if len(out) == 0 {
		return out, fmt.Errorf("no valid owners found")
	}
	return out, nil
}

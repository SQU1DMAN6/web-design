package service

import (
	"inkdrop/config"
	"inkdrop/model"
	"strings"
)

// SearchService handles global search across drops, files, and users.
type SearchService struct {
	perm *PermissionService
}

// NewSearchService creates a new SearchService.
func NewSearchService(perm *PermissionService) *SearchService {
	return &SearchService{perm: perm}
}

// SearchResult is a generic search result.
type SearchResult struct {
	Type        string `json:"type"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	DropID      string `json:"drop_id,omitempty"`
	DropName    string `json:"drop_name,omitempty"`
	OwnerName   string `json:"owner_name,omitempty"`
	Path        string `json:"path,omitempty"`
}

// SearchDrops finds drops matching a query that the user can access.
func (s *SearchService) SearchDrops(query string, userID int64) ([]*SearchResult, error) {
	db := config.GetDB()
	q := strings.TrimSpace(strings.ToLower(query))
	if q == "" {
		return nil, nil
	}

	// Get all drops the user has access to
	drops, err := model.GetDropsByUser(db, userID)
	if err != nil {
		return nil, err
	}

	var results []*SearchResult
	for _, drop := range drops {
		if strings.Contains(strings.ToLower(drop.Name), q) ||
			strings.Contains(strings.ToLower(drop.Description), q) {
			// Get owner name
			owner, _ := model.GetUserByID(int(drop.OwnerID), db)
			ownerName := ""
			if owner != nil {
				ownerName = owner.Name
			}
			results = append(results, &SearchResult{
				Type:        "drop",
				ID:          drop.ID,
				Name:        drop.Name,
				Description: drop.Description,
				OwnerName:   ownerName,
			})
		}
	}

	return results, nil
}

// SearchFiles finds files within drops that the user can access.
func (s *SearchService) SearchFiles(query string, userID int64) ([]*SearchResult, error) {
	db := config.GetDB()
	q := strings.TrimSpace(strings.ToLower(query))
	if q == "" {
		return nil, nil
	}

	// Get accessible drops
	drops, err := model.GetDropsByUser(db, userID)
	if err != nil {
		return nil, err
	}

	var results []*SearchResult
	for _, drop := range drops {
		// Get files in this drop
		files, err := model.GetFilesByDrop(db, drop.ID)
		if err != nil {
			continue
		}
		for _, file := range files {
			if strings.Contains(strings.ToLower(file.Name), q) {
				results = append(results, &SearchResult{
					Type:     "file",
					ID:       file.ID,
					Name:     file.Name,
					DropID:   drop.ID,
					DropName: drop.Name,
					Path:     file.Path,
				})
			}
		}
	}

	return results, nil
}

// SearchUsers finds users matching a query.
func (s *SearchService) SearchUsers(query string, currentUserID int64) ([]*SearchResult, error) {
	db := config.GetDB()
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, nil
	}

	users, err := model.SearchUsersByName(db, "", q, 20)
	if err != nil {
		return nil, err
	}

	var results []*SearchResult
	for _, user := range users {
		results = append(results, &SearchResult{
			Type:        "user",
			ID:          user.Name, // Use name as ID for display
			Name:        user.Name,
			Description: user.Bio,
		})
	}

	return results, nil
}

// GlobalSearch searches across all types.
func (s *SearchService) GlobalSearch(query string, userID int64) (map[string][]*SearchResult, error) {
	drops, _ := s.SearchDrops(query, userID)
	files, _ := s.SearchFiles(query, userID)
	users, _ := s.SearchUsers(query, userID)

	return map[string][]*SearchResult{
		"drops": drops,
		"files": files,
		"users": users,
	}, nil
}

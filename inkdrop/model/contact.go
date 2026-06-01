package model

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

// Contact represents a bidirectional contact relationship between two users
type Contact struct {
	ID          int64  `bun:",pk,autoincrement"`
	UserID      int64  `bun:",notnull"` // The user who added the contact
	ContactID   int64  `bun:",notnull"` // The user being added as a contact
	AddedAt     int64  `bun:",notnull"` // Unix timestamp
	UserName    string `bun:"-"`        // Populated from User.Name, not stored
	ContactName string `bun:"-"`        // Populated from User.Name, not stored
}

func ModelContact(db *bun.DB) error {
	ctx := context.Background()
	_, err := db.NewCreateTable().
		Model((*Contact)(nil)).
		IfNotExists().
		Exec(ctx)
	if err != nil {
		return err
	}

	// Create index for faster lookups
	_, err = db.Exec("CREATE INDEX IF NOT EXISTS idx_contact_userid ON contacts (user_id)")
	if err != nil {
		fmt.Println("Warning: Could not create index on user_id:", err)
	}

	_, err = db.Exec("CREATE INDEX IF NOT EXISTS idx_contact_userid_contactid ON contacts (user_id, contact_id)")
	if err != nil {
		fmt.Println("Warning: Could not create index on user_id, contact_id:", err)
	}

	return nil
}

// AddContact adds a contact for a user (one-directional)
// The adding user will see the contact in their contact list
func AddContact(db *bun.DB, userID int64, contactID int64) error {
	if userID == contactID {
		return fmt.Errorf("cannot add self as contact")
	}

	ctx := context.Background()

	// Check if contact already exists
	var existing Contact
	err := db.NewSelect().
		Model(&existing).
		Where("user_id = ? AND contact_id = ?", userID, contactID).
		Scan(ctx)
	if err == nil {
		return fmt.Errorf("contact already exists")
	}

	// Check both users exist
	user1, err := GetUserByID(int(userID), db)
	if err != nil || user1 == nil {
		return fmt.Errorf("user not found")
	}
	user2, err := GetUserByID(int(contactID), db)
	if err != nil || user2 == nil {
		return fmt.Errorf("contact user not found")
	}

	contact := &Contact{
		UserID:    userID,
		ContactID: contactID,
		AddedAt:   getCurrentTimestamp(),
	}

	_, err = db.NewInsert().Model(contact).Exec(ctx)
	return err
}

// RemoveContact removes a contact for a user
func RemoveContact(db *bun.DB, userID int64, contactID int64) error {
	ctx := context.Background()
	_, err := db.NewDelete().
		Model((*Contact)(nil)).
		Where("user_id = ? AND contact_id = ?", userID, contactID).
		Exec(ctx)
	return err
}

// GetUserContacts returns all contacts for a given user, populated with user details
func GetUserContacts(db *bun.DB, userID int64) ([]Contact, error) {
	var contacts []Contact
	ctx := context.Background()

	err := db.NewSelect().
		Model(&contacts).
		Where("user_id = ?", userID).
		Order("added_at DESC").
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	// Populate contact names
	for i, c := range contacts {
		contactUser, err := GetUserByID(int(c.ContactID), db)
		if err == nil && contactUser != nil {
			contacts[i].ContactName = contactUser.Name
		}
	}

	return contacts, nil
}

// SearchUsers returns users matching a search term (for adding contacts)
// Excludes the current user and already-added contacts
func SearchUsers(db *bun.DB, currentUserID int64, searchTerm string) ([]User, error) {
	var users []User
	ctx := context.Background()

	// Get list of already-added contacts
	existingContacts, _ := GetUserContacts(db, currentUserID)
	existingIDs := make([]int64, 0)
	for _, c := range existingContacts {
		existingIDs = append(existingIDs, c.ContactID)
	}
	existingIDs = append(existingIDs, currentUserID) // Exclude self

	// Build dynamic query
	query := db.NewSelect().
		Model(&users).
		Where("(name LIKE ? OR email LIKE ?)", "%"+searchTerm+"%", "%"+searchTerm+"%").
		Limit(20)

	// Add exclusion clause
	if len(existingIDs) > 0 {
		query = query.Where("id NOT IN (?)", bun.In(existingIDs))
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, err
	}

	return users, nil
}

func getCurrentTimestamp() int64 {
	return time.Now().Unix()
}

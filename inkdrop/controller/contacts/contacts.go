package contacts

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"inkdrop/config"
	"inkdrop/model"
)

// AddContact adds a user to the current user's contact list
func AddContact(w http.ResponseWriter, r *http.Request) {
	ss := config.GetSessionManager()
	isLoggedIn := ss.GetBool(r.Context(), "isLoggedIn")
	currentUsername := ss.GetString(r.Context(), "name")

	if !isLoggedIn || currentUsername == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	}
	r.ParseForm()
	contactUsername := strings.TrimSpace(r.FormValue("contact"))
	if contactUsername == "" {
		http.Error(w, "Contact username required", http.StatusBadRequest)
		return
	}

	db := config.GetDB()

	// Get current user
	currentUser, err := model.GetUserByName(currentUsername, db)
	if err != nil || currentUser == nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Get contact user
	contactUser, err := model.GetUserByName(contactUsername, db)
	if err != nil || contactUser == nil {
		http.Error(w, "Contact user not found", http.StatusNotFound)
		return
	}

	// Add contact
	err = model.AddContact(db, currentUser.ID, contactUser.ID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to add contact: %v", err), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `{"success":true}`)
}

// RemoveContact removes a user from the current user's contact list
func RemoveContact(w http.ResponseWriter, r *http.Request) {
	ss := config.GetSessionManager()
	isLoggedIn := ss.GetBool(r.Context(), "isLoggedIn")
	currentUsername := ss.GetString(r.Context(), "name")

	if !isLoggedIn || currentUsername == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	r.ParseForm()
	contactUsername := strings.TrimSpace(r.FormValue("contact"))
	if contactUsername == "" {
		http.Error(w, "Contact username required", http.StatusBadRequest)
		return
	}

	db := config.GetDB()

	// Get current user
	currentUser, err := model.GetUserByName(currentUsername, db)
	if err != nil || currentUser == nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Get contact user
	contactUser, err := model.GetUserByName(contactUsername, db)
	if err != nil || contactUser == nil {
		http.Error(w, "Contact user not found", http.StatusNotFound)
		return
	}

	// Remove contact
	err = model.RemoveContact(db, currentUser.ID, contactUser.ID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to remove contact: %v", err), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `{"success":true}`)
}

// ListContacts returns all contacts for the current user
func ListContacts(w http.ResponseWriter, r *http.Request) {
	ss := config.GetSessionManager()
	isLoggedIn := ss.GetBool(r.Context(), "isLoggedIn")
	currentUsername := ss.GetString(r.Context(), "name")

	if !isLoggedIn || currentUsername == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	db := config.GetDB()

	// Get current user
	currentUser, err := model.GetUserByName(currentUsername, db)
	if err != nil || currentUser == nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Get contacts
	contacts, err := model.GetUserContacts(db, currentUser.ID)
	if err != nil {
		http.Error(w, "Failed to retrieve contacts", http.StatusInternalServerError)
		return
	}

	type contactResponse struct {
		Name string `json:"name"`
		Bio  string `json:"bio,omitempty"`
		PFP  string `json:"pfp,omitempty"`
	}

	response := make([]contactResponse, 0, len(contacts))
	for _, c := range contacts {
		contactUser, err := model.GetUserByID(int(c.ContactID), db)
		if err != nil || contactUser == nil {
			continue
		}
		response = append(response, contactResponse{
			Name: contactUser.Name,
			Bio:  contactUser.Bio,
			PFP:  contactUser.PFP,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// SearchUsers searches for users to add as contacts
func SearchUsers(w http.ResponseWriter, r *http.Request) {
	ss := config.GetSessionManager()
	isLoggedIn := ss.GetBool(r.Context(), "isLoggedIn")
	currentUsername := ss.GetString(r.Context(), "name")

	if !isLoggedIn || currentUsername == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	searchTerm := strings.TrimSpace(r.URL.Query().Get("q"))
	if searchTerm == "" || len(searchTerm) < 2 {
		http.Error(w, "Search term must be at least 2 characters", http.StatusBadRequest)
		return
	}

	db := config.GetDB()

	// Get current user
	currentUser, err := model.GetUserByName(currentUsername, db)
	if err != nil || currentUser == nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Search users
	users, err := model.SearchUsers(db, currentUser.ID, searchTerm)
	if err != nil {
		http.Error(w, "Failed to search users", http.StatusInternalServerError)
		return
	}

	type searchResponse struct {
		Name string `json:"name"`
		Bio  string `json:"bio,omitempty"`
		PFP  string `json:"pfp,omitempty"`
	}

	response := make([]searchResponse, 0, len(users))
	for _, user := range users {
		response = append(response, searchResponse{
			Name: user.Name,
			Bio:  user.Bio,
			PFP:  user.PFP,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetUserProfile returns public profile information for a user
func GetUserProfile(w http.ResponseWriter, r *http.Request) {
	username := strings.TrimSpace(r.URL.Query().Get("user"))
	if username == "" {
		http.Error(w, "User parameter required", http.StatusBadRequest)
		return
	}

	db := config.GetDB()
	user, err := model.GetUserByName(username, db)
	if err != nil || user == nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"name": user.Name,
		"bio":  user.Bio,
		"pfp":  user.PFP,
	})
}

// GetContactsForOwnerDropdown returns contacts for use in the owner dropdown
func GetContactsForOwnerDropdown(w http.ResponseWriter, r *http.Request) {
	ss := config.GetSessionManager()
	isLoggedIn := ss.GetBool(r.Context(), "isLoggedIn")
	currentUsername := ss.GetString(r.Context(), "name")

	if !isLoggedIn || currentUsername == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	db := config.GetDB()

	// Get current user
	currentUser, err := model.GetUserByName(currentUsername, db)
	if err != nil || currentUser == nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Get contacts
	contacts, err := model.GetUserContacts(db, currentUser.ID)
	if err != nil {
		http.Error(w, "Failed to retrieve contacts", http.StatusInternalServerError)
		return
	}

	// Build response with contact info
	type ContactInfo struct {
		Name string `json:"name"`
		Bio  string `json:"bio,omitempty"`
		PFP  string `json:"pfp,omitempty"`
	}

	response := make([]ContactInfo, 0, len(contacts))
	for _, c := range contacts {
		contactUser, err := model.GetUserByID(int(c.ContactID), db)
		if err == nil && contactUser != nil {
			response = append(response, ContactInfo{
				Name: contactUser.Name,
				Bio:  contactUser.Bio,
				PFP:  contactUser.PFP,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

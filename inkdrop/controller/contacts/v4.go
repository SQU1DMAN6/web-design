package contacts

import (
	"encoding/json"
	"net/http"
	"strings"

	"inkdrop/apicommon"
	"inkdrop/config"
	"inkdrop/model"
)

// ListContactsV4 handles GET /api/v4/contacts
// Returns the current user's contacts with name, bio, and pfp.
func ListContactsV4(w http.ResponseWriter, r *http.Request) {
	userID := apicommon.GetUserID(r)
	if userID == 0 {
		apicommon.WriteError(w, http.StatusUnauthorized, "AUTH_REQUIRED", "Authentication required")
		return
	}

	db := config.GetDB()
	user, err := model.GetUserByID(int(userID), db)
	if err != nil || user == nil {
		apicommon.WriteError(w, http.StatusNotFound, "USER_NOT_FOUND", "User not found")
		return
	}

	contacts, err := model.ListMutualContacts(db, user.Name)
	if err != nil {
		apicommon.WriteError(w, http.StatusInternalServerError, "FETCH_ERROR", "Failed to retrieve contacts")
		return
	}

	type contactResponse struct {
		Name string `json:"name"`
		Bio  string `json:"bio,omitempty"`
		PFP  string `json:"pfp,omitempty"`
	}

	response := make([]contactResponse, 0, len(contacts))
	for _, name := range contacts {
		contactUser, err := model.GetUserByName(name, db)
		if err != nil || contactUser == nil {
			continue
		}
		response = append(response, contactResponse{
			Name: contactUser.Name,
			Bio:  contactUser.Bio,
			PFP:  model.ResolveProfilePicture(contactUser.PFP, contactUser.Name),
		})
	}

	apicommon.WriteSuccess(w, response)
}

// AddContactV4 handles POST /api/v4/contacts
// Adds a user as a contact by sending a contact request.
func AddContactV4(w http.ResponseWriter, r *http.Request) {
	userID := apicommon.GetUserID(r)
	if userID == 0 {
		apicommon.WriteError(w, http.StatusUnauthorized, "AUTH_REQUIRED", "Authentication required")
		return
	}

	var req struct {
		Username string `json:"username"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apicommon.WriteError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid request body")
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" {
		apicommon.WriteError(w, http.StatusBadRequest, "MISSING_USERNAME", "Username is required")
		return
	}

	db := config.GetDB()
	user, err := model.GetUserByID(int(userID), db)
	if err != nil || user == nil {
		apicommon.WriteError(w, http.StatusNotFound, "USER_NOT_FOUND", "User not found")
		return
	}

	if err := model.RequestContact(db, user.Name, req.Username); err != nil {
		apicommon.WriteError(w, http.StatusBadRequest, "CONTACT_ERROR", err.Error())
		return
	}

	apicommon.WriteJSON(w, http.StatusCreated, apicommon.APIResponse{Success: true, Data: map[string]string{"status": "request_sent"}})
}

// RemoveContactV4 handles DELETE /api/v4/contacts/{name}
func RemoveContactV4(w http.ResponseWriter, r *http.Request) {
	userID := apicommon.GetUserID(r)
	if userID == 0 {
		apicommon.WriteError(w, http.StatusUnauthorized, "AUTH_REQUIRED", "Authentication required")
		return
	}

	contactName := strings.TrimSpace(r.PathValue("name"))
	if contactName == "" {
		apicommon.WriteError(w, http.StatusBadRequest, "MISSING_USERNAME", "Contact username is required")
		return
	}

	db := config.GetDB()
	user, err := model.GetUserByID(int(userID), db)
	if err != nil || user == nil {
		apicommon.WriteError(w, http.StatusNotFound, "USER_NOT_FOUND", "User not found")
		return
	}

	if err := model.RemoveAcceptedContact(db, user.Name, contactName); err != nil {
		apicommon.WriteError(w, http.StatusBadRequest, "CONTACT_ERROR", err.Error())
		return
	}

	apicommon.WriteSuccess(w, map[string]string{"status": "removed"})
}

// SearchUsersV4 handles GET /api/v4/contacts/search?q=
// Searches for users to add as contacts.
func SearchUsersV4(w http.ResponseWriter, r *http.Request) {
	userID := apicommon.GetUserID(r)
	if userID == 0 {
		apicommon.WriteError(w, http.StatusUnauthorized, "AUTH_REQUIRED", "Authentication required")
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" || len(query) < 2 {
		apicommon.WriteError(w, http.StatusBadRequest, "MISSING_QUERY", "Search query must be at least 2 characters")
		return
	}

	db := config.GetDB()
	user, err := model.GetUserByID(int(userID), db)
	if err != nil || user == nil {
		apicommon.WriteError(w, http.StatusNotFound, "USER_NOT_FOUND", "User not found")
		return
	}

	users, err := model.SearchUsersByName(db, user.Name, query, 20)
	if err != nil {
		apicommon.WriteError(w, http.StatusInternalServerError, "SEARCH_ERROR", "Failed to search users")
		return
	}

	type userResponse struct {
		Name   string `json:"name"`
		Bio    string `json:"bio,omitempty"`
		PFP    string `json:"pfp,omitempty"`
		Status string `json:"status,omitempty"`
	}

	response := make([]userResponse, 0, len(users))
	for _, u := range users {
		response = append(response, userResponse{
			Name:   u.Name,
			Bio:    u.Bio,
			PFP:    model.ResolveProfilePicture(u.PFP, u.Name),
			Status: model.ContactStatus(db, user.Name, u.Name),
		})
	}

	apicommon.WriteSuccess(w, response)
}

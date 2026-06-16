package account

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"inkdrop/config"
	userModel "inkdrop/model"
	"inkdrop/repository"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const maxProfilePictureBytes int64 = 5 * 1024 * 1024

func CurrentAccountAPI(w http.ResponseWriter, r *http.Request) {
	sessionUser, ok := requireUser(w, r)
	if !ok {
		return
	}

	db := config.GetDB()
	user, err := userModel.GetUserByName(sessionUser, db)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]interface{}{"success": false, "error": "user not found"})
		return
	}

	incoming, outgoing, err := userModel.ListContactRequests(db, sessionUser)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	contacts, err := userModel.ListMutualContacts(db, sessionUser)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"user":     userPayload(*user, ""),
		"incoming": contactPayloads(incoming),
		"outgoing": contactPayloads(outgoing),
		"contacts": contacts,
	})
}

func UpdateAccountSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	sessionUser, ok := requireUser(w, r)
	if !ok {
		return
	}

	if err := r.ParseMultipartForm(maxProfilePictureBytes + 1024*1024); err != nil {
		http.Error(w, "Failed to parse account settings form", http.StatusBadRequest)
		return
	}

	db := config.GetDB()
	currentUser, err := userModel.GetUserByName(sessionUser, db)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	bio := strings.TrimSpace(r.FormValue("bio"))
	if len(bio) > 512 {
		bio = bio[:512]
	}

	newPFP := ""
	file, header, err := r.FormFile("profile_picture")
	if err == nil {
		defer file.Close()
		savedPath, saveErr := saveProfilePicture(sessionUser, file, header.Filename, currentUser.PFP)
		if saveErr != nil {
			http.Error(w, saveErr.Error(), http.StatusBadRequest)
			return
		}
		newPFP = savedPath
	}

	if err := userModel.UpdateUserProfile(db, sessionUser, bio, newPFP); err != nil {
		http.Error(w, "Failed to update account settings", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func CommunityAPI(w http.ResponseWriter, r *http.Request) {
	sessionUser, ok := requireUser(w, r)
	if !ok {
		return
	}

	db := config.GetDB()
	users, err := userModel.SearchUsersByName(db, sessionUser, r.URL.Query().Get("q"), 20)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	payload := make([]map[string]interface{}, 0, len(users))
	for _, user := range users {
		payload = append(payload, userPayload(user, userModel.ContactStatus(db, sessionUser, user.Name)))
	}

	incoming, outgoing, err := userModel.ListContactRequests(db, sessionUser)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"users":    payload,
		"incoming": contactPayloads(incoming),
		"outgoing": contactPayloads(outgoing),
	})
}

func ContactsAPI(w http.ResponseWriter, r *http.Request) {
	sessionUser, ok := requireUser(w, r)
	if !ok {
		return
	}
	contacts, err := userModel.ListMutualContacts(config.GetDB(), sessionUser)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "contacts": contacts})
}

func RequestContact(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	sessionUser, ok := requireUser(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "invalid form"})
		return
	}
	recipient := strings.TrimSpace(r.FormValue("recipient"))
	if err := userModel.RequestContact(config.GetDB(), sessionUser, recipient); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

func RespondContact(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	sessionUser, ok := requireUser(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "invalid form"})
		return
	}
	id, err := strconv.ParseInt(strings.TrimSpace(r.FormValue("id")), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "invalid request id"})
		return
	}
	action := strings.TrimSpace(r.FormValue("action"))
	if action != "accept" && action != "decline" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "invalid action"})
		return
	}
	if err := userModel.RespondToContact(config.GetDB(), id, sessionUser, action == "accept"); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

func requireUser(w http.ResponseWriter, r *http.Request) (string, bool) {
	SS := config.GetSessionManager()
	sessionUser := SS.GetString(r.Context(), "name")
	if SS.GetBool(r.Context(), "isLoggedIn") != true || sessionUser == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{"success": false, "error": "login required"})
		return "", false
	}
	return sessionUser, true
}

func saveProfilePicture(username string, src io.Reader, originalName string, oldPFP string) (string, error) {
	data, err := io.ReadAll(io.LimitReader(src, maxProfilePictureBytes+1))
	if err != nil {
		return "", fmt.Errorf("failed to read profile picture")
	}
	if int64(len(data)) > maxProfilePictureBytes {
		return "", fmt.Errorf("profile picture must be smaller than 5 MB")
	}

	contentType := http.DetectContentType(data)
	ext := ""
	switch contentType {
	case "image/png":
		ext = ".png"
	case "image/jpeg":
		ext = ".jpg"
	case "image/gif":
		ext = ".gif"
	case "image/webp":
		ext = ".webp"
	default:
		return "", fmt.Errorf("profile picture must be PNG, JPEG, GIF, or WebP")
	}

	if _, _, err := image.DecodeConfig(bytes.NewReader(data)); err != nil {
		// WebP is not supported by the standard decoder, so rely on content sniffing.
		if contentType != "image/webp" {
			return "", fmt.Errorf("profile picture is not a valid image")
		}
	}

	userDir := filepath.Join(repository.UserPFPDir, username)
	if err := os.MkdirAll(userDir, 0755); err != nil {
		return "", fmt.Errorf("failed to prepare profile picture directory")
	}

	filename := "profile" + ext
	if strings.TrimSpace(originalName) != "" {
		base := strings.TrimSuffix(filepath.Base(originalName), filepath.Ext(originalName))
		base = strings.Map(func(r rune) rune {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
				return r
			}
			return '_'
		}, base)
		if base != "" {
			filename = base + ext
		}
	}
	target := filepath.Join(userDir, filename)
	if err := os.WriteFile(target, data, 0644); err != nil {
		return "", fmt.Errorf("failed to save profile picture")
	}

	removeOldProfilePicture(username, oldPFP, target)
	return "/pfp/" + username + "/" + filename, nil
}

func removeOldProfilePicture(username string, oldPFP string, newTarget string) {
	oldPFP = strings.TrimSpace(oldPFP)
	if oldPFP == "" || oldPFP == "/pfp/default.png" || oldPFP == "default.png" || oldPFP == "/default.png" {
		return
	}

	var oldPath string
	if strings.HasPrefix(oldPFP, "/pfp/") {
		oldPath = filepath.Join(repository.UserPFPDir, strings.TrimPrefix(oldPFP, "/pfp/"))
	} else if strings.HasPrefix(oldPFP, "/ftr/userData/") || strings.HasPrefix(oldPFP, "/ftr/userpfp/") {
		oldPath = filepath.Clean(oldPFP)
	} else {
		return
	}

	if filepath.Clean(oldPath) == filepath.Clean(newTarget) {
		return
	}
	_ = os.Remove(oldPath)
}

func userPayload(user userModel.User, status string) map[string]interface{} {
	return map[string]interface{}{
		"name":   user.Name,
		"bio":    user.Bio,
		"pfp":    userModel.ResolveProfilePicture(user.PFP, user.Name),
		"status": status,
	}
}

func contactPayloads(contacts []userModel.ContactRequest) []map[string]interface{} {
	payload := make([]map[string]interface{}, 0, len(contacts))
	for _, contact := range contacts {
		payload = append(payload, map[string]interface{}{
			"id":        contact.ID,
			"requester": contact.Requester,
			"recipient": contact.Recipient,
			"status":    contact.Status,
		})
	}
	return payload
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func DefaultPFP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		return
	}

	http.ServeFile(w, r, "/ftr/userData/default.png")
}

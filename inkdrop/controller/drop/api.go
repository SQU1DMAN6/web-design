package drop

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"inkdrop/apicommon"
	"inkdrop/model"
	"inkdrop/service"
)

var (
	dropService *service.DropService
)

// Init sets up the controller with required services.
func Init(ds *service.DropService) {
	dropService = ds
}

// ListDrops handles GET /api/v4/drops
func ListDrops(w http.ResponseWriter, r *http.Request) {
	userName := apicommon.GetUserName(r)
	fmt.Fprintf(os.Stderr, "[listdrops] context_user=%q session_user=%q\n", userName, "")
	if userName == "" {
		apicommon.WriteError(w, http.StatusUnauthorized, "AUTH_REQUIRED", "Authentication required")
		return
	}

	drops, err := dropService.GetUserDrops(userName)
	if err != nil {
		apicommon.WriteError(w, http.StatusInternalServerError, "FETCH_ERROR", err.Error())
		return
	}

	apicommon.WriteSuccess(w, drops)
}

// CreateDrop handles POST /api/v4/drops
func CreateDrop(w http.ResponseWriter, r *http.Request) {
	userName := apicommon.GetUserName(r)
	if userName == "" {
		apicommon.WriteError(w, http.StatusUnauthorized, "AUTH_REQUIRED", "Authentication required")
		return
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apicommon.WriteError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid request body")
		return
	}

	if req.Name == "" {
		apicommon.WriteError(w, http.StatusBadRequest, "MISSING_NAME", "Drop name is required")
		return
	}

	drop, err := dropService.CreateDrop(req.Name, userName, req.Description)
	if err != nil {
		apicommon.WriteError(w, http.StatusInternalServerError, "CREATE_ERROR", err.Error())
		return
	}

	apicommon.WriteJSON(w, http.StatusCreated, apicommon.APIResponse{Success: true, Data: drop})
}

// GetDrop handles GET /api/v4/drop/{id}
func GetDrop(w http.ResponseWriter, r *http.Request) {
	dropID := r.PathValue("id")
	if dropID == "" {
		apicommon.WriteError(w, http.StatusBadRequest, "MISSING_ID", "Drop ID is required")
		return
	}

	drop, err := dropService.GetDrop(dropID)
	if err != nil {
		apicommon.WriteError(w, http.StatusNotFound, "NOT_FOUND", "Drop not found")
		return
	}

	apicommon.WriteSuccess(w, drop)
}

// UpdateDrop handles PUT /api/v4/drop/{id}
func UpdateDrop(w http.ResponseWriter, r *http.Request) {
	_ = apicommon.GetUserID(r) // verified by auth middleware
	dropID := r.PathValue("id")
	if dropID == "" {
		apicommon.WriteError(w, http.StatusBadRequest, "MISSING_ID", "Drop ID is required")
		return
	}

	var req struct {
		Description string               `json:"description"`
		Visibility  model.DropVisibility `json:"visibility"`
		Settings    model.DropSettings   `json:"settings"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apicommon.WriteError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid request body")
		return
	}

	if err := dropService.UpdateDropSettings(dropID, req.Description, req.Visibility, req.Settings); err != nil {
		apicommon.WriteError(w, http.StatusInternalServerError, "UPDATE_ERROR", err.Error())
		return
	}

	apicommon.WriteSuccess(w, map[string]string{"status": "updated"})
}

// DeleteDrop handles DELETE /api/v4/drop/{id}
func DeleteDrop(w http.ResponseWriter, r *http.Request) {
	userID := apicommon.GetUserID(r)
	dropID := r.PathValue("id")
	if dropID == "" {
		apicommon.WriteError(w, http.StatusBadRequest, "MISSING_ID", "Drop ID is required")
		return
	}

	if err := dropService.DeleteDrop(dropID, userID); err != nil {
		apicommon.WriteError(w, http.StatusForbidden, "DELETE_ERROR", err.Error())
		return
	}

	apicommon.WriteSuccess(w, map[string]string{"status": "deleted"})
}

// MigrateDrop handles POST /api/v4/drops/migrate
func MigrateDrop(w http.ResponseWriter, r *http.Request) {
	userName := apicommon.GetUserName(r)
	if userName == "" {
		apicommon.WriteError(w, http.StatusUnauthorized, "AUTH_REQUIRED", "Authentication required")
		return
	}

	var req struct {
		DropName string `json:"drop_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apicommon.WriteError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid request body")
		return
	}

	drop, err := dropService.MigrateFromLegacy(userName, req.DropName)
	if err != nil {
		apicommon.WriteError(w, http.StatusInternalServerError, "MIGRATE_ERROR", err.Error())
		return
	}

	apicommon.WriteJSON(w, http.StatusCreated, apicommon.APIResponse{Success: true, Data: drop})
}
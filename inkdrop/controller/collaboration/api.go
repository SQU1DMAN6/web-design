package collaboration

import (
	"encoding/json"
	"net/http"
	"strconv"

	"inkdrop/apicommon"
	"inkdrop/config"
	"inkdrop/model"
	"inkdrop/service"
)

var (
	permService *service.PermissionService
)

// Init sets up the controller with required services.
func Init(ps *service.PermissionService) {
	permService = ps
}

// GetMembers handles GET /api/v4/drop/{id}/members
func GetMembers(w http.ResponseWriter, r *http.Request) {
	dropID := r.PathValue("id")
	if dropID == "" {
		apicommon.WriteError(w, http.StatusBadRequest, "MISSING_ID", "Drop ID is required")
		return
	}

	members, err := permService.GetMembers(dropID)
	if err != nil {
		apicommon.WriteError(w, http.StatusInternalServerError, "FETCH_ERROR", err.Error())
		return
	}

	apicommon.WriteSuccess(w, members)
}

// AddMember handles POST /api/v4/drop/{id}/members
func AddMember(w http.ResponseWriter, r *http.Request) {
	_ = apicommon.GetUserID(r) // verified by auth middleware
	dropID := r.PathValue("id")
	if dropID == "" {
		apicommon.WriteError(w, http.StatusBadRequest, "MISSING_ID", "Drop ID is required")
		return
	}

	var req struct {
		UserID   int64          `json:"user_id"`
		UserName string         `json:"user_name"`
		Role     model.DropRole `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apicommon.WriteError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid request body")
		return
	}

	// If user_name is provided, look up user ID
	targetID := req.UserID
	if req.UserName != "" && targetID == 0 {
		db := config.GetDB()
		user, err := model.GetUserByName(req.UserName, db)
		if err != nil {
			apicommon.WriteError(w, http.StatusNotFound, "USER_NOT_FOUND", "User not found")
			return
		}
		targetID = user.ID
	}

	if targetID == 0 {
		apicommon.WriteError(w, http.StatusBadRequest, "MISSING_USER", "User ID or name is required")
		return
	}

	if req.Role == "" {
		req.Role = model.DropRoleViewer
	}

	if err := permService.AddMember(dropID, targetID, req.Role); err != nil {
		apicommon.WriteError(w, http.StatusInternalServerError, "ADD_ERROR", err.Error())
		return
	}

	apicommon.WriteSuccess(w, map[string]string{"status": "member_added"})
}

// RemoveMember handles DELETE /api/v4/drop/{id}/members/{userId}
func RemoveMember(w http.ResponseWriter, r *http.Request) {
	dropID := r.PathValue("id")
	userIDStr := r.PathValue("userId")
	if dropID == "" || userIDStr == "" {
		apicommon.WriteError(w, http.StatusBadRequest, "MISSING_ID", "Drop ID and user ID are required")
		return
	}

	targetID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		apicommon.WriteError(w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user ID")
		return
	}

	if err := permService.RemoveMember(dropID, targetID); err != nil {
		apicommon.WriteError(w, http.StatusInternalServerError, "REMOVE_ERROR", err.Error())
		return
	}

	apicommon.WriteSuccess(w, map[string]string{"status": "member_removed"})
}

// ShareDrop handles POST /api/v4/drop/{id}/share
func ShareDrop(w http.ResponseWriter, r *http.Request) {
	dropID := r.PathValue("id")
	if dropID == "" {
		apicommon.WriteError(w, http.StatusBadRequest, "MISSING_ID", "Drop ID is required")
		return
	}

	var req struct {
		UserID     int64          `json:"user_id"`
		UserName   string         `json:"user_name"`
		Permission model.DropRole `json:"permission"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apicommon.WriteError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid request body")
		return
	}

	targetID := req.UserID
	if req.UserName != "" && targetID == 0 {
		db := config.GetDB()
		user, err := model.GetUserByName(req.UserName, db)
		if err != nil {
			apicommon.WriteError(w, http.StatusNotFound, "USER_NOT_FOUND", "User not found")
			return
		}
		targetID = user.ID
	}

	if targetID == 0 {
		apicommon.WriteError(w, http.StatusBadRequest, "MISSING_USER", "User ID or name is required")
		return
	}

	if req.Permission == "" {
		req.Permission = model.DropRoleViewer
	}

	if err := permService.AddMember(dropID, targetID, req.Permission); err != nil {
		apicommon.WriteError(w, http.StatusInternalServerError, "SHARE_ERROR", err.Error())
		return
	}

	apicommon.WriteSuccess(w, map[string]string{"status": "shared"})
}

// UpdateRole handles PUT /api/v4/drop/{id}/members/{userId}/role
func UpdateRole(w http.ResponseWriter, r *http.Request) {
	actorID := apicommon.GetUserID(r)
	dropID := r.PathValue("id")
	userIDStr := r.PathValue("userId")
	if dropID == "" || userIDStr == "" {
		apicommon.WriteError(w, http.StatusBadRequest, "MISSING_ID", "Drop ID and user ID are required")
		return
	}

	targetID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		apicommon.WriteError(w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid user ID")
		return
	}

	var req struct {
		Role model.DropRole `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apicommon.WriteError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid request body")
		return
	}

	if err := permService.UpdateRole(dropID, actorID, targetID, req.Role); err != nil {
		apicommon.WriteError(w, http.StatusForbidden, "ROLE_ERROR", err.Error())
		return
	}

	apicommon.WriteSuccess(w, map[string]string{"status": "role_updated"})
}

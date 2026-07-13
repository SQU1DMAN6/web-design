package file

import (
	"encoding/json"
	"io"
	"net/http"

	"inkdrop/apicommon"
	"inkdrop/service"
)

var (
	fileService *service.FileService
)

// Init sets up the controller with required services.
func Init(fs *service.FileService) {
	fileService = fs
}

// ListFiles handles GET /api/v4/drop/{id}/files
func ListFiles(w http.ResponseWriter, r *http.Request) {
	dropID := r.PathValue("id")
	if dropID == "" {
		apicommon.WriteError(w, http.StatusBadRequest, "MISSING_ID", "Drop ID is required")
		return
	}

	files, err := fileService.ListFiles(dropID, "/")
	if err != nil {
		apicommon.WriteError(w, http.StatusInternalServerError, "FETCH_ERROR", err.Error())
		return
	}

	apicommon.WriteSuccess(w, files)
}

// CreateFile handles POST /api/v4/drop/{id}/files
func CreateFile(w http.ResponseWriter, r *http.Request) {
	userID := apicommon.GetUserID(r)
	dropID := r.PathValue("id")
	if dropID == "" {
		apicommon.WriteError(w, http.StatusBadRequest, "MISSING_ID", "Drop ID is required")
		return
	}

	var req struct {
		Name    string `json:"name"`
		Type    string `json:"type"`
		Path    string `json:"path"`
		Content string `json:"content,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apicommon.WriteError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid request body")
		return
	}

	if req.Name == "" {
		apicommon.WriteError(w, http.StatusBadRequest, "MISSING_NAME", "File name is required")
		return
	}

	if req.Path == "" {
		req.Path = "/"
	}

	var data []byte
	if req.Content != "" {
		data = []byte(req.Content)
	}

	file, err := fileService.CreateFile(dropID, req.Name, req.Type, req.Path, userID, data)
	if err != nil {
		apicommon.WriteError(w, http.StatusInternalServerError, "CREATE_ERROR", err.Error())
		return
	}

	apicommon.WriteJSON(w, http.StatusCreated, apicommon.APIResponse{Success: true, Data: file})
}

// GetFile handles GET /api/v4/drop/{id}/files/{fileId}
func GetFile(w http.ResponseWriter, r *http.Request) {
	fileID := r.PathValue("fileId")
	if fileID == "" {
		apicommon.WriteError(w, http.StatusBadRequest, "MISSING_FILE_ID", "File ID is required")
		return
	}

	file, err := fileService.GetFile(fileID)
	if err != nil {
		apicommon.WriteError(w, http.StatusNotFound, "NOT_FOUND", "File not found")
		return
	}

	apicommon.WriteSuccess(w, file)
}

// ReadFileContent handles GET /api/v4/drop/{id}/files/{fileId}/content
func ReadFileContent(w http.ResponseWriter, r *http.Request) {
	dropID := r.PathValue("id")
	fileID := r.PathValue("fileId")
	if fileID == "" || dropID == "" {
		apicommon.WriteError(w, http.StatusBadRequest, "MISSING_ID", "Drop and file IDs are required")
		return
	}

	data, err := fileService.ReadFile(dropID, fileID)
	if err != nil {
		apicommon.WriteError(w, http.StatusNotFound, "NOT_FOUND", "File content not found")
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(data)
}

// UpdateFile handles PUT /api/v4/drop/{id}/files/{fileId}
func UpdateFile(w http.ResponseWriter, r *http.Request) {
	userID := apicommon.GetUserID(r)
	dropID := r.PathValue("id")
	fileID := r.PathValue("fileId")
	if fileID == "" || dropID == "" {
		apicommon.WriteError(w, http.StatusBadRequest, "MISSING_ID", "Drop and file IDs are required")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		apicommon.WriteError(w, http.StatusBadRequest, "READ_ERROR", "Failed to read request body")
		return
	}

	// Try to parse as JSON with content field, otherwise use raw body
	var req struct {
		Content string `json:"content"`
		Message string `json:"message"`
	}
	content := string(body)
	message := ""
	if err := json.Unmarshal(body, &req); err == nil {
		if req.Content != "" {
			content = req.Content
		}
		message = req.Message
	}

	file, err := fileService.UpdateFile(dropID, fileID, []byte(content), userID, message)
	if err != nil {
		apicommon.WriteError(w, http.StatusInternalServerError, "UPDATE_ERROR", err.Error())
		return
	}

	apicommon.WriteSuccess(w, file)
}

// DeleteFile handles DELETE /api/v4/drop/{id}/files/{fileId}
func DeleteFile(w http.ResponseWriter, r *http.Request) {
	userID := apicommon.GetUserID(r)
	dropID := r.PathValue("id")
	fileID := r.PathValue("fileId")
	if fileID == "" || dropID == "" {
		apicommon.WriteError(w, http.StatusBadRequest, "MISSING_ID", "Drop and file IDs are required")
		return
	}

	if err := fileService.DeleteFile(dropID, fileID, userID); err != nil {
		apicommon.WriteError(w, http.StatusInternalServerError, "DELETE_ERROR", err.Error())
		return
	}

	apicommon.WriteSuccess(w, map[string]string{"status": "deleted"})
}

// RenameFile handles POST /api/v4/drop/{id}/files/{fileId}/rename
func RenameFile(w http.ResponseWriter, r *http.Request) {
	userID := apicommon.GetUserID(r)
	dropID := r.PathValue("id")
	fileID := r.PathValue("fileId")
	if fileID == "" || dropID == "" {
		apicommon.WriteError(w, http.StatusBadRequest, "MISSING_ID", "Drop and file IDs are required")
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apicommon.WriteError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid request body")
		return
	}

	file, err := fileService.RenameFile(dropID, fileID, req.Name, userID)
	if err != nil {
		apicommon.WriteError(w, http.StatusInternalServerError, "RENAME_ERROR", err.Error())
		return
	}

	apicommon.WriteSuccess(w, file)
}

// MoveFile handles POST /api/v4/drop/{id}/files/{fileId}/move
func MoveFile(w http.ResponseWriter, r *http.Request) {
	userID := apicommon.GetUserID(r)
	dropID := r.PathValue("id")
	fileID := r.PathValue("fileId")
	if fileID == "" || dropID == "" {
		apicommon.WriteError(w, http.StatusBadRequest, "MISSING_ID", "Drop and file IDs are required")
		return
	}

	var req struct {
		Destination string `json:"destination"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apicommon.WriteError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid request body")
		return
	}

	file, err := fileService.MoveFile(dropID, fileID, req.Destination, userID)
	if err != nil {
		apicommon.WriteError(w, http.StatusInternalServerError, "MOVE_ERROR", err.Error())
		return
	}

	apicommon.WriteSuccess(w, file)
}

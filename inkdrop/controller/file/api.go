package file

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"inkdrop/apicommon"
	"inkdrop/model"
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
// Supports ?path=/folder/subfolder to list files in a specific directory.
func ListFiles(w http.ResponseWriter, r *http.Request) {
	dropID := r.PathValue("id")
	if dropID == "" {
		apicommon.WriteError(w, http.StatusBadRequest, "MISSING_ID", "Drop ID is required")
		return
	}

	relPath := r.URL.Query().Get("path")
	if relPath == "" {
		relPath = "/"
	}

	files, err := fileService.ListFiles(dropID, relPath)
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
		status := http.StatusInternalServerError
		code := "CREATE_ERROR"
		if strings.Contains(err.Error(), "already exists") {
			status = http.StatusConflict
			code = "CONFLICT"
		} else if strings.Contains(err.Error(), "name") || strings.Contains(err.Error(), "invalid") {
			status = http.StatusBadRequest
			code = "INVALID_NAME"
		}
		apicommon.WriteError(w, status, code, err.Error())
		return
	}

	apicommon.WriteJSON(w, http.StatusCreated, apicommon.APIResponse{Success: true, Data: file})
}

// CreateFolder handles POST /api/v4/drop/{id}/folders
func CreateFolder(w http.ResponseWriter, r *http.Request) {
	userID := apicommon.GetUserID(r)
	dropID := r.PathValue("id")
	if dropID == "" {
		apicommon.WriteError(w, http.StatusBadRequest, "MISSING_ID", "Drop ID is required")
		return
	}

	var req struct {
		Name   string `json:"name"`
		Parent string `json:"parent"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apicommon.WriteError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid request body")
		return
	}

	if req.Name == "" {
		apicommon.WriteError(w, http.StatusBadRequest, "MISSING_NAME", "Folder name is required")
		return
	}

	if req.Parent == "" {
		req.Parent = "/"
	}

	dir, err := fileService.CreateDirectory(dropID, req.Name, req.Parent, userID)
	if err != nil {
		status := http.StatusInternalServerError
		code := "CREATE_ERROR"
		if strings.Contains(err.Error(), "already exists") {
			status = http.StatusConflict
			code = "CONFLICT"
		} else if strings.Contains(err.Error(), "name") || strings.Contains(err.Error(), "invalid") {
			status = http.StatusBadRequest
			code = "INVALID_NAME"
		}
		apicommon.WriteError(w, status, code, err.Error())
		return
	}

	apicommon.WriteJSON(w, http.StatusCreated, apicommon.APIResponse{Success: true, Data: dir})
}

// UploadFile handles POST /api/v4/drop/{id}/upload
// Multipart form: file=@filename, path=/destination/dir
func UploadFile(w http.ResponseWriter, r *http.Request) {
	userID := apicommon.GetUserID(r)
	dropID := r.PathValue("id")
	if dropID == "" {
		apicommon.WriteError(w, http.StatusBadRequest, "MISSING_ID", "Drop ID is required")
		return
	}

	// Parse multipart form with max upload size
	if err := r.ParseMultipartForm(service.MaxUploadSize); err != nil {
		apicommon.WriteError(w, http.StatusBadRequest, "UPLOAD_ERROR", "Failed to parse upload: "+err.Error())
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		apicommon.WriteError(w, http.StatusBadRequest, "MISSING_FILE", "File field 'file' is required")
		return
	}
	defer file.Close()

	parentPath := r.FormValue("path")
	if parentPath == "" {
		parentPath = "/"
	}

	// Validate filename
	name := filepath.Base(header.Filename)
	if name == "." || name == ".." || strings.ContainsAny(name, "/\\") {
		apicommon.WriteError(w, http.StatusBadRequest, "INVALID_NAME", "Invalid filename")
		return
	}

	uploaded, err := fileService.UploadFile(dropID, name, parentPath, userID, file, header.Size)
	if err != nil {
		status := http.StatusInternalServerError
		code := "UPLOAD_ERROR"
		if strings.Contains(err.Error(), "too large") {
			status = http.StatusRequestEntityTooLarge
			code = "UPLOAD_TOO_LARGE"
		} else if strings.Contains(err.Error(), "already exists") {
			status = http.StatusConflict
			code = "CONFLICT"
		} else if strings.Contains(err.Error(), "name") || strings.Contains(err.Error(), "invalid") {
			status = http.StatusBadRequest
			code = "INVALID_NAME"
		}
		apicommon.WriteError(w, status, code, err.Error())
		return
	}

	apicommon.WriteJSON(w, http.StatusCreated, apicommon.APIResponse{Success: true, Data: uploaded})
}

// GetFile handles GET /api/v4/drop/{id}/files/{fileId}
func GetFile(w http.ResponseWriter, r *http.Request) {
	dropID := r.PathValue("id")
	fileID := r.PathValue("fileId")
	if fileID == "" || dropID == "" {
		apicommon.WriteError(w, http.StatusBadRequest, "MISSING_ID", "Drop and file IDs are required")
		return
	}

	file, err := fileService.GetFileInDrop(dropID, fileID)
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

// DownloadFile handles GET /api/v4/drop/{id}/files/{fileId}/download
func DownloadFile(w http.ResponseWriter, r *http.Request) {
	dropID := r.PathValue("id")
	fileID := r.PathValue("fileId")
	if fileID == "" || dropID == "" {
		apicommon.WriteError(w, http.StatusBadRequest, "MISSING_ID", "Drop and file IDs are required")
		return
	}

	file, err := fileService.GetFileInDrop(dropID, fileID)
	if err != nil {
		apicommon.WriteError(w, http.StatusNotFound, "NOT_FOUND", "File not found")
		return
	}

	if file.Type == model.FileTypeFolder {
		apicommon.WriteError(w, http.StatusBadRequest, "IS_DIRECTORY", "Cannot download a directory")
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", file.Name))
	w.Header().Set("Content-Type", fileService.GetFileMimeType(file))
	w.Header().Set("X-Content-Type-Options", "nosniff")

	if err := fileService.StreamFile(dropID, fileID, w); err != nil {
		apicommon.WriteError(w, http.StatusInternalServerError, "DOWNLOAD_ERROR", "Failed to stream file")
		return
	}
}

// PreviewFile handles GET /api/v4/drop/{id}/files/{fileId}/preview
func PreviewFile(w http.ResponseWriter, r *http.Request) {
	dropID := r.PathValue("id")
	fileID := r.PathValue("fileId")
	if fileID == "" || dropID == "" {
		apicommon.WriteError(w, http.StatusBadRequest, "MISSING_ID", "Drop and file IDs are required")
		return
	}

	file, err := fileService.GetFileInDrop(dropID, fileID)
	if err != nil {
		apicommon.WriteError(w, http.StatusNotFound, "NOT_FOUND", "File not found")
		return
	}

	if file.Type == model.FileTypeFolder {
		apicommon.WriteError(w, http.StatusBadRequest, "IS_DIRECTORY", "Cannot preview a directory")
		return
	}

	w.Header().Set("Content-Type", fileService.GetFileMimeType(file))
	w.Header().Set("Content-Disposition", "inline")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	if err := fileService.StreamFile(dropID, fileID, w); err != nil {
		apicommon.WriteError(w, http.StatusInternalServerError, "PREVIEW_ERROR", "Failed to stream file")
		return
	}
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

// TrashFile handles POST /api/v4/drop/{id}/files/{fileId}/trash
func TrashFile(w http.ResponseWriter, r *http.Request) {
	userID := apicommon.GetUserID(r)
	dropID := r.PathValue("id")
	fileID := r.PathValue("fileId")
	if fileID == "" || dropID == "" {
		apicommon.WriteError(w, http.StatusBadRequest, "MISSING_ID", "Drop and file IDs are required")
		return
	}

	if err := fileService.TrashFile(dropID, fileID, userID); err != nil {
		apicommon.WriteError(w, http.StatusInternalServerError, "TRASH_ERROR", err.Error())
		return
	}

	apicommon.WriteSuccess(w, map[string]string{"status": "trashed"})
}

// RestoreFile handles POST /api/v4/drop/{id}/files/{fileId}/restore
func RestoreFile(w http.ResponseWriter, r *http.Request) {
	userID := apicommon.GetUserID(r)
	dropID := r.PathValue("id")
	fileID := r.PathValue("fileId")
	if fileID == "" || dropID == "" {
		apicommon.WriteError(w, http.StatusBadRequest, "MISSING_ID", "Drop and file IDs are required")
		return
	}

	file, err := fileService.RestoreFile(dropID, fileID, userID)
	if err != nil {
		apicommon.WriteError(w, http.StatusInternalServerError, "RESTORE_ERROR", err.Error())
		return
	}

	apicommon.WriteSuccess(w, file)
}

// ListTrash handles GET /api/v4/drop/{id}/trash
func ListTrash(w http.ResponseWriter, r *http.Request) {
	dropID := r.PathValue("id")
	if dropID == "" {
		apicommon.WriteError(w, http.StatusBadRequest, "MISSING_ID", "Drop ID is required")
		return
	}

	trashed, err := fileService.ListTrash(dropID)
	if err != nil {
		apicommon.WriteError(w, http.StatusInternalServerError, "FETCH_ERROR", err.Error())
		return
	}

	apicommon.WriteSuccess(w, trashed)
}

// EmptyTrash handles POST /api/v4/drop/{id}/trash/empty
func EmptyTrash(w http.ResponseWriter, r *http.Request) {
	userID := apicommon.GetUserID(r)
	dropID := r.PathValue("id")
	if dropID == "" {
		apicommon.WriteError(w, http.StatusBadRequest, "MISSING_ID", "Drop ID is required")
		return
	}

	if err := fileService.EmptyTrash(dropID, userID); err != nil {
		apicommon.WriteError(w, http.StatusInternalServerError, "EMPTY_TRASH_ERROR", err.Error())
		return
	}

	apicommon.WriteSuccess(w, map[string]string{"status": "trash_emptied"})
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
		status := http.StatusInternalServerError
		code := "RENAME_ERROR"
		if strings.Contains(err.Error(), "already exists") {
			status = http.StatusConflict
			code = "CONFLICT"
		} else if strings.Contains(err.Error(), "name") || strings.Contains(err.Error(), "invalid") {
			status = http.StatusBadRequest
			code = "INVALID_NAME"
		}
		apicommon.WriteError(w, status, code, err.Error())
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
		status := http.StatusInternalServerError
		code := "MOVE_ERROR"
		if strings.Contains(err.Error(), "already exists") {
			status = http.StatusConflict
			code = "CONFLICT"
		} else if strings.Contains(err.Error(), "into itself") {
			status = http.StatusBadRequest
			code = "INVALID_MOVE"
		}
		apicommon.WriteError(w, status, code, err.Error())
		return
	}

	apicommon.WriteSuccess(w, file)
}

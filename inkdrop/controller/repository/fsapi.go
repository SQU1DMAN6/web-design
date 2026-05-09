package repository

import (
	"encoding/json"
	"fmt"
	"inkdrop/config"
	repoStore "inkdrop/repository"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

type fsRenameRequest struct {
	NewPath string `json:"newPath"`
}

func RepositoryFSAPI(w http.ResponseWriter, r *http.Request) {
	repoOwner := strings.TrimSpace(chi.URLParam(r, "user"))
	repoName := strings.TrimSpace(chi.URLParam(r, "reponame"))
	itemPath := normalizeAPIRepoPath(chi.URLParam(r, "*"))
	if repoOwner == "" || repoName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "user and repo are required"})
		return
	}

	_, canRead, canWrite := fsAPIPermissions(r, repoOwner, repoName)
	if !canRead {
		writeJSON(w, http.StatusForbidden, map[string]interface{}{"success": false, "error": "permission denied"})
		return
	}
	if requiresFSWrite(r) && !canWrite {
		writeJSON(w, http.StatusForbidden, map[string]interface{}{"success": false, "error": "write permission denied"})
		return
	}

	switch r.Method {
	case http.MethodGet, http.MethodHead:
		handleFSAPIGet(w, r, repoOwner, repoName, itemPath)
	case http.MethodPut:
		handleFSAPIPut(w, r, repoOwner, repoName, itemPath)
	case http.MethodDelete:
		handleFSAPIDelete(w, repoOwner, repoName, itemPath)
	case http.MethodPost:
		handleFSAPIPost(w, r, repoOwner, repoName, itemPath)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{"success": false, "error": "method not allowed"})
	}
}

func handleFSAPIGet(w http.ResponseWriter, r *http.Request, repoOwner string, repoName string, itemPath string) {
	if r.URL.Query().Get("list") == "1" {
		entries, err := repoStore.ListRepositoryEntriesRecursive(repoOwner, repoName)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]interface{}{"success": false, "error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "entries": entries})
		return
	}

	if itemPath == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "file path is required"})
		return
	}
	filePath, err := repoStore.GetItemPath(repoOwner, repoName, "/", itemPath)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "invalid file path"})
		return
	}
	file, err := os.Open(filePath)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]interface{}{"success": false, "error": "file not found"})
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "path is not a regular file"})
		return
	}

	w.Header().Set("Content-Disposition", "inline; filename="+strconv.Quote(path.Base(itemPath)))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
}

func handleFSAPIPut(w http.ResponseWriter, r *http.Request, repoOwner string, repoName string, itemPath string) {
	if itemPath == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "file path is required"})
		return
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 500<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "failed to read request body"})
		return
	}
	filePath, err := repoStore.WriteFileAtRepoPath(repoOwner, repoName, itemPath, data, true)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	info, _ := os.Stat(filePath)
	payload := map[string]interface{}{"success": true, "path": itemPath}
	if info != nil {
		payload["size"] = info.Size()
		payload["modified"] = info.ModTime().Unix()
	}
	writeJSON(w, http.StatusOK, payload)
}

func handleFSAPIDelete(w http.ResponseWriter, repoOwner string, repoName string, itemPath string) {
	if itemPath == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "path is required"})
		return
	}
	if err := repoStore.DeleteItem(repoOwner, repoName, "/", itemPath); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

func handleFSAPIPost(w http.ResponseWriter, r *http.Request, repoOwner string, repoName string, itemPath string) {
	switch strings.ToLower(strings.TrimSpace(r.URL.Query().Get("op"))) {
	case "mkdir":
		if itemPath == "" {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "directory path is required"})
			return
		}
		if _, err := repoStore.CreateDirectoryAtRepoPath(repoOwner, repoName, itemPath); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "path": itemPath})
	case "rename":
		if itemPath == "" {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "source path is required"})
			return
		}
		var req fsRenameRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "invalid rename payload"})
			return
		}
		newPath := normalizeAPIRepoPath(req.NewPath)
		if newPath == "" {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "newPath is required"})
			return
		}
		if err := repoStore.RenameItem(repoOwner, repoName, "/", itemPath, newPath); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "path": newPath})
	default:
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "unsupported filesystem operation"})
	}
}

func fsAPIPermissions(r *http.Request, repoOwner string, repoName string) (string, bool, bool) {
	SS := config.GetSessionManager()
	sessionUser := SS.GetString(r.Context(), "name")
	isLoggedIn := SS.GetBool(r.Context(), "isLoggedIn")

	meta, _ := repoStore.LoadRepoMeta(repoOwner, repoName)
	repoPath := fmt.Sprintf("%s/%s/%s", repoStore.RepoDir, repoOwner, repoName)
	if ok, _ := repoStore.DirExists(repoPath); !ok {
		return sessionUser, false, false
	}

	canRead := meta != nil && meta.Public
	canWrite := false
	if isLoggedIn && sessionUser != "" {
		if sessionUser == repoOwner {
			canRead = true
			canWrite = true
		}
		if meta != nil {
			for _, owner := range meta.Owners {
				if owner == sessionUser {
					canRead = true
					canWrite = true
					break
				}
			}
		}
	}
	return sessionUser, canRead, canWrite
}

func requiresFSWrite(r *http.Request) bool {
	if r.Method == http.MethodPut || r.Method == http.MethodDelete {
		return true
	}
	return r.Method == http.MethodPost
}

func normalizeAPIRepoPath(raw string) string {
	clean := path.Clean("/" + strings.TrimSpace(raw))
	if clean == "." || clean == "/" {
		return ""
	}
	return strings.TrimPrefix(clean, "/")
}

func buildFSAPIPath(userName string, repoName string, itemPath string) string {
	base := "/api/fs/" + url.PathEscape(userName) + "/" + url.PathEscape(repoName)
	itemPath = normalizeAPIRepoPath(itemPath)
	if itemPath == "" {
		return base
	}
	segments := strings.Split(itemPath, "/")
	for index, segment := range segments {
		segments[index] = url.PathEscape(segment)
	}
	return base + "/" + strings.Join(segments, "/")
}

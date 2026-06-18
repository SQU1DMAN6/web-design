package repository

import (
	"encoding/json"
	"fmt"
	"inkdrop/config"
	userModel "inkdrop/model"
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
	// ?list=1 — return full recursive listing
	if r.URL.Query().Get("list") == "1" {
		entries, err := repoStore.ListDropEntriesRecursive(repoOwner, repoName)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]interface{}{"success": false, "error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "entries": entries})
		return
	}

	// ?stat=1 — return single file/directory metadata without content
	if r.URL.Query().Get("stat") == "1" {
		if itemPath == "" {
			// stat on the drop root itself
			root := repoStore.RepositoryRoot(repoOwner, repoName)
			info, err := os.Stat(root)
			if err != nil {
				writeJSON(w, http.StatusNotFound, map[string]interface{}{"success": false, "error": "drop not found"})
				return
			}
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"success":  true,
				"path":     "",
				"type":     "dir",
				"size":     info.Size(),
				"modified": info.ModTime().Unix(),
				"name":     repoName,
			})
			return
		}
		entry, err := repoStore.StatDropEntry(repoOwner, repoName, itemPath)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": err.Error()})
			return
		}
		if entry == nil {
			writeJSON(w, http.StatusNotFound, map[string]interface{}{"success": false, "error": "path not found"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "entry": entry})
		return
	}

	// No query params — stream file content (with optional Range support)
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

	fileSize := info.Size()

	// Handle Range header for partial content streaming
	rangeHeader := r.Header.Get("Range")
	if rangeHeader != "" && strings.HasPrefix(rangeHeader, "bytes=") {
		rangeStr := strings.TrimPrefix(rangeHeader, "bytes=")
		rangeParts := strings.SplitN(rangeStr, "-", 2)
		if len(rangeParts) == 2 {
			start, errStart := strconv.ParseInt(strings.TrimSpace(rangeParts[0]), 10, 64)
			end, errEnd := strconv.ParseInt(strings.TrimSpace(rangeParts[1]), 10, 64)

			if errStart == nil {
				if start < 0 {
					start = 0
				}
				if start >= fileSize {
					w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", fileSize))
					http.Error(w, "range not satisfiable", http.StatusRequestedRangeNotSatisfiable)
					return
				}
				if errEnd != nil || end >= fileSize {
					end = fileSize - 1
				}
				if end < start {
					end = start
				}
				length := end - start + 1

				file.Seek(start, io.SeekStart)
				w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, fileSize))
				w.Header().Set("Content-Length", strconv.FormatInt(length, 10))
				w.Header().Set("Content-Type", "application/octet-stream")
				w.WriteHeader(http.StatusPartialContent)
				io.CopyN(w, file, length)
				return
			}
		}
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
	filePath, err := repoStore.WriteFileAtDropPath(repoOwner, repoName, itemPath, data, true)
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
		if _, err := repoStore.CreateDirectoryAtDropPath(repoOwner, repoName, itemPath); err != nil {
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
	repoPath := fmt.Sprintf("%s/%s/%s", repoStore.DropDir, repoOwner, repoName)
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
		if !canRead && userModel.CanReadContactDrops(config.GetDB(), sessionUser, repoOwner) {
			canRead = true
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

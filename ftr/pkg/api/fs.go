package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
)

type RepoEntry struct {
	Path     string `json:"path"`
	Type     string `json:"type"`
	Size     int64  `json:"size"`
	Modified int64  `json:"modified"`
	Hash     string `json:"hash,omitempty"`
}

func (c *Client) FSListRepo(user, repo string) ([]RepoEntry, error) {
	req, err := c.newFSRequest(http.MethodGet, user, repo, "", "?list=1", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("filesystem list request failed: %w", err)
	}
	defer resp.Body.Close()

	var payload struct {
		Success bool        `json:"success"`
		Error   string      `json:"error"`
		Entries []RepoEntry `json:"entries"`
	}
	if err := decodeFSJSONResponse(resp, &payload); err != nil {
		return nil, err
	}
	if !payload.Success {
		if payload.Error == "" {
			payload.Error = "filesystem list failed"
		}
		return nil, fmt.Errorf("%s", payload.Error)
	}
	return payload.Entries, nil
}

func (c *Client) FSDownloadTo(user, repo, repoPath, destPath string) error {
	req, err := c.newFSRequest(http.MethodGet, user, repo, repoPath, "", nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("filesystem read request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return decodeFSError(resp)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer out.Close()
	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("failed to write downloaded file: %w", err)
	}
	return nil
}

func (c *Client) FSWriteFile(user, repo, repoPath string, reader io.Reader, size int64) error {
	req, err := c.newFSRequest(http.MethodPut, user, repo, repoPath, "", reader)
	if err != nil {
		return err
	}
	if size >= 0 {
		req.ContentLength = size
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("filesystem write request failed: %w", err)
	}
	defer resp.Body.Close()

	var payload struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}
	if err := decodeFSJSONResponse(resp, &payload); err != nil {
		return err
	}
	if !payload.Success {
		if payload.Error == "" {
			payload.Error = "filesystem write failed"
		}
		return fmt.Errorf("%s", payload.Error)
	}
	return nil
}

func (c *Client) FSMkdir(user, repo, repoPath string) error {
	req, err := c.newFSRequest(http.MethodPost, user, repo, repoPath, "?op=mkdir", nil)
	if err != nil {
		return err
	}
	return c.doFSJSONMutation(req, "filesystem mkdir failed")
}

func (c *Client) FSDeletePath(user, repo, repoPath string) error {
	req, err := c.newFSRequest(http.MethodDelete, user, repo, repoPath, "", nil)
	if err != nil {
		return err
	}
	return c.doFSJSONMutation(req, "filesystem delete failed")
}

func (c *Client) FSRenamePath(user, repo, oldPath, newPath string) error {
	body, err := json.Marshal(map[string]string{"newPath": normalizeRepoPathForAPI(newPath)})
	if err != nil {
		return err
	}
	req, err := c.newFSRequest(http.MethodPost, user, repo, oldPath, "?op=rename", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.doFSJSONMutation(req, "filesystem rename failed")
}

func (c *Client) doFSJSONMutation(req *http.Request, fallback string) error {
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s: %w", fallback, err)
	}
	defer resp.Body.Close()

	var payload struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}
	if err := decodeFSJSONResponse(resp, &payload); err != nil {
		return err
	}
	if !payload.Success {
		if payload.Error == "" {
			payload.Error = fallback
		}
		return fmt.Errorf("%s", payload.Error)
	}
	return nil
}

func (c *Client) newFSRequest(method, user, repo, repoPath, rawQuery string, body io.Reader) (*http.Request, error) {
	resource := "/api/fs/" + url.PathEscape(user) + "/" + url.PathEscape(repo)
	repoPath = normalizeRepoPathForAPI(repoPath)
	if repoPath != "" {
		parts := strings.Split(repoPath, "/")
		for index, part := range parts {
			parts[index] = url.PathEscape(part)
		}
		resource += "/" + strings.Join(parts, "/")
	}
	if rawQuery != "" {
		resource += rawQuery
	}

	req, err := http.NewRequest(method, inkdropURL(resource), body)
	if err != nil {
		return nil, fmt.Errorf("failed to create filesystem request: %w", err)
	}
	req.Header.Set("X-FTR-CLIENT", "FtR-CLI")
	if c.sessionID != "" {
		req.Header.Set("Cookie", "PHPSESSID="+c.sessionID)
	}
	return req, nil
}

func decodeFSJSONResponse(resp *http.Response, target interface{}) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read filesystem response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message := strings.TrimSpace(string(body))
		var payload struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(body, &payload); err == nil && payload.Error != "" {
			message = payload.Error
		}
		return fmt.Errorf("filesystem request failed: %s: %s", resp.Status, message)
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("failed to parse filesystem response: %w", err)
	}
	return nil
}

func decodeFSError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	message := strings.TrimSpace(string(body))
	var payload struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err == nil && payload.Error != "" {
		message = payload.Error
	}
	return fmt.Errorf("filesystem request failed: %s: %s", resp.Status, message)
}

func normalizeRepoPathForAPI(repoPath string) string {
	clean := path.Clean("/" + strings.TrimSpace(repoPath))
	if clean == "." || clean == "/" {
		return ""
	}
	return strings.TrimPrefix(clean, "/")
}

package api

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const (
	BaseURL = "https://quanthai.net/inkdrop"
	RepoURL = BaseURL + "/repos"
)

type Client struct {
	http      *http.Client
	sessionID string
	configDir string
}

func NewClient() (*Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create cookie jar: %w", err)
	}

	// Create config directory in /var/lib for shared access
	configDir := "/var/lib/ftr"
	if err := os.MkdirAll(configDir, 0777); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	// Ensure permissions allow both user and sudo access
	if err := os.Chmod(configDir, 0777); err != nil {
		fmt.Printf("Warning: Failed to set directory permissions: %v\n", err)
	}

	client := &Client{
		http: &http.Client{
			Jar: jar,
		},
		configDir: configDir,
	}

	// Try to load existing session
	if err := client.loadSession(); err == nil {
		return client, nil
	}

	return client, nil
}

func (c *Client) loadSession() error {
	sessionFile := filepath.Join(c.configDir, "session")
	data, err := os.ReadFile(sessionFile)
	if err != nil {
		return err
	}
	c.sessionID = string(data)
	return nil
}

func (c *Client) saveSession() error {
	sessionFile := filepath.Join(c.configDir, "session")
	if err := os.WriteFile(sessionFile, []byte(c.sessionID), 0666); err != nil {
		return err
	}
	// Ensure file is readable/writable by both user and sudo
	return os.Chmod(sessionFile, 0666)
}

func (c *Client) Login(email, password string) error {
	data := url.Values{}
	data.Set("email", email)
	data.Set("password", password)

	resp, err := c.http.PostForm(BaseURL+"/login.php", data)
	if err != nil {
		return fmt.Errorf("login request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body to check for errors
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Check if login failed by looking for error message in HTML
	if bytes.Contains(body, []byte("Error logging in")) {
		return fmt.Errorf("invalid credentials")
	}

	// Look for session cookie
	var foundSession bool
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "PHPSESSID" {
			c.sessionID = cookie.Value
			if err := c.saveSession(); err != nil {
				fmt.Println("Warning: Failed to save session")
			}
			foundSession = true
			break
		}
	}

	if !foundSession {
		return fmt.Errorf("login failed: no session cookie received")
	}

	return nil
}

func (c *Client) CreateRepo(user, repoName string) error {
	// The repository will be created automatically when we try to upload
	// Just verify we have the right permissions
	if user != os.Getenv("USER") {
		return fmt.Errorf("cannot create repository - not authorized")
	}
	return nil
}

func (c *Client) UploadFile(repoPath string, fileName string, reader io.Reader) error {
	// Get real username regardless of sudo
	realUser := os.Getenv("SUDO_USER")
	if realUser == "" {
		realUser = os.Getenv("USER")
	}

	if c.sessionID == "" {
		return fmt.Errorf("not logged in. Please run 'ftr login' first")
	}

	// Split user/repo
	parts := strings.Split(repoPath, "/")
	if len(parts) != 2 {
		return fmt.Errorf("invalid repository path. Must be in format user/repo")
	}
	user, repoName := parts[0], parts[1]

	// First try to access the repo
	resp, err := c.http.Get(fmt.Sprintf("%s/repo.php?name=%s&user=%s", BaseURL, url.QueryEscape(repoName), url.QueryEscape(user)))
	if err != nil {
		return fmt.Errorf("failed to check repository: %w", err)
	}

	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()

	// If repo doesn't exist and we're the owner, create it
	if bytes.Contains(body, []byte("repository is not found")) {
		// Get real username regardless of sudo
		realUser := os.Getenv("SUDO_USER")
		if realUser == "" {
			realUser = os.Getenv("USER")
		}

		if user != realUser { // Only create if we're the owner
			return fmt.Errorf("repository does not exist and you are not the owner")
		}
		if err := c.CreateRepo(user, repoName); err != nil {
			return fmt.Errorf("failed to create repository: %w", err)
		}
	}

	// Create multipart form
	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	// Add file
	fw, err := w.CreateFormFile("upload", fileName)
	if err != nil {
		return fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := io.Copy(fw, reader); err != nil {
		return fmt.Errorf("failed to copy file data: %w", err)
	}
	w.Close()

	// Create request to repo.php with appropriate query parameters
	uploadURL := fmt.Sprintf("%s/repo.php?name=%s&user=%s", BaseURL, url.QueryEscape(repoName), url.QueryEscape(user))
	req, err := http.NewRequest("POST", uploadURL, &b)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	// Send request
	resp, err = c.http.Do(req)
	if err != nil {
		return fmt.Errorf("upload request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response to check for success/failure message
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Debug output to see what we're getting back
	fmt.Printf("Server response: %s\n", string(body))

	// Check if we got redirected to login page
	if bytes.Contains(body, []byte("Login with an existing InkDrop account")) {
		c.sessionID = "" // Clear invalid session
		_ = os.Remove(filepath.Join(c.configDir, "session")) // Remove invalid session file
		return fmt.Errorf("session expired or invalid - please run 'ftr login' again")
	}

	// Look for success message in the response
	if bytes.Contains(body, []byte("color: #0f0")) && bytes.Contains(body, []byte("Uploaded")) {
		return nil // Success case
	}

	// Error cases
	if bytes.Contains(body, []byte("Failed to create repository")) {
		return fmt.Errorf("failed to create repository - permission denied")
	}

	if bytes.Contains(body, []byte("Upload failed")) || bytes.Contains(body, []byte("color: red")) {
		return fmt.Errorf("upload failed - server rejected the file")
	}

	if bytes.Contains(body, []byte("cannot upload")) || !bytes.Contains(body, []byte("uploadForm")) {
		return fmt.Errorf("upload failed - not authorized to upload to this repository")
	}

	return fmt.Errorf("upload failed - unexpected server response. Try 'ftr login' if you haven't logged in recently")
}

func (c *Client) DownloadFile(repoPath string, fileName string) (io.ReadCloser, error) {
	// Download from /inkdrop/repos/USER/REPO/filename
	downloadURL := fmt.Sprintf("%s/repos/%s/%s", BaseURL, repoPath, fileName)

	resp, err := c.http.Get(downloadURL)
	if err != nil {
		return nil, fmt.Errorf("download request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("download failed with status: %s", resp.Status)
	}

	return resp.Body, nil
}

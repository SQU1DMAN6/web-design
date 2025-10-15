package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
)

const (
	BaseURL = "https://quanthai.net/inkdrop/repos"
)

type Client struct {
	http      *http.Client
	sessionID string
	configDir string
}

type loginResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func NewClient() (*Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create cookie jar: %w", err)
	}

	// Create config directory in user's home
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}
	configDir := filepath.Join(home, ".config", "ftr")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
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
	return os.WriteFile(sessionFile, []byte(c.sessionID), 0600)
}

func (c *Client) Login(username, password string) error {
	data := url.Values{}
	data.Set("username", username)
	data.Set("password", password)

	resp, err := c.http.PostForm(BaseURL+"/login.php", data)
	if err != nil {
		return fmt.Errorf("login request failed: %w", err)
	}
	defer resp.Body.Close()

	var loginResp loginResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		return fmt.Errorf("failed to parse login response: %w", err)
	}

	if !loginResp.Success {
		return fmt.Errorf("login failed: %s", loginResp.Message)
	}

	// Save session cookies
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "PHPSESSID" {
			c.sessionID = cookie.Value
			if err := c.saveSession(); err != nil {
				fmt.Println("Warning: Failed to save session")
			}
			break
		}
	}

	return nil
}

func (c *Client) UploadFile(repoPath string, fileName string, reader io.Reader) error {
	if c.sessionID == "" {
		return fmt.Errorf("not logged in")
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

	// Create request
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/%s", BaseURL, repoPath), &b)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	// Send request
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("upload request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("upload failed with status: %s", resp.Status)
	}

	return nil
}

func (c *Client) DownloadFile(repoPath string, fileName string) (io.ReadCloser, error) {
	url := fmt.Sprintf("%s/%s/%s", BaseURL, repoPath, fileName)

	resp, err := c.http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("download request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("download failed with status: %s", resp.Status)
	}

	return resp.Body, nil
}

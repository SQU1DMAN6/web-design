package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type RepoConfig struct {
	TargetDirectory string `json:"target_directory,omitempty"`
	PollInterval    string `json:"poll_interval,omitempty"`
	Encrypt         bool   `json:"encrypt,omitempty"`
	Auto            string `json:"auto,omitempty"`
	Ask             bool   `json:"ask,omitempty"`
}

type Config struct {
	Default RepoConfig            `json:"default,omitempty"`
	Repos   map[string]RepoConfig `json:"repos,omitempty"`
}

func getConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = filepath.Join(home, ".config")
	}
	return filepath.Join(configHome, "ftr"), nil
}

func GetConfigPath() (string, error) {
	configDir, err := getConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "daemon.json"), nil
}

func LoadConfig() (Config, error) {
	path, err := GetConfigPath()
	if err != nil {
		return Config{}, err
	}
	cfg := Config{Repos: make(map[string]RepoConfig)}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return Config{}, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	if cfg.Repos == nil {
		cfg.Repos = make(map[string]RepoConfig)
	}
	return cfg, nil
}

func SaveConfig(cfg Config) error {
	path, err := GetConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")
	return encoder.Encode(cfg)
}

func MergeRepoConfig(cfg Config, repoPath string) RepoConfig {
	merged := cfg.Default
	if repoCfg, ok := cfg.Repos[repoPath]; ok {
		if repoCfg.TargetDirectory != "" {
			merged.TargetDirectory = repoCfg.TargetDirectory
		}
		if repoCfg.PollInterval != "" {
			merged.PollInterval = repoCfg.PollInterval
		}
		merged.Encrypt = repoCfg.Encrypt
		if repoCfg.Auto != "" {
			merged.Auto = repoCfg.Auto
		}
		merged.Ask = repoCfg.Ask
	}
	return merged
}

func SetConfigValue(cfg *RepoConfig, key, value string) error {
	switch strings.ToLower(key) {
	case "target_directory", "targetdirectory", "target":
		cfg.TargetDirectory = value
	case "poll_interval", "pollinterval", "poll-interval":
		cfg.PollInterval = value
	case "encrypt":
		if value == "true" || value == "1" {
			cfg.Encrypt = true
		} else if value == "false" || value == "0" {
			cfg.Encrypt = false
		} else {
			return fmt.Errorf("invalid value for encrypt: %s", value)
		}
	case "auto":
		cfg.Auto = value
	case "ask":
		if value == "true" || value == "1" {
			cfg.Ask = true
		} else if value == "false" || value == "0" {
			cfg.Ask = false
		} else {
			return fmt.Errorf("invalid value for ask: %s", value)
		}
	default:
		return fmt.Errorf("unknown config key: %s", key)
	}
	return nil
}

func PrintConfig(cfg Config) error {
	path, err := GetConfigPath()
	if err != nil {
		return err
	}
	fmt.Printf("Daemon configuration file: %s\n", path)
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(cfg)
}

func systemdServiceName(repoPath string) string {
	safeName := strings.ReplaceAll(repoPath, "/", "-")
	return fmt.Sprintf("ftr-daemon@%s.service", safeName)
}

func systemdUserDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "systemd", "user"), nil
}

func ServiceUnitPath(repoPath string) (string, error) {
	dir, err := systemdUserDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, systemdServiceName(repoPath)), nil
}

func writeSystemdUnitFile(repoPath, execPath string) (string, error) {
	servicePath, err := ServiceUnitPath(repoPath)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(servicePath), 0755); err != nil {
		return "", err
	}

	contents := fmt.Sprintf(`[Unit]
Description=FtR live sync daemon for %s
After=network-online.target

[Service]
Type=simple
ExecStart=%s daemon run %s
Restart=on-failure
RestartSec=10
WorkingDirectory=%%h
Environment=HOME=%%h
Environment=XDG_CONFIG_HOME=%%h/.config
StandardOutput=append:%%h/.config/ftr/daemon.log
StandardError=append:%%h/.config/ftr/daemon.log

[Install]
WantedBy=default.target
`, repoPath, execPath, repoPath)

	return servicePath, os.WriteFile(servicePath, []byte(contents), 0644)
}

func runSystemctl(args ...string) ([]byte, error) {
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("systemd commands are supported only on Linux")
	}
	cmdPath, err := exec.LookPath("systemctl")
	if err != nil {
		return nil, fmt.Errorf("systemctl not found: %w", err)
	}
	cmd := exec.Command(cmdPath, append([]string{"--user"}, args...)...)
	return cmd.CombinedOutput()
}

func InstallService(repoPath, execPath string) error {
	if _, err := writeSystemdUnitFile(repoPath, execPath); err != nil {
		return err
	}
	if _, err := runSystemctl("daemon-reload"); err != nil {
		return fmt.Errorf("failed to reload systemd user units: %w", err)
	}
	if _, err := runSystemctl("enable", systemdServiceName(repoPath)); err != nil {
		return fmt.Errorf("failed to enable service: %w", err)
	}
	if _, err := runSystemctl("start", systemdServiceName(repoPath)); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}
	return nil
}

func UninstallService(repoPath string) error {
	serviceName := systemdServiceName(repoPath)
	if _, err := runSystemctl("stop", serviceName); err != nil {
		// continue even if stop fails
	}
	if _, err := runSystemctl("disable", serviceName); err != nil {
		// continue even if disable fails
	}
	path, err := ServiceUnitPath(repoPath)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	if _, err := runSystemctl("daemon-reload"); err != nil {
		return fmt.Errorf("failed to reload systemd user units: %w", err)
	}
	return nil
}

func StartService(repoPath string) error {
	if _, err := runSystemctl("start", systemdServiceName(repoPath)); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}
	return nil
}

func StopService(repoPath string) error {
	if _, err := runSystemctl("stop", systemdServiceName(repoPath)); err != nil {
		return fmt.Errorf("failed to stop service: %w", err)
	}
	return nil
}

func ServiceStatus(repoPath string) (string, error) {
	output, err := runSystemctl("status", systemdServiceName(repoPath))
	message := strings.ToLower(strings.TrimSpace(string(output)))
	if err != nil {
		if strings.Contains(message, "could not be found") || strings.Contains(message, "not be found") {
			return "", fmt.Errorf("systemd user service for %s is not installed. Run 'ftr daemon install %s' first", repoPath, repoPath)
		}
		if strings.Contains(message, "loaded:") && strings.Contains(message, "active:") {
			return string(output), nil
		}
	}
	return string(output), err
}

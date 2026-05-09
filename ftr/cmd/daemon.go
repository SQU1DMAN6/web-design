package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"ftr/pkg/api"
	"ftr/pkg/daemon"

	"github.com/spf13/cobra"
)

var daemonTargetDirectory string

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Manage the FtR sync daemon",
	Long:  `Manage a live sync daemon for FtR repositories, including systemd service support and daemon configuration.`,
}

var daemonRunCmd = &cobra.Command{
	Use:          "run <user>/<repo>",
	Short:        "Run live sync in the foreground",
	SilenceUsage: true,
	Args:         cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repoPath := args[0]
		parts := strings.Split(repoPath, "/")
		if len(parts) != 2 {
			return fmt.Errorf("repository path must be in format user/repo")
		}
		user, repo := parts[0], parts[1]

		client, err := api.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create API client: %w", err)
		}

		cfg, err := daemon.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to read daemon configuration: %w", err)
		}

		repoCfg := daemon.MergeRepoConfig(cfg, repoPath)

		targetDir := daemonTargetDirectory
		if targetDir == "" {
			targetDir = repoCfg.TargetDirectory
		}
		if targetDir == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to determine home directory: %w", err)
			}
			targetDir = filepath.Join(home, "FtR", user, repo)
		}
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return fmt.Errorf("failed to create sync directory: %w", err)
		}

		pollInterval, err := getPollInterval(cmd, repoCfg)
		if err != nil {
			return err
		}

		encrypt, _ := cmd.Flags().GetBool("encrypt")
		autoChoice, _ := cmd.Flags().GetString("auto")
		askConflicts, _ := cmd.Flags().GetBool("ask")

		if !cmd.Flags().Changed("encrypt") {
			encrypt = repoCfg.Encrypt
		}
		if !cmd.Flags().Changed("auto") && autoChoice == "ask" {
			autoChoice = repoCfg.Auto
		}
		if !cmd.Flags().Changed("ask") {
			askConflicts = repoCfg.Ask
		}

		fmt.Printf("Running live daemon for %s/%s in %s\n", user, repo, targetDir)
		return runLiveSync(context.Background(), client, user, repo, targetDir, encrypt, 10, autoChoice, askConflicts, pollInterval)
	},
}

var daemonInstallCmd = &cobra.Command{
	Use:          "install <user>/<repo>",
	Short:        "Install a systemd user service for the daemon",
	SilenceUsage: true,
	Args:         cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repoPath := args[0]
		if !strings.Contains(repoPath, "/") {
			return fmt.Errorf("repository path must be in format user/repo")
		}

		execPath, err := resolveInstallableExecutable()
		if err != nil {
			return err
		}

		if err := daemon.InstallService(repoPath, execPath); err != nil {
			return err
		}

		if err := daemon.StartService(repoPath); err != nil {
			fmt.Printf("Installed and enabled systemd user service for %s, but failed to start it: %v\n", repoPath, err)
			return nil
		}

		fmt.Printf("Installed, enabled, and started systemd user service for %s\n", repoPath)
		return nil
	},
}

var daemonUninstallCmd = &cobra.Command{
	Use:          "uninstall <user>/<repo>",
	Short:        "Uninstall the daemon systemd user service",
	SilenceUsage: true,
	Args:         cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repoPath := args[0]
		if !strings.Contains(repoPath, "/") {
			return fmt.Errorf("repository path must be in format user/repo")
		}

		if err := daemon.UninstallService(repoPath); err != nil {
			return err
		}

		fmt.Printf("Uninstalled systemd user service for %s\n", repoPath)
		return nil
	},
}

var daemonStartCmd = &cobra.Command{
	Use:          "start <user>/<repo>",
	Short:        "Start the configured daemon service",
	SilenceUsage: true,
	Args:         cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repoPath := args[0]
		if !strings.Contains(repoPath, "/") {
			return fmt.Errorf("repository path must be in format user/repo")
		}

		if err := daemon.StartService(repoPath); err != nil {
			return err
		}

		fmt.Printf("Started daemon service for %s\n", repoPath)
		return nil
	},
}

var daemonStopCmd = &cobra.Command{
	Use:          "stop <user>/<repo>",
	Short:        "Stop the configured daemon service",
	SilenceUsage: true,
	Args:         cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repoPath := args[0]
		if !strings.Contains(repoPath, "/") {
			return fmt.Errorf("repository path must be in format user/repo")
		}

		if err := daemon.StopService(repoPath); err != nil {
			return err
		}

		fmt.Printf("Stopped daemon service for %s\n", repoPath)
		return nil
	},
}

var daemonStatusCmd = &cobra.Command{
	Use:          "status <user>/<repo>",
	Short:        "Show current daemon service status",
	SilenceUsage: true,
	Args:         cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repoPath := args[0]
		if !strings.Contains(repoPath, "/") {
			return fmt.Errorf("repository path must be in format user/repo")
		}

		status, err := daemon.ServiceStatus(repoPath)
		if status != "" {
			fmt.Print(status)
		}
		if err != nil {
			return fmt.Errorf("failed to query daemon service status: %w", err)
		}

		return nil
	},
}

var daemonConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage daemon configuration",
}

var daemonConfigViewCmd = &cobra.Command{
	Use:   "view",
	Short: "Show daemon configuration",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := daemon.LoadConfig()
		if err != nil {
			return err
		}
		return daemon.PrintConfig(cfg)
	},
}

var daemonConfigSetCmd = &cobra.Command{
	Use:   "set <default|user/repo> <key> <value>",
	Short: "Set daemon configuration values",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		target := args[0]
		key := args[1]
		value := args[2]

		if target != "default" && !strings.Contains(target, "/") {
			return fmt.Errorf("target must be 'default' or a specific repository in user/repo format")
		}

		cfg, err := daemon.LoadConfig()
		if err != nil {
			return err
		}

		if target == "default" {
			if err := daemon.SetConfigValue(&cfg.Default, key, value); err != nil {
				return err
			}
		} else {
			repoCfg := cfg.Repos[target]
			if err := daemon.SetConfigValue(&repoCfg, key, value); err != nil {
				return err
			}
			if cfg.Repos == nil {
				cfg.Repos = make(map[string]daemon.RepoConfig)
			}
			cfg.Repos[target] = repoCfg
		}

		if err := daemon.SaveConfig(cfg); err != nil {
			return err
		}
		fmt.Println("Daemon configuration saved.")
		return nil
	},
}

func resolveInstallableExecutable() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to resolve executable path: %w", err)
	}

	tmpDir := os.TempDir()
	if strings.Contains(execPath, filepath.Join(tmpDir, "go-build")) || strings.Contains(execPath, "go-build") {
		binaryName := filepath.Base(execPath)
		pathBinary, err := exec.LookPath(binaryName)
		if err == nil {
			return pathBinary, nil
		}
		return "", fmt.Errorf("cannot install daemon from a temporary Go run binary (%s). Build a permanent executable or install %s to your PATH", execPath, binaryName)
	}

	return execPath, nil
}

func getPollInterval(cmd *cobra.Command, repoCfg daemon.RepoConfig) (time.Duration, error) {
	if cmd.Flags().Changed("poll-interval") {
		return cmd.Flags().GetDuration("poll-interval")
	}
	if repoCfg.PollInterval != "" {
		d, err := time.ParseDuration(repoCfg.PollInterval)
		if err != nil {
			return 0, fmt.Errorf("invalid poll interval in config: %w", err)
		}
		return d, nil
	}
	return 30 * time.Second, nil
}

func init() {
	daemonCmd.AddCommand(daemonRunCmd)
	daemonCmd.AddCommand(daemonInstallCmd)
	daemonCmd.AddCommand(daemonUninstallCmd)
	daemonCmd.AddCommand(daemonStartCmd)
	daemonCmd.AddCommand(daemonStopCmd)
	daemonCmd.AddCommand(daemonStatusCmd)
	daemonConfigCmd.AddCommand(daemonConfigViewCmd)
	daemonConfigCmd.AddCommand(daemonConfigSetCmd)
	daemonCmd.AddCommand(daemonConfigCmd)

	daemonRunCmd.Flags().StringVarP(&daemonTargetDirectory, "target", "T", "", "Target directory to sync repository with")
	daemonRunCmd.Flags().Bool("encrypt", false, "Encrypt uploaded files")
	daemonRunCmd.Flags().String("auto", "ask", "Auto-resolve conflicts: ask|local|remote|skip|both")
	daemonRunCmd.Flags().Bool("ask", false, "Ask interactively about conflicts")
	daemonRunCmd.Flags().Duration("poll-interval", 30*time.Second, "Remote polling interval for live sync")
}

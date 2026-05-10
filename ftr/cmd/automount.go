package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"bazil.org/fuse"
	"github.com/coreos/go-systemd/v22/unit"
	"github.com/spf13/cobra"
)

var automountCmd = &cobra.Command{
	Use:   "automount <user>/<repo> [mountpoint]",
	Short: "Manage systemd services for mounting repositories",
	Long: `Manage systemd services for automatically mounting repositories on the remote InkDrop machine to the local client.
Files are presented as placeholders and contents are downloaded only when opened.
Writes are uploaded back to the remote repository when file handles are closed.
`,
}

var automountinstallCmd = &cobra.Command{
	Use:   "install <user>/<repo> [mountpoint]",
	Short: "Install an FtR mount service to /etc/systemd/system",
	Long:  `Create and install an FtR systemd service to /etc/systemd/system to mount a specific remote repository on the InkDrop machine to the local client.`,
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		repoPath := args[0]
		parts := strings.Split(repoPath, "/")
		if len(parts) != 2 {
			return fmt.Errorf("repository must be in the format user/repo")
		}
		repoOwner, repo := parts[0], parts[1]

		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to determine home directory: %w", err)
		}

		var mountPoint string
		if len(args) == 2 {
			var err error
			mountPoint, err = expandPath(args[1])
			if err != nil {
				return fmt.Errorf("Failed to expand mount point path: %w", err)
			}
		} else {
			mountPoint = filepath.Join(home, "FtR", repoOwner, repo)
		}
		if err := os.MkdirAll(mountPoint, 0755); err != nil {
			return fmt.Errorf("failed to create mount point %s: %w", mountPoint, err)
		}
		if info, err := os.Stat(mountPoint); err != nil {
			if isTransportEndpointNotConnected(err) {
				fmt.Fprintf(os.Stderr, "warning: stale mount point detected, attempting cleanup: %s\n", mountPoint)
				if uerr := fuse.Unmount(mountPoint); uerr != nil {
					return fmt.Errorf("mount point %s is stale and cleanup failed: %w (unmount failed: %v)", mountPoint, err, uerr)
				}
				info, err = os.Stat(mountPoint)
				if err != nil {
					return fmt.Errorf("mount point %s is not accessible after cleanup: %w", mountPoint, err)
				}
			} else {
				return fmt.Errorf("mount point %s is not accessible: %w", mountPoint, err)
			}
		} else if !info.IsDir() {
			return fmt.Errorf("mount point %s is a directory", mountPoint)
		}

		currentUser, err := user.Current()
		if err != nil {
			return fmt.Errorf("failed to determine current user: %s", err)
		}

		var currentUserName string = currentUser.Username
		fmt.Printf("Will install systemd service as user %s\n", currentUserName)

		execStart := fmt.Sprintf(
			"/usr/local/bin/ftr mount %s/%s %s",
			repoOwner,
			repo,
			mountPoint,
		)
		// Define the systemd unit file sections
		opts := []*unit.UnitOption{
			// [Unit] section of service file
			unit.NewUnitOption("Unit", "Description", fmt.Sprintf("Mount %s/%s to %s", repoOwner, repo, mountPoint)),

			// [Service] section of service file
			unit.NewUnitOption("Service", "Type", "simple"),
			unit.NewUnitOption("Service", "ExecStart", execStart),
			unit.NewUnitOption("Service", "Restart", "on-failure"),
			unit.NewUnitOption("Service", "User", currentUserName),
			unit.NewUnitOption("Service", "RestartSec", "5s"),

			// [Install] section of service file
			unit.NewUnitOption("Install", "WantedBy", "default.target"),
		}

		readerRaw := unit.Serialize(opts)
		if err != nil {
			return fmt.Errorf("failed to get data from unit file")
		}

		serviceFilePath := filepath.Join(home, ".config", "systemd", "user", fmt.Sprintf("ftrmount-%s-%s.service", repoOwner, repo))
		filePathDir := filepath.Dir(serviceFilePath)
		err = os.MkdirAll(filePathDir, 0755)
		if err != nil {
			return fmt.Errorf("failed to make parent directory for file: %s", err)
		}

		file, err := os.Create(serviceFilePath)
		if err != nil {
			return fmt.Errorf("failed to create service file: %w", err)
		}

		defer file.Close()

		if _, err := file.ReadFrom(readerRaw); err != nil {
			return fmt.Errorf("failed to write service file: %w", err)
		}

		return nil
	},
}

var automountenableCmd = &cobra.Command{
	Use:   "enable <user>/<repo>",
	Short: "Enable a systemd service to mount a repository",
	Long: `Enable a systemd service to mount a repository on boot, and mount the the repository immediately.
	The repository's mount point is the mount point specified by the systemd service file installed under ~/.config/systemd/user`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repoPath := args[0]
		parts := strings.Split(repoPath, "/")
		if len(parts) != 2 {
			return fmt.Errorf("repository path must be in the format user/repo")
		}

		user, repo := parts[0], parts[1]

		serviceName := fmt.Sprintf("ftrmount-%s-%s.service", user, repo)

		command := exec.Command("systemctl", "--user", "enable", "--now", serviceName)
		output, err := command.CombinedOutput()
		if err != nil {
			return fmt.Errorf("error executing command: %v\noutput: %s", err, string(output))
		}

		fmt.Printf("Repository %s/%s was mounted and its systemd service was enabled.\n", user, repo)
		return nil
	},
}

var automountdisableCmd = &cobra.Command{
	Use:   "disable <user>/<repo>",
	Short: "Unmount a repository and disable its respective systemd service",
	Long:  "Immediately unmount a repository and disable its respective systemd service so it does not mount the repository again on boot.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repoPath := args[0]
		parts := strings.Split(repoPath, "/")
		if len(parts) != 2 {
			return fmt.Errorf("repository path must be in the form user/repo")
		}
		user, repo := parts[0], parts[1]
		serviceName := fmt.Sprintf("ftrmount-%s-%s.service", user, repo)

		command := exec.Command("systemctl", "--user", "disable", serviceName)
		output, err := command.CombinedOutput()
		if err != nil {
			return fmt.Errorf("error executing command: %v\noutput: %s", err, string(output))
		}

		command2 := exec.Command("systemctl", "--user", "stop", serviceName)
		output2, err2 := command2.CombinedOutput()
		if err2 != nil {
			return fmt.Errorf("error executing command: %v\noutput: %s", err, string(output2))
		}

		fmt.Printf("Repository %s/%s was unmounted and its systemd service was disabled.\n", user, repo)
		return nil
	},
}

var automountuninstallCmd = &cobra.Command{
	Use:   "uninstall <user>/<repo>",
	Short: "Uninstall the systemd service file of a remote repository",
	Long:  "Uninstall the systemd service file of a remote repository",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repoPath := args[0]
		parts := strings.Split(repoPath, "/")
		if len(parts) != 2 {
			return fmt.Errorf("repository path must be in the format user/repo")
		}

		user, repo := parts[0], parts[1]

		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to determine home directory: %w", err)
		}

		serviceFilePath := filepath.Join(home, ".config", "systemd", "user", fmt.Sprintf("ftrmount-%s-%s.service", user, repo))

		err = os.Remove(serviceFilePath)
		if err != nil {
			return fmt.Errorf("failed to remove service file: %w", err)
		}

		fmt.Println("Service uninstalled successfully.")
		return nil
	},
}

func expandPath(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}

		if path == "~" {
			path = home
		} else {
			path = filepath.Join(home, path[2:])
		}
	} else if strings.HasPrefix(path, "~") {
		return "", fmt.Errorf("unsupported tilde expansion")
	}

	return filepath.Abs(path)
}

func init() {
	automountCmd.AddCommand(automountinstallCmd)
	automountCmd.AddCommand(automountenableCmd)
	automountCmd.AddCommand(automountdisableCmd)
	automountCmd.AddCommand(automountuninstallCmd)
}

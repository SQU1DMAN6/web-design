package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var removeCmd = &cobra.Command{
	Use:   "remove [repo]",
	Short: "Remove an installed package",
	Long: `Remove an installed package from the system.
This will remove the binary from /usr/local/bin and its directory from /usr/share.

Example: ftr remove myapp
         ftr remove user/myapp`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repoPath := args[0]
		
		// Extract repo name if full path is given
		repoName := repoPath
		if strings.Contains(repoPath, "/") {
			parts := strings.Split(repoPath, "/")
			repoName = parts[len(parts)-1]
		}

		binPath := filepath.Join("/usr/local/bin", repoName)
		sharePath := filepath.Join("/usr/share", repoName)

		// Remove binary
		if _, err := os.Stat(binPath); err == nil {
			if err := exec.Command("sudo", "rm", "-rf", binPath).Run(); err != nil {
				return fmt.Errorf("failed to remove binary: %w", err)
			}
			fmt.Printf("Removed binary from %s\n", binPath)
		} else {
			fmt.Println("Binary not found in /usr/local/bin")
		}

		// Remove share directory
		if _, err := os.Stat(sharePath); err == nil {
			if err := exec.Command("sudo", "rm", "-rf", sharePath).Run(); err != nil {
				return fmt.Errorf("failed to remove share directory: %w", err)
			}
			fmt.Printf("Removed directory %s\n", sharePath)
		} else {
			fmt.Println("Share directory not found")
		}

		return nil
	},
}
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func init() {
	// No need to register
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Log out of your FtR session",
	Long:  "Removes all saved session data, effectively logging you out of FtR.",
	RunE: func(cmd *cobra.Command, args []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}

		configDir := filepath.Join(home, ".config", "ftr")
		sessionFile := filepath.Join(configDir, "session")

		// Delete the session file if it exists
		if _, err := os.Stat(sessionFile); err == nil {
			if err := os.Remove(sessionFile); err != nil {
				return fmt.Errorf("failed to remove session file: %w", err)
			}
			fmt.Println("Logged out successfully.")
		} else {
			fmt.Println("No active session found.")
		}

		// Optionally, remove the config directory if empty
		_ = os.Remove(configDir)

		return nil
	},
}

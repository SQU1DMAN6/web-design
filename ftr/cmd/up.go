package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"ftr/pkg/api"
	"os"
	"path/filepath"
)

func init() {
	// No need to register here as it's done in commands.go
}

var upCmd = &cobra.Command{
	Use:   "up [file] [user/repo]",
	Short: "Upload a file to a repository",
	Long: `Upload a file to a repository on the InkDrop server.

Example: ftr up myfile.txt user/repo`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		sourcePath := args[0]
		repoPath := args[1]

		// Check if source exists
		info, err := os.Stat(sourcePath)
		if err != nil {
			return fmt.Errorf("failed to access source path: %w", err)
		}
		if info.IsDir() {
			return fmt.Errorf("source must be a file, not a directory")
		}

		// Upload to server
		client, err := api.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create API client: %w", err)
		}

		fmt.Printf("Uploading %s to %s...\n", sourcePath, repoPath)
		f, err := os.Open(sourcePath)
		if err != nil {
			return fmt.Errorf("failed to open file: %w", err)
		}
		defer f.Close()

		if err := client.UploadFile(repoPath, filepath.Base(sourcePath), f); err != nil {
			return fmt.Errorf("upload failed: %w", err)
		}

		fmt.Printf("File %s uploaded successfully\n", filepath.Base(sourcePath))
		return nil
	},
}
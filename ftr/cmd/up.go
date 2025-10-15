package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"ftr/pkg/api"
	"ftr/pkg/fsdl"
	"os"
	"path/filepath"
)

// init registers this command to the root command
func init() {
	rootCmd.AddCommand(upCmd)
}

var upCmd = &cobra.Command{
	Use:   "up [source] [user/repo]",
	Short: "Upload files to a repository",
	Long: `Upload files to a repository by creating an FSDL package.
The source directory will be packed into an FSDL file and uploaded.

Example: ftr up ./myapp user/myapp`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		sourcePath := args[0]
		repoPath := args[1]

		// Check if source exists
		info, err := os.Stat(sourcePath)
		if err != nil {
			return fmt.Errorf("failed to access source path: %w", err)
		}
		if !info.IsDir() {
			return fmt.Errorf("source must be a directory")
		}

		// Create temporary directory
		tmpDir := "/tmp/fsdl"
		if err := os.MkdirAll(tmpDir, 0755); err != nil {
			return fmt.Errorf("failed to create temp directory: %w", err)
		}

		// Create and pack FSDL
		repoName := filepath.Base(sourcePath)
		pkg, err := fsdl.Create(repoName, sourcePath)
		if err != nil {
			return fmt.Errorf("failed to create package: %w", err)
		}

		fsdlPath := filepath.Join(tmpDir, repoName+".fsdl")
		fmt.Println("Creating FSDL package...")
		err = pkg.Pack(sourcePath, fsdlPath)
		if err != nil {
			return fmt.Errorf("failed to pack files: %w", err)
		}

		// Upload to server
		client, err := api.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create API client: %w", err)
		}

		fmt.Printf("Uploading package to %s...\n", repoPath)
		f, err := os.Open(fsdlPath)
		if err != nil {
			return fmt.Errorf("failed to open package file: %w", err)
		}
		defer f.Close()

		if err := client.UploadFile(repoPath, repoName+".fsdl", f); err != nil {
			return fmt.Errorf("upload failed: %w", err)
		}

		fmt.Println("Package uploaded successfully")
		return nil
	},
}
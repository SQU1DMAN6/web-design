package cmd

import (
	"fmt"
	"ftr/pkg/api"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var remoteCmd = &cobra.Command{
	Use:   "remote",
	Short: "Manage remote Drops",
	Long:  `Perform actions on remote Drops by interacting with the InkDrop server.`,
}

var remoteRemoveCmd = &cobra.Command{
	Use:   "delete <user>/<repo>/<file> [<user>/<repo>/<file>...]",
	Short: "Remove one or more files from a remote Drop",
	Long: `Permanently removes one or more files from a remote Drop on the InkDrop server. This action requires you to be logged in and to be the owner of the Drop.

Examples:
  ftr remote delete user/repo/file.txt
  ftr remote delete user/repo/file1.txt user/repo/file2.txt`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Deduplicate file paths while preserving order
		seen := make(map[string]struct{})
		filePaths := []string{}
		for _, path := range args {
			if _, ok := seen[path]; ok {
				continue
			}
			seen[path] = struct{}{}
			filePaths = append(filePaths, path)
		}

		client, err := api.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create API client: %w", err)
		}

		if !client.IsLoggedIn() {
			return fmt.Errorf("you must be logged in to remove files. Please run 'ftr login'")
		}

		// Parallel deletion with a small concurrency limit
		sem := make(chan struct{}, 6)
		errCh := make(chan error, len(filePaths))

		for _, filePath := range filePaths {
			filePath := filePath
			parts := strings.SplitN(filePath, "/", 3)
			if len(parts) != 3 || parts[2] == "" {
				fmt.Fprintf(os.Stderr, "invalid path format: %s (must be <user>/<repo>/<file>)\n", filePath)
				errCh <- fmt.Errorf("invalid path format")
				continue
			}
			user, repo, fileName := parts[0], parts[1], parts[2]

			sem <- struct{}{}
			go func() {
				defer func() { <-sem }()

				fmt.Printf("Removing '%s' from %s/%s...\n", fileName, user, repo)

				if err := client.FSDeletePath(user, repo, fileName); err != nil {
					fmt.Fprintf(os.Stderr, "failed to remove %s: %v\n", filePath, err)
					errCh <- err
					return
				}

				fmt.Printf("Successfully removed '%s' from %s/%s.\n", fileName, user, repo)
				errCh <- nil
			}()
		}

		// wait for all goroutines to finish
		for i := 0; i < cap(sem); i++ {
			sem <- struct{}{}
		}
		close(errCh)

		var lastErr error
		for e := range errCh {
			if e != nil {
				lastErr = e
			}
		}

		if lastErr != nil {
			return fmt.Errorf("one or more deletions failed")
		}
		return nil
	},
}

var remoteRenameCmd = &cobra.Command{
	Use:   "rename <user>/<repo>/<old-path> <new-path>",
	Short: "Rename or move a remote file or directory",
	Long: `Rename or move a file or directory inside a remote repository.

The destination may be either a repository-relative path or a full
<user>/<repo>/<path> path in the same repository.

Examples:
  ftr remote rename user/repo/file.txt renamed.txt
  ftr remote rename user/repo/docs/old.md docs/new.md`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		source, err := parseRemotePath(args[0])
		if err != nil {
			return err
		}
		destPath, err := parseRemoteDestination(source, args[1])
		if err != nil {
			return err
		}
		client, err := api.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create API client: %w", err)
		}
		if !client.IsLoggedIn() {
			return fmt.Errorf("you must be logged in to rename remote items. Please run 'ftr login'")
		}
		if err := client.FSRenamePath(source.user, source.repo, source.path, destPath); err != nil {
			return fmt.Errorf("rename failed: %w", err)
		}
		fmt.Printf("Renamed %s/%s/%s -> %s\n", source.user, source.repo, source.path, destPath)
		return nil
	},
}

var remoteMkdirCmd = &cobra.Command{
	Use:   "mkdir <user>/<repo>/<dir> [<user>/<repo>/<dir>...]",
	Short: "Create remote directories",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := api.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create API client: %w", err)
		}
		if !client.IsLoggedIn() {
			return fmt.Errorf("you must be logged in to create remote directories. Please run 'ftr login'")
		}
		for _, raw := range args {
			spec, err := parseRemotePath(raw)
			if err != nil {
				return err
			}
			if err := client.FSMkdir(spec.user, spec.repo, spec.path); err != nil {
				return fmt.Errorf("mkdir %s failed: %w", raw, err)
			}
			fmt.Printf("Created remote directory %s/%s/%s\n", spec.user, spec.repo, spec.path)
		}
		return nil
	},
}

var remoteDownCmd = &cobra.Command{
	Use:   "down <user>/<repo>/<file-path> [<user>/<repo>/<file-path>...]",
	Short: "Download one or more files from a remote Drop",
	Long: `Downloads specific files from a remote Drop to the current directory.

Examples:
  ftr remote down user/repo/file.txt
  ftr remote down user/repo/file1.txt user/repo/file2.txt`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Deduplicate file paths while preserving order
		seen := make(map[string]struct{})
		filePaths := []string{}
		for _, path := range args {
			if _, ok := seen[path]; ok {
				continue
			}
			seen[path] = struct{}{}
			filePaths = append(filePaths, path)
		}

		client, err := api.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create API client: %w", err)
		}

		// Parallel download with a small concurrency limit
		sem := make(chan struct{}, 6)
		errCh := make(chan error, len(filePaths))

		for _, fullPath := range filePaths {
			fullPath := fullPath
			parts := strings.SplitN(fullPath, "/", 3)
			if len(parts) < 3 {
				fmt.Fprintf(os.Stderr, "invalid path format: %s (must be <user>/<repo>/<file-path>)\n", fullPath)
				errCh <- fmt.Errorf("invalid path format")
				continue
			}
			user, repo, filePath := parts[0], parts[1], parts[2]
			repoPath := fmt.Sprintf("%s/%s", user, repo)

			sem <- struct{}{}
			go func() {
				defer func() { <-sem }()

				destPath := filepath.Base(filePath)
				fmt.Printf("Downloading '%s' from %s to '%s'...\n", filePath, repoPath, destPath)

				if err := client.DownloadAndVerify(user, repo, filePath, destPath, nil); err != nil {
					fmt.Fprintf(os.Stderr, "failed to download %s: %v\n", fullPath, err)
					errCh <- err
					return
				}

				fmt.Printf("Successfully downloaded %s.\n", destPath)
				errCh <- nil
			}()
		}

		// wait for all goroutines to finish
		for i := 0; i < cap(sem); i++ {
			sem <- struct{}{}
		}
		close(errCh)

		var lastErr error
		for e := range errCh {
			if e != nil {
				lastErr = e
			}
		}

		if lastErr != nil {
			return fmt.Errorf("one or more downloads failed")
		}
		return nil
	},
}

func init() {
	remoteCmd.AddCommand(remoteRemoveCmd)
	remoteCmd.AddCommand(remoteDownCmd)
	remoteCmd.AddCommand(remoteRenameCmd)
	remoteCmd.AddCommand(remoteMkdirCmd)
}

type remotePath struct {
	user string
	repo string
	path string
}

func parseRemotePath(raw string) (remotePath, error) {
	parts := strings.SplitN(strings.TrimSpace(raw), "/", 3)
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return remotePath{}, fmt.Errorf("invalid remote path %q; expected <user>/<repo>/<path>", raw)
	}
	return remotePath{user: parts[0], repo: parts[1], path: parts[2]}, nil
}

func parseRemoteDestination(source remotePath, raw string) (string, error) {
	candidate := strings.TrimSpace(raw)
	if candidate == "" {
		return "", fmt.Errorf("destination path is required")
	}
	if full, err := parseRemotePath(candidate); err == nil {
		if full.user != source.user || full.repo != source.repo {
			return "", fmt.Errorf("cross-repository rename is not supported")
		}
		return full.path, nil
	}
	return candidate, nil
}

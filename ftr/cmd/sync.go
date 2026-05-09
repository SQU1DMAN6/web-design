package cmd

import (
	"bufio"
	"context"
	"crypto/sha256"
	"fmt"
	"ftr/pkg/api"
	"ftr/pkg/screen"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
)

// var defaultSyncDir string
var targetDirectory string

type LocalFileInfo struct {
	Info os.FileInfo
	Hash string
}

var syncCmd = &cobra.Command{
	Use:   "sync <user>/<repo> [flags]",
	Short: "Synchronise files with a repository",
	Long: `Snchronise files between your local sync directory and a remote repository.
Files are compared by name and timestamp to detect conflicts.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repoPath := args[0]
		parts := strings.Split(repoPath, "/")
		if len(parts) != 2 {
			return fmt.Errorf("repository path must be in format user/repo")
		}
		user, repo := parts[0], parts[1]

		encrypt, _ := cmd.Flags().GetBool("encrypt")
		workers, _ := cmd.Flags().GetInt("workers")
		autoChoice, _ := cmd.Flags().GetString("auto")
		askConflicts, _ := cmd.Flags().GetBool("ask")

		client, err := api.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}

		// Get sync directory
		syncDir := targetDirectory
		if syncDir == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to determine home directory: %w", err)
			}
			syncDir = filepath.Join(home, "FtR", user, repo)
		}
		if err := os.MkdirAll(syncDir, 0755); err != nil {
			return fmt.Errorf("failed to create sync directory: %w", err)
		}

		fmt.Printf("Syncing %s/%s with %s\n", user, repo, syncDir)

		watchMode, _ := cmd.Flags().GetBool("watch")
		pollInterval, _ := cmd.Flags().GetDuration("poll-interval")
		if watchMode {
			fmt.Printf("Starting live sync daemon for %s/%s in %s\n", user, repo, syncDir)
			return runLiveSync(context.Background(), client, user, repo, syncDir, encrypt, workers, autoChoice, askConflicts, pollInterval)
		}

		return performSyncOnce(client, repoPath, syncDir, encrypt, workers, autoChoice, askConflicts)
	},
}

func performSyncOnce(client *api.Client, repoPath, syncDir string, encrypt bool, workers int, autoChoice string, askConflicts bool) error {
	parts := strings.Split(repoPath, "/")
	user, repo := parts[0], parts[1]

	// List remote files
	remoteFiles, err := client.ListRepoFiles(user, repo)
	if err != nil {
		if strings.Contains(err.Error(), "invalid character '<'") {
			return screen.SuggestLoginError(fmt.Errorf("failed to list repo files: %w", err))
		}
		return fmt.Errorf("failed to list remote files: %w", err)
	}

	// List local files
	localFiles := make(map[string]LocalFileInfo)
	if err := filepath.Walk(syncDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			rel, _ := filepath.Rel(syncDir, path)
			rel = filepath.ToSlash(rel)

			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()

			h := sha256.New()
			if _, err := io.Copy(h, f); err != nil {
				return err
			}

			localFiles[rel] = LocalFileInfo{Info: info, Hash: fmt.Sprintf("%x", h.Sum(nil))}
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to scan and hash local files: %w", err)
	}

	// Build task lists: uploads, downloads, conflicts
	uploads := []string{}   // local only or newer
	downloads := []string{} // remote only or newer
	conflicts := []string{} // both exist with different content

	remoteMap := make(map[string]map[string]interface{})
	for _, rf := range remoteFiles {
		if path, ok := rf["path"].(string); ok {
			remoteMap[path] = rf
		}
	}

	// Find uploads and conflicts
	for localPath, localFile := range localFiles {
		remoteFile, exists := remoteMap[localPath]
		if exists {
			// File exists on both sides - check hashes
			if remoteHash, ok := remoteFile["hash"].(string); ok {
				if !strings.EqualFold(remoteHash, localFile.Hash) {
					conflicts = append(conflicts, localPath)
				}
			}
		} else {
			// Local only - upload
			uploads = append(uploads, localPath)
		}
	}

	// Find downloads
	for remoteName := range remoteMap {
		if _, exists := localFiles[remoteName]; !exists {
			downloads = append(downloads, remoteName)
		}
	}

	fmt.Printf("Will upload %d new local files.\n", len(uploads))
	fmt.Printf("Will download %d new remote files.\n", len(downloads))
	fmt.Printf("Will handle %d conflicts.\n", len(conflicts))

	if askConflicts {
		applyAll := false
		var applyAction string
		for _, conflict := range conflicts {
			if applyAll {
				switch applyAction {
				case "u":
					uploads = append(uploads, conflict)
				case "d":
					downloads = append(downloads, conflict)
				case "b":
					downloads = append(downloads, conflict)
					uploads = append(uploads, conflict)
				case "s":
					// skip
				}
				continue
			}

			fmt.Printf("Conflict: %s\n", conflict)
			lf := localFiles[conflict]
			fmt.Printf("  Local:  %s (modified: %s, size: %d)\n", conflict, lf.Info.ModTime().Format("2006-01-02 15:04:05"), lf.Info.Size())

			if rf, ok := remoteMap[conflict]; ok {
				if m, ok := rf["modified"].(float64); ok {
					t := time.Unix(int64(m), 0).Format("2006-01-02 15:04:05")
					sz := int64(0)
					if s, ok := rf["size"].(float64); ok {
						sz = int64(s)
					}
					fmt.Printf("  Remote: %s (modified: %s, size: %d)\n", conflict, t, sz)
				} else {
					fmt.Printf("  Remote: %s (modified: unknown)\n", conflict)
				}
			}

			defaultChoice := "s"
			localTime := lf.Info.ModTime().Unix()
			remoteTime := int64(0)
			if rf, ok := remoteMap[conflict]; ok {
				if m, ok := rf["modified"].(float64); ok {
					remoteTime = int64(m)
				}
			}
			if localTime > remoteTime {
				defaultChoice = "u"
			}
			if remoteTime > localTime {
				defaultChoice = "d"
			}

			fmt.Printf("  Choose: [u]pload local, [d]ownload remote, [s]kip, [b]oth (keep both) (default: %s): ", defaultChoice)
			reader := bufio.NewReader(os.Stdin)
			choice, _ := reader.ReadString('\n')
			choice = strings.TrimSpace(choice)
			if choice == "" {
				choice = defaultChoice
			}

			switch choice {
			case "u":
				uploads = append(uploads, conflict)
			case "d":
				downloads = append(downloads, conflict)
			case "b":
				downloads = append(downloads, conflict)
				uploads = append(uploads, conflict)
			case "s":
				// skip
			default:
				fmt.Println("Unrecognized choice; skipping")
			}

			fmt.Printf("  Apply this action to all remaining conflicts? [y/N] ")
			resp, _ := reader.ReadString('\n')
			resp = strings.TrimSpace(resp)
			if strings.EqualFold(resp, "y") {
				applyAll = true
				if choice == "u" || choice == "d" || choice == "b" || choice == "s" {
					applyAction = choice
				}
			}
		}
	} else {
		switch autoChoice {
		case "local":
			uploads = append(uploads, conflicts...)
		case "remote":
			downloads = append(downloads, conflicts...)
		case "both":
			for _, c := range conflicts {
				downloads = append(downloads, c)
				uploads = append(uploads, c)
			}
		case "skip":
			// do nothing
		default:
			for _, c := range conflicts {
				lf, lok := localFiles[c]
				rf, rok := remoteMap[c]
				localTime := int64(0)
				remoteTime := int64(0)
				if lok {
					localTime = lf.Info.ModTime().Unix()
				}
				if rok {
					if m, ok := rf["modified"].(float64); ok {
						remoteTime = int64(m)
					}
				}
				if localTime >= remoteTime {
					uploads = append(uploads, c)
				} else {
					downloads = append(downloads, c)
				}
			}
		}
	}

	var errsMu sync.Mutex
	errorsList := []string{}
	type syncTask struct {
		isUpload bool
		path     string
	}
	tasksChan := make(chan syncTask, len(uploads)+len(downloads))
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range tasksChan {
				if task.isUpload {
					localPath := task.path
					fullPath := filepath.Join(syncDir, localPath)
					if file, err := os.Open(fullPath); err == nil {
						info, _ := file.Stat()
						err = client.UploadFile(repoPath, localPath, file, info.Size(), encrypt, nil)
						file.Close()
						if err != nil {
							msg := fmt.Sprintf("Upload failed: %s: %v", localPath, err)
							errsMu.Lock()
							errorsList = append(errorsList, msg)
							errsMu.Unlock()
						}
					} else {
						msg := fmt.Sprintf("Cannot open local file: %s: %v", localPath, err)
						errsMu.Lock()
						errorsList = append(errorsList, msg)
						errsMu.Unlock()
					}
				} else {
					remoteName := task.path
					destPath := filepath.Join(syncDir, remoteName)
					os.MkdirAll(filepath.Dir(destPath), 0755)
					err = client.DownloadAndVerify(user, repo, remoteName, destPath, nil)
					if err != nil {
						msg := fmt.Sprintf("Download failed: %s: %v", remoteName, err)
						errsMu.Lock()
						errorsList = append(errorsList, msg)
						errsMu.Unlock()
					}
				}
			}
		}()
	}

	for _, path := range uploads {
		tasksChan <- syncTask{isUpload: true, path: path}
	}
	for _, path := range downloads {
		tasksChan <- syncTask{isUpload: false, path: path}
	}
	close(tasksChan)

	wg.Wait()

	if len(errorsList) > 0 {
		fmt.Printf("\nErrors encountered during sync:\n")
		for _, e := range errorsList {
			fmt.Printf("- %s\n", e)
		}
	}

	fmt.Printf("\nSync complete!\n")
	return nil
}

func runLiveSync(ctx context.Context, client *api.Client, user, repo, syncDir string, encrypt bool, workers int, autoChoice string, askConflicts bool, pollInterval time.Duration) error {
	if pollInterval <= 0 {
		pollInterval = 30 * time.Second
	}
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}
	defer watcher.Close()

	if err := addWatcherRecursive(watcher, syncDir); err != nil {
		return fmt.Errorf("watch setup failed: %w", err)
	}

	fmt.Printf("Watching %s for changes.\n", syncDir)
	fmt.Println("Performing initial sync...")
	fmt.Println("Press Ctrl+C to stop live sync.")
	if err := performSyncOnce(client, fmt.Sprintf("%s/%s", user, repo), syncDir, encrypt, workers, autoChoice, askConflicts); err != nil {
		fmt.Printf("Initial sync failed: %v\n", err)
	}

	syncTriggered := make(chan struct{}, 1)
	triggerSync := func() {
		select {
		case syncTriggered <- struct{}{}:
		default:
		}
	}

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if shouldHandleEvent(event) {
					fmt.Printf("Local change detected: %s %s\n", event.Name, event.Op)
					if event.Op&fsnotify.Create == fsnotify.Create {
						if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
							_ = addWatcherRecursive(watcher, event.Name)
						}
					}
					triggerSync()
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				fmt.Printf("Watcher error: %v\n", err)
			case <-ctx.Done():
				return
			}
		}
	}()

	timer := time.NewTimer(pollInterval)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("Stopping live sync.")
			return nil
		case <-syncTriggered:
			fmt.Println("Syncing due to local change...")
			if err := performSyncOnce(client, fmt.Sprintf("%s/%s", user, repo), syncDir, encrypt, workers, autoChoice, askConflicts); err != nil {
				fmt.Printf("Live sync error: %v\n", err)
			}
			if !timer.Stop() {
				<-timer.C
			}
			timer.Reset(pollInterval)
		case <-timer.C:
			fmt.Println("Polling remote repository for changes...")
			if err := performSyncOnce(client, fmt.Sprintf("%s/%s", user, repo), syncDir, encrypt, workers, autoChoice, askConflicts); err != nil {
				fmt.Printf("Live sync error: %v\n", err)
			}
			timer.Reset(pollInterval)
		}
	}
}

func addWatcherRecursive(watcher *fsnotify.Watcher, root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return watcher.Add(path)
		}
		return nil
	})
}

func shouldHandleEvent(event fsnotify.Event) bool {
	return event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) != 0
}

func init() {
	syncCmd.Flags().BoolP("encrypt", "E", false, "Encrypt uploaded files")
	syncCmd.Flags().IntP("workers", "w", 10, "Number of parallel workers")
	syncCmd.Flags().String("auto", "ask", "Auto-resolve conflicts: ask|local|remote|skip|both")
	syncCmd.Flags().BoolP("ask", "A", false, "Ask interactively about conflicts")
	syncCmd.Flags().BoolP("watch", "W", false, "Watch local directory and sync continuously")
	syncCmd.Flags().Duration("poll-interval", 30*time.Second, "Remote polling interval in watch mode")
	syncCmd.Flags().StringVarP(&targetDirectory, "target", "T", "", "Target directory to sync remote repository with (defaults to ~/FtR/user/repo)")
}

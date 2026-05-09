package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"ftr/pkg/api"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"github.com/spf13/cobra"
)

type RemoteFS struct {
	client          *api.Client
	user            string
	repo            string
	root            *RemoteDir
	refreshInterval time.Duration
	lastRefresh     time.Time
	readOnly        bool
	mu              sync.Mutex
}

type RemoteDir struct {
	fsys    *RemoteFS
	path    string
	entries map[string]fs.Node
	mu      sync.Mutex
}

type RemoteFile struct {
	fsys  *RemoteFS
	path  string
	size  uint64
	mtime time.Time
	hash  string
}

type RemoteFileHandle struct {
	file     *os.File
	fileNode *RemoteFile
	writable bool
	dirty    bool
	mu       sync.Mutex
}

var mountCmd = &cobra.Command{
	Use:   "mount <user>/<repo> [mountpoint]",
	Short: "Mount a remote FtR repository as a network directory",
	Long: `Mount a remote FtR repository into a local mount point using FUSE.
Files are presented as placeholders and contents are downloaded only when opened.
Writes are uploaded back to the remote repository when file handles are closed.
`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		repoPath := args[0]
		parts := strings.Split(repoPath, "/")
		if len(parts) != 2 {
			return fmt.Errorf("repository path must be in format user/repo")
		}
		user, repo := parts[0], parts[1]

		mountPoint := ""
		if len(args) == 2 {
			mountPoint = args[1]
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to determine home directory: %w", err)
			}
			mountPoint = filepath.Join(home, "FtR", user, repo)
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
			return fmt.Errorf("mount point %s is not a directory", mountPoint)
		}

		client, err := api.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create API client: %w", err)
		}
		client.SetStatsEnabled(false)

		fileList, err := client.FSListRepo(user, repo)
		if err != nil {
			return fmt.Errorf("failed to list remote repository: %w", err)
		}

		rfs, err := NewRemoteFS(client, user, repo, fileList)
		if err != nil {
			return err
		}

		mountOpts := []fuse.MountOption{
			fuse.FSName("ftr"),
			fuse.Subtype("ftrfs"),
		}
		if mountReadOnly {
			mountOpts = append(mountOpts, fuse.ReadOnly())
		}

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		conn, err := fuse.Mount(mountPoint, mountOpts...)
		if err != nil {
			return fmt.Errorf("failed to mount filesystem: %w", err)
		}
		defer conn.Close()

		go func() {
			<-ctx.Done()
			_ = fuse.Unmount(mountPoint)
		}()

		fmt.Printf("Mounted %s at %s\n", repoPath, mountPoint)
		return fs.Serve(conn, rfs)
	},
}

func NewRemoteFS(client *api.Client, user, repo string, fileList []api.RepoEntry) (*RemoteFS, error) {
	refreshInterval := mountRefreshInterval
	if refreshInterval <= 0 {
		refreshInterval = 500 * time.Millisecond
	}
	rfs := &RemoteFS{
		client:          client,
		user:            user,
		repo:            repo,
		refreshInterval: refreshInterval,
		readOnly:        mountReadOnly,
	}

	rfs.root = buildRemoteTree(rfs, fileList)
	rfs.lastRefresh = time.Now()
	return rfs, nil
}

func (rfs *RemoteFS) Root() (fs.Node, error) {
	return rfs.root, nil
}

func (rfs *RemoteFS) refreshTreeIfNeeded() error {
	rfs.mu.Lock()
	if time.Since(rfs.lastRefresh) < rfs.refreshInterval {
		rfs.mu.Unlock()
		return nil
	}
	rfs.mu.Unlock()

	fileList, err := rfs.client.FSListRepo(rfs.user, rfs.repo)
	if err != nil {
		return err
	}

	newRoot := buildRemoteTree(rfs, fileList)

	rfs.root.mu.Lock()
	rfs.root.entries = newRoot.entries
	rfs.root.mu.Unlock()

	rfs.mu.Lock()
	rfs.lastRefresh = time.Now()
	rfs.mu.Unlock()
	return nil
}

func buildRemoteTree(rfs *RemoteFS, fileList []api.RepoEntry) *RemoteDir {
	root := &RemoteDir{
		fsys:    rfs,
		path:    "",
		entries: make(map[string]fs.Node),
	}

	for _, entry := range fileList {
		remotePath := strings.Trim(entry.Path, "/")
		if remotePath == "" {
			continue
		}
		size := uint64(max(entry.Size, 0))
		mtime := time.Unix(entry.Modified, 0)
		if entry.Modified <= 0 {
			mtime = time.Now()
		}
		hash := entry.Hash

		parts := strings.Split(remotePath, "/")
		current := root
		for idx, part := range parts {
			if part == "" {
				continue
			}
			if idx == len(parts)-1 {
				if entry.Type == "dir" {
					if _, exists := current.entries[part]; !exists {
						current.entries[part] = &RemoteDir{
							fsys:    rfs,
							path:    path.Join(current.path, part),
							entries: make(map[string]fs.Node),
						}
					}
					continue
				}
				current.entries[part] = &RemoteFile{
					fsys:  rfs,
					path:  remotePath,
					size:  size,
					mtime: mtime,
					hash:  hash,
				}
				continue
			}
			node, ok := current.entries[part]
			if !ok {
				dir := &RemoteDir{
					fsys:    rfs,
					path:    path.Join(current.path, part),
					entries: make(map[string]fs.Node),
				}
				current.entries[part] = dir
				node = dir
			}
			if dir, ok := node.(*RemoteDir); ok {
				current = dir
			} else {
				break
			}
		}
	}

	return root
}

func (d *RemoteDir) refreshEntriesFromRoot() {
	if d.path == "" {
		return
	}
	current := d.fsys.root
	for _, part := range strings.Split(d.path, "/") {
		if part == "" {
			continue
		}
		current.mu.Lock()
		node := current.entries[part]
		current.mu.Unlock()
		next, ok := node.(*RemoteDir)
		if !ok {
			return
		}
		current = next
	}
	current.mu.Lock()
	entries := current.entries
	current.mu.Unlock()

	d.mu.Lock()
	d.entries = entries
	d.mu.Unlock()
}

func (d *RemoteDir) Attr(ctx context.Context, a *fuse.Attr) error {
	if d.fsys.readOnly {
		a.Mode = os.ModeDir | 0555
	} else {
		a.Mode = os.ModeDir | 0755
	}
	return nil
}

func (d *RemoteDir) Lookup(ctx context.Context, name string) (fs.Node, error) {
	if err := d.fsys.refreshTreeIfNeeded(); err != nil {
		return nil, err
	}
	d.refreshEntriesFromRoot()

	d.mu.Lock()
	node, ok := d.entries[name]
	d.mu.Unlock()
	if !ok {
		return nil, fuse.ENOENT
	}
	return node, nil
}

func (d *RemoteDir) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	if err := d.fsys.refreshTreeIfNeeded(); err != nil {
		return nil, err
	}
	d.refreshEntriesFromRoot()

	d.mu.Lock()
	de := make([]fuse.Dirent, 0, len(d.entries))
	for name, node := range d.entries {
		switch node.(type) {
		case *RemoteDir:
			de = append(de, fuse.Dirent{Name: name, Type: fuse.DT_Dir})
		case *RemoteFile:
			de = append(de, fuse.Dirent{Name: name, Type: fuse.DT_File})
		}
	}
	d.mu.Unlock()
	return de, nil
}

func (d *RemoteDir) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (fs.Node, fs.Handle, error) {
	if d.fsys.readOnly {
		return nil, nil, fuse.Errno(syscall.EROFS)
	}
	filePath := path.Join(d.path, req.Name)
	d.mu.Lock()
	if _, ok := d.entries[req.Name]; ok {
		d.mu.Unlock()
		return nil, nil, fuse.EEXIST
	}
	file := &RemoteFile{
		fsys: d.fsys,
		path: filePath,
	}
	d.entries[req.Name] = file
	d.mu.Unlock()

	handle, err := file.openHandle(req.Flags, true)
	if err != nil {
		d.mu.Lock()
		delete(d.entries, req.Name)
		d.mu.Unlock()
		return nil, nil, err
	}

	return file, handle, nil
}

func (d *RemoteDir) Remove(ctx context.Context, req *fuse.RemoveRequest) error {
	if d.fsys.readOnly {
		return fuse.Errno(syscall.EROFS)
	}

	d.mu.Lock()
	node, ok := d.entries[req.Name]
	if !ok {
		d.mu.Unlock()
		return fuse.ENOENT
	}
	if _, ok := node.(*RemoteDir); ok && !req.Dir {
		d.mu.Unlock()
		return fuse.Errno(syscall.EISDIR)
	}
	if _, ok := node.(*RemoteFile); ok && req.Dir {
		d.mu.Unlock()
		return fuse.Errno(syscall.ENOTDIR)
	}
	d.mu.Unlock()

	targetPath := path.Join(d.path, req.Name)
	if err := d.fsys.client.FSDeletePath(d.fsys.user, d.fsys.repo, targetPath); err != nil {
		return err
	}

	d.mu.Lock()
	delete(d.entries, req.Name)
	d.mu.Unlock()
	return nil
}

func (d *RemoteDir) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (fs.Node, error) {
	if d.fsys.readOnly {
		return nil, fuse.Errno(syscall.EROFS)
	}
	d.mu.Lock()
	if _, ok := d.entries[req.Name]; ok {
		d.mu.Unlock()
		return nil, fuse.EEXIST
	}
	d.mu.Unlock()
	dirPath := path.Join(d.path, req.Name)
	if err := d.fsys.client.FSMkdir(d.fsys.user, d.fsys.repo, dirPath); err != nil {
		return nil, err
	}
	dir := &RemoteDir{
		fsys:    d.fsys,
		path:    dirPath,
		entries: make(map[string]fs.Node),
	}
	d.mu.Lock()
	d.entries[req.Name] = dir
	d.mu.Unlock()
	return dir, nil
}

func (d *RemoteDir) Rename(ctx context.Context, req *fuse.RenameRequest, newDir fs.Node) error {
	if d.fsys.readOnly {
		return fuse.Errno(syscall.EROFS)
	}
	targetDir, ok := newDir.(*RemoteDir)
	if !ok {
		return fuse.Errno(syscall.ENOTDIR)
	}
	d.mu.Lock()
	node := d.entries[req.OldName]
	d.mu.Unlock()
	if node == nil {
		return fuse.ENOENT
	}
	targetDir.mu.Lock()
	if _, exists := targetDir.entries[req.NewName]; exists {
		targetDir.mu.Unlock()
		return fuse.EEXIST
	}
	targetDir.mu.Unlock()
	oldPath := path.Join(d.path, req.OldName)
	newPath := path.Join(targetDir.path, req.NewName)
	if err := d.fsys.client.FSRenamePath(d.fsys.user, d.fsys.repo, oldPath, newPath); err != nil {
		return err
	}

	d.mu.Lock()
	delete(d.entries, req.OldName)
	d.mu.Unlock()
	updateRemoteNodePath(node, oldPath, newPath)
	targetDir.mu.Lock()
	targetDir.entries[req.NewName] = node
	targetDir.mu.Unlock()
	return nil
}

func updateRemoteNodePath(node fs.Node, oldPath string, newPath string) {
	switch typed := node.(type) {
	case *RemoteFile:
		typed.path = newPath
	case *RemoteDir:
		typed.path = newPath
		typed.mu.Lock()
		children := make([]fs.Node, 0, len(typed.entries))
		for _, child := range typed.entries {
			children = append(children, child)
		}
		typed.mu.Unlock()
		for _, child := range children {
			switch childNode := child.(type) {
			case *RemoteFile:
				rel := strings.TrimPrefix(childNode.path, strings.TrimSuffix(oldPath, "/")+"/")
				childNode.path = path.Join(newPath, rel)
			case *RemoteDir:
				rel := strings.TrimPrefix(childNode.path, strings.TrimSuffix(oldPath, "/")+"/")
				updateRemoteNodePath(childNode, childNode.path, path.Join(newPath, rel))
			}
		}
	}
}

func (f *RemoteFile) Attr(ctx context.Context, a *fuse.Attr) error {
	if f.fsys.readOnly {
		a.Mode = 0444
	} else {
		a.Mode = 0644
	}
	a.Size = f.size
	a.Mtime = f.mtime
	return nil
}

func (f *RemoteFile) Setattr(ctx context.Context, req *fuse.SetattrRequest, resp *fuse.SetattrResponse) error {
	if f.fsys.readOnly {
		return fuse.Errno(syscall.EROFS)
	}
	if req.Valid.Size() {
		tempFile, err := os.CreateTemp("", "ftr-mount-setattr-*")
		if err != nil {
			return err
		}
		tempName := tempFile.Name()
		defer func() {
			tempFile.Close()
			os.Remove(tempName)
		}()

		if req.Size > 0 && f.size > 0 {
			if err := f.downloadTo(tempFile); err != nil {
				return err
			}
		}
		if err := tempFile.Truncate(int64(req.Size)); err != nil {
			return err
		}
		if _, err := tempFile.Seek(0, 0); err != nil {
			return err
		}
		if err := f.uploadFrom(tempFile, int64(req.Size)); err != nil {
			return err
		}
	}
	return f.Attr(ctx, &resp.Attr)
}

func (f *RemoteFile) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	return f.openHandle(req.Flags, false)
}

func (f *RemoteFile) openHandle(flags fuse.OpenFlags, created bool) (fs.Handle, error) {
	writable := flags.IsWriteOnly() || flags.IsReadWrite()
	truncate := flags&fuse.OpenTruncate != 0
	if f.fsys.readOnly && writable {
		return nil, fuse.Errno(syscall.EROFS)
	}

	tempFile, err := os.CreateTemp("", "ftr-mount-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary file: %w", err)
	}

	if !created && !truncate && f.size > 0 {
		if err := f.downloadTo(tempFile); err != nil {
			tempFile.Close()
			os.Remove(tempFile.Name())
			return nil, err
		}
		if _, err := tempFile.Seek(0, 0); err != nil {
			tempFile.Close()
			os.Remove(tempFile.Name())
			return nil, err
		}
	}

	return &RemoteFileHandle{file: tempFile, fileNode: f, writable: writable, dirty: writable && (created || truncate)}, nil
}

func (f *RemoteFile) downloadTo(dest *os.File) error {
	if err := f.fsys.client.FSDownloadTo(f.fsys.user, f.fsys.repo, f.path, dest.Name()); err != nil {
		return err
	}
	return nil
}

func (f *RemoteFile) uploadFrom(src *os.File, size int64) error {
	if _, err := src.Seek(0, 0); err != nil {
		return err
	}
	if err := f.fsys.client.FSWriteFile(f.fsys.user, f.fsys.repo, f.path, src, size); err != nil {
		return err
	}
	f.size = uint64(size)
	f.mtime = time.Now()
	return nil
}

func (h *RemoteFileHandle) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	buf := make([]byte, req.Size)
	n, err := h.file.ReadAt(buf, req.Offset)
	if err != nil && err != io.EOF {
		return err
	}
	resp.Data = buf[:n]
	return nil
}

func (h *RemoteFileHandle) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	if !h.writable {
		return fuse.EPERM
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	n, err := h.file.WriteAt(req.Data, req.Offset)
	if err != nil {
		return err
	}
	h.dirty = true
	resp.Size = n
	return nil
}

func (h *RemoteFileHandle) Release(ctx context.Context, req *fuse.ReleaseRequest) error {
	if err := h.Flush(ctx, nil); err != nil {
		h.file.Close()
		os.Remove(h.file.Name())
		return err
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	name := h.file.Name()
	err := h.file.Close()
	if err != nil {
		os.Remove(name)
		return err
	}
	return os.Remove(name)
}

func (h *RemoteFileHandle) Flush(ctx context.Context, req *fuse.FlushRequest) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.dirty {
		return nil
	}
	info, err := h.file.Stat()
	if err != nil {
		return err
	}
	if err := h.fileNode.uploadFrom(h.file, info.Size()); err != nil {
		return err
	}
	h.dirty = false
	return nil
}

func isTransportEndpointNotConnected(err error) bool {
	if errors.Is(err, syscall.ENOTCONN) {
		return true
	}
	var pathErr *os.PathError
	if errors.As(err, &pathErr) && errors.Is(pathErr.Err, syscall.ENOTCONN) {
		return true
	}
	return strings.Contains(err.Error(), "transport endpoint is not connected")
}

var mountReadOnly bool
var mountRefreshInterval time.Duration

func init() {
	mountCmd.Flags().BoolVar(&mountReadOnly, "readonly", false, "Mount repository read-only")
	mountCmd.Flags().DurationVar(&mountRefreshInterval, "refresh-interval", 500*time.Millisecond, "How often the mounted repository checks for remote changes")
	rootCmd.AddCommand(mountCmd)
}

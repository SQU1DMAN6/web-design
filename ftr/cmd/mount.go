package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"os/user"
	"path"
	"path/filepath"
	"sort"
	"strconv"
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
	refreshMu       sync.Mutex
	server          *fs.Server
	defaultUid      uint32
	defaultGid      uint32
}

type RemoteDir struct {
	fsys    *RemoteFS
	path    string
	entries map[string]fs.Node
	mu      sync.Mutex
	mode    os.FileMode
	uid     uint32
	gid     uint32
}

type RemoteFile struct {
	fsys  *RemoteFS
	path  string
	size  uint64
	mtime time.Time
	hash  string
	mode  os.FileMode
	uid   uint32
	gid   uint32
}

type RemoteFileHandle struct {
	file     *os.File
	fileNode *RemoteFile
	writable bool
	dirty    bool
	mu       sync.Mutex
}

type remoteEntryState struct {
	path     string
	kind     string
	size     int64
	modified int64
	hash     string
}

type remoteChangeKind int

const (
	remoteChangeAdded remoteChangeKind = iota
	remoteChangeRemoved
	remoteChangeModified
)

type remoteChange struct {
	kind   remoteChangeKind
	parent *RemoteDir
	node   fs.Node
	name   string
}

var mountCmd = &cobra.Command{
	Use:   "mount <user>/<repo> [mountpoint]",
	Short: "Mount a remote FtR Drop as a network directory",
	Long: `Mount a remote FtR Drop into a local mount point using FUSE.
Files are presented as placeholders and contents are downloaded only when opened.
Writes are uploaded back to the remote Drop when file handles are closed.
`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		repoPath := args[0]
		parts := strings.Split(repoPath, "/")
		if len(parts) != 2 {
			return fmt.Errorf("Drop path must be in format user/repo")
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
			return fmt.Errorf("failed to list remote Drop: %w", err)
		}

		rfs, err := NewRemoteFS(client, user, repo, fileList)
		if err != nil {
			return err
		}

		mountOpts := []fuse.MountOption{
			fuse.FSName("ftr"),
			fuse.Subtype("ftrfs"),
			fuse.AllowOther(),
			fuse.AllowSUID(),
			fuse.DefaultPermissions(),
		}
		if mountReadOnly {
			mountOpts = append(mountOpts, fuse.ReadOnly())
		}

		ctx, stop := signal.NotifyContext(
			context.Background(),
			os.Interrupt,
			syscall.SIGTERM,
		)
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

		server := fs.New(conn, nil)
		rfs.setServer(server)
		go rfs.watchRemoteChanges(ctx)

		fmt.Printf("Mounted %s at %s\n", repoPath, mountPoint)
		return server.Serve(rfs)
	},
}

func currentUserIds() (uint32, uint32) {
	uid := uint32(0)
	gid := uint32(0)
	current, err := user.Current()
	if err == nil {
		if parsed, err := strconv.ParseUint(current.Uid, 10, 32); err == nil {
			uid = uint32(parsed)
		}
		if parsed, err := strconv.ParseUint(current.Gid, 10, 32); err == nil {
			gid = uint32(parsed)
		}
	}
	return uid, gid
}

func NewRemoteFS(client *api.Client, user, repo string, fileList []api.RepoEntry) (*RemoteFS, error) {
	refreshInterval := mountRefreshInterval
	if refreshInterval <= 0 {
		refreshInterval = 500 * time.Millisecond
	}
	defaultUid, defaultGid := currentUserIds()
	rfs := &RemoteFS{
		client:          client,
		user:            user,
		repo:            repo,
		refreshInterval: refreshInterval,
		readOnly:        mountReadOnly,
		defaultUid:      defaultUid,
		defaultGid:      defaultGid,
	}

	rfs.root = buildRemoteTree(rfs, fileList)
	rfs.lastRefresh = time.Now()
	return rfs, nil
}

func (rfs *RemoteFS) Root() (fs.Node, error) {
	return rfs.root, nil
}

func (rfs *RemoteFS) Statfs(ctx context.Context, req *fuse.StatfsRequest, resp *fuse.StatfsResponse) error {
	resp.Blocks = 1024 * 1024 * 1024
	resp.Bfree = 4 * 1024 * 1024 * 1024
	resp.Bavail = 4 * 1024 * 1024 * 1024
	resp.Files = 1 << 20
	resp.Ffree = 1 << 20
	resp.Bsize = 4096
	resp.Namelen = 255
	resp.Frsize = 4096
	return nil
}

func (rfs *RemoteFS) cacheTTL() time.Duration {
	if rfs.refreshInterval <= 0 {
		return 500 * time.Millisecond
	}
	return rfs.refreshInterval
}

func (rfs *RemoteFS) setServer(server *fs.Server) {
	rfs.mu.Lock()
	rfs.server = server
	rfs.mu.Unlock()
}

func (rfs *RemoteFS) currentServer() *fs.Server {
	rfs.mu.Lock()
	defer rfs.mu.Unlock()
	return rfs.server
}

func (rfs *RemoteFS) watchRemoteChanges(ctx context.Context) {
	ticker := time.NewTicker(rfs.cacheTTL())
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := rfs.refreshTree(true); err != nil {
				fmt.Fprintf(os.Stderr, "ftr mount refresh failed: %v\n", err)
			}
		}
	}
}

func (rfs *RemoteFS) refreshTreeIfNeeded() error {
	rfs.mu.Lock()
	if time.Since(rfs.lastRefresh) < rfs.refreshInterval {
		rfs.mu.Unlock()
		return nil
	}
	rfs.mu.Unlock()

	return rfs.refreshTree(true)
}

func (rfs *RemoteFS) refreshTree(sendNotifications bool) error {
	rfs.refreshMu.Lock()
	defer rfs.refreshMu.Unlock()

	fileList, err := rfs.client.FSListRepo(rfs.user, rfs.repo)
	if err != nil {
		return err
	}

	changes := rfs.syncRemoteEntries(fileList)
	if sendNotifications {
		rfs.notifyRemoteChanges(changes)
	}

	rfs.mu.Lock()
	rfs.lastRefresh = time.Now()
	rfs.mu.Unlock()
	return nil
}

func (rfs *RemoteFS) syncRemoteEntries(fileList []api.RepoEntry) []remoteChange {
	desired := buildDesiredEntryState(fileList)
	existing := map[string]fs.Node{}
	collectRemoteNodes(rfs.root, existing)

	changes := make([]remoteChange, 0)
	deletePaths := make([]string, 0)
	for remotePath := range existing {
		if _, ok := desired[remotePath]; !ok {
			deletePaths = append(deletePaths, remotePath)
		}
	}
	sort.Slice(deletePaths, func(i, j int) bool {
		return remotePathDepth(deletePaths[i]) > remotePathDepth(deletePaths[j])
	})
	for _, remotePath := range deletePaths {
		parent, name := rfs.parentDirAndName(remotePath)
		if parent == nil || name == "" {
			continue
		}
		parent.mu.Lock()
		node := parent.entries[name]
		delete(parent.entries, name)
		parent.mu.Unlock()
		if node != nil {
			changes = append(changes, remoteChange{kind: remoteChangeRemoved, parent: parent, node: node, name: name})
		}
	}

	upsertPaths := make([]string, 0, len(desired))
	for remotePath := range desired {
		if remotePath != "" {
			upsertPaths = append(upsertPaths, remotePath)
		}
	}
	sort.Slice(upsertPaths, func(i, j int) bool {
		leftDepth := remotePathDepth(upsertPaths[i])
		rightDepth := remotePathDepth(upsertPaths[j])
		if leftDepth == rightDepth {
			return upsertPaths[i] < upsertPaths[j]
		}
		return leftDepth < rightDepth
	})

	for _, remotePath := range upsertPaths {
		entry := desired[remotePath]
		parent, name := rfs.ensureParentDir(remotePath, &changes)
		if parent == nil || name == "" {
			continue
		}

		parent.mu.Lock()
		node := parent.entries[name]
		if node == nil {
			node = rfs.newNodeFromState(entry)
			parent.entries[name] = node
			parent.mu.Unlock()
			changes = append(changes, remoteChange{kind: remoteChangeAdded, parent: parent, node: node, name: name})
			continue
		}

		currentKind := remoteNodeKind(node)
		if currentKind != entry.kind {
			oldNode := node
			node = rfs.newNodeFromState(entry)
			parent.entries[name] = node
			parent.mu.Unlock()
			changes = append(changes, remoteChange{kind: remoteChangeRemoved, parent: parent, node: oldNode, name: name})
			changes = append(changes, remoteChange{kind: remoteChangeAdded, parent: parent, node: node, name: name})
			continue
		}

		if file, ok := node.(*RemoteFile); ok && file.applyState(entry) {
			parent.mu.Unlock()
			changes = append(changes, remoteChange{kind: remoteChangeModified, parent: parent, node: file, name: name})
			continue
		}
		parent.mu.Unlock()
	}

	return changes
}

func buildDesiredEntryState(fileList []api.RepoEntry) map[string]remoteEntryState {
	desired := map[string]remoteEntryState{}
	for _, entry := range fileList {
		remotePath := cleanRemotePath(entry.Path)
		if remotePath == "" {
			continue
		}
		parts := strings.Split(remotePath, "/")
		for index := 1; index < len(parts); index++ {
			parentPath := strings.Join(parts[:index], "/")
			desired[parentPath] = remoteEntryState{path: parentPath, kind: "dir"}
		}
		kind := strings.ToLower(strings.TrimSpace(entry.Type))
		if kind != "dir" {
			kind = "file"
		}
		desired[remotePath] = remoteEntryState{
			path:     remotePath,
			kind:     kind,
			size:     entry.Size,
			modified: entry.Modified,
			hash:     entry.Hash,
		}
	}
	return desired
}

func collectRemoteNodes(dir *RemoteDir, out map[string]fs.Node) {
	dir.mu.Lock()
	snapshot := make(map[string]fs.Node, len(dir.entries))
	for name, node := range dir.entries {
		snapshot[name] = node
	}
	dir.mu.Unlock()

	for name, node := range snapshot {
		remotePath := path.Join(dir.path, name)
		out[remotePath] = node
		if childDir, ok := node.(*RemoteDir); ok {
			collectRemoteNodes(childDir, out)
		}
	}
}

func (rfs *RemoteFS) ensureParentDir(remotePath string, changes *[]remoteChange) (*RemoteDir, string) {
	parentPath, name := splitRemoteParent(remotePath)
	parent := rfs.root
	if parentPath == "" {
		return parent, name
	}

	for _, part := range strings.Split(parentPath, "/") {
		if part == "" {
			continue
		}
		parent.mu.Lock()
		node := parent.entries[part]
		if node == nil {
			dirPath := path.Join(parent.path, part)
			dir := &RemoteDir{
				fsys:    rfs,
				path:    dirPath,
				entries: make(map[string]fs.Node),
				mode:    os.ModeDir | 0755,
				uid:     rfs.defaultUid,
				gid:     rfs.defaultGid,
			}
			parent.entries[part] = dir
			parent.mu.Unlock()
			*changes = append(*changes, remoteChange{kind: remoteChangeAdded, parent: parent, node: dir, name: part})
			parent = dir
			continue
		}
		dir, ok := node.(*RemoteDir)
		if !ok {
			parent.mu.Unlock()
			return nil, ""
		}
		parent.mu.Unlock()
		parent = dir
	}
	return parent, name
}

func (rfs *RemoteFS) parentDirAndName(remotePath string) (*RemoteDir, string) {
	parentPath, name := splitRemoteParent(remotePath)
	if parentPath == "" {
		return rfs.root, name
	}
	dir := rfs.findDir(parentPath)
	return dir, name
}

func (rfs *RemoteFS) findDir(remotePath string) *RemoteDir {
	remotePath = cleanRemotePath(remotePath)
	if remotePath == "" {
		return rfs.root
	}
	current := rfs.root
	for _, part := range strings.Split(remotePath, "/") {
		current.mu.Lock()
		node := current.entries[part]
		current.mu.Unlock()
		next, ok := node.(*RemoteDir)
		if !ok {
			return nil
		}
		current = next
	}
	return current
}

func (rfs *RemoteFS) newNodeFromState(entry remoteEntryState) fs.Node {
	if entry.kind == "dir" {
		mode := os.ModeDir | 0755
		return &RemoteDir{
			fsys:    rfs,
			path:    entry.path,
			entries: make(map[string]fs.Node),
			mode:    mode,
			uid:     rfs.defaultUid,
			gid:     rfs.defaultGid,
		}
	}
	return &RemoteFile{
		fsys:  rfs,
		path:  entry.path,
		size:  uint64(max(entry.size, 0)),
		mtime: remoteEntryModTime(entry.modified),
		hash:  entry.hash,
		mode:  0644,
		uid:   rfs.defaultUid,
		gid:   rfs.defaultGid,
	}
}

func (f *RemoteFile) applyState(entry remoteEntryState) bool {
	newSize := uint64(max(entry.size, 0))
	newMtime := remoteEntryModTime(entry.modified)
	changed := f.size != newSize || !f.mtime.Equal(newMtime) || f.hash != entry.hash
	f.size = newSize
	f.mtime = newMtime
	f.hash = entry.hash
	return changed
}

func remoteEntryModTime(modified int64) time.Time {
	if modified <= 0 {
		return time.Now()
	}
	return time.Unix(modified, 0)
}

func remoteNodeKind(node fs.Node) string {
	switch node.(type) {
	case *RemoteDir:
		return "dir"
	case *RemoteFile:
		return "file"
	default:
		return ""
	}
}

func cleanRemotePath(raw string) string {
	clean := path.Clean("/" + strings.TrimSpace(raw))
	if clean == "." || clean == "/" {
		return ""
	}
	return strings.TrimPrefix(clean, "/")
}

func splitRemoteParent(remotePath string) (string, string) {
	remotePath = cleanRemotePath(remotePath)
	if remotePath == "" {
		return "", ""
	}
	parent, name := path.Split(remotePath)
	return strings.Trim(parent, "/"), strings.Trim(name, "/")
}

func remotePathDepth(remotePath string) int {
	remotePath = cleanRemotePath(remotePath)
	if remotePath == "" {
		return 0
	}
	return strings.Count(remotePath, "/") + 1
}

func buildRemoteTree(rfs *RemoteFS, fileList []api.RepoEntry) *RemoteDir {
	root := &RemoteDir{
		fsys:    rfs,
		path:    "",
		entries: make(map[string]fs.Node),
		mode:    os.ModeDir | 0755,
		uid:     rfs.defaultUid,
		gid:     rfs.defaultGid,
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
							mode:    os.ModeDir | 0755,
							uid:     rfs.defaultUid,
							gid:     rfs.defaultGid,
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
					mode:  0644,
					uid:   rfs.defaultUid,
					gid:   rfs.defaultGid,
				}
				continue
			}
			node, ok := current.entries[part]
			if !ok {
				dir := &RemoteDir{
					fsys:    rfs,
					path:    path.Join(current.path, part),
					entries: make(map[string]fs.Node),
					mode:    os.ModeDir | 0755,
					uid:     rfs.defaultUid,
					gid:     rfs.defaultGid,
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

func (rfs *RemoteFS) notifyRemoteChanges(changes []remoteChange) {
	if len(changes) == 0 {
		return
	}
	server := rfs.currentServer()
	if server == nil {
		return
	}
	changedDirs := map[*RemoteDir]struct{}{}
	for _, change := range changes {
		if change.parent != nil {
			changedDirs[change.parent] = struct{}{}
		}
		switch change.kind {
		case remoteChangeRemoved:
			if change.parent != nil {
				ignoreFuseNotifyError(server.NotifyDelete(change.parent, change.node, change.name))
				ignoreFuseNotifyError(server.InvalidateEntry(change.parent, change.name))
			}
			if change.node != nil {
				ignoreFuseNotifyError(server.InvalidateNodeData(change.node))
			}
		case remoteChangeAdded:
			if change.parent != nil {
				ignoreFuseNotifyError(server.InvalidateEntry(change.parent, change.name))
			}
			if change.node != nil {
				ignoreFuseNotifyError(server.InvalidateNodeData(change.node))
			}
		case remoteChangeModified:
			if change.parent != nil {
				ignoreFuseNotifyError(server.InvalidateEntry(change.parent, change.name))
			}
			if change.node != nil {
				ignoreFuseNotifyError(server.InvalidateNodeData(change.node))
			}
		}
	}
	for dir := range changedDirs {
		ignoreFuseNotifyError(server.InvalidateNodeData(dir))
	}
}

func ignoreFuseNotifyError(err error) {
	if err == nil || errors.Is(err, fuse.ErrNotCached) || errors.Is(err, fuse.ENOSYS) || errors.Is(err, fuse.ENOENT) {
		return
	}
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

func (d *RemoteDir) Getattr(ctx context.Context, req *fuse.GetattrRequest, resp *fuse.GetattrResponse) error {
	if err := d.Attr(ctx, &resp.Attr); err != nil {
		return err
	}
	resp.Attr.Valid = d.fsys.cacheTTL()
	return nil
}

func (d *RemoteDir) Setattr(ctx context.Context, req *fuse.SetattrRequest, resp *fuse.SetattrResponse) error {
	if d.fsys.readOnly {
		return fuse.Errno(syscall.EROFS)
	}

	// Handle mode changes (e.g., chmod)
	if req.Valid.Mode() {
		d.mode = req.Mode
	}

	// Handle uid/gid changes (chown)
	if req.Valid.Uid() {
		d.uid = req.Uid
	}
	if req.Valid.Gid() {
		d.gid = req.Gid
	}

	if err := d.Attr(ctx, &resp.Attr); err != nil {
		return err
	}
	return nil
}

func (d *RemoteDir) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) error {
	// Directories can always be opened for reading
	if req.Flags&fuse.OpenTruncate != 0 {
		return fuse.Errno(syscall.EISDIR)
	}
	if req.Flags.IsWriteOnly() || req.Flags.IsReadWrite() {
		if d.fsys.readOnly {
			return fuse.Errno(syscall.EROFS)
		}
	}
	return nil
}

func (d *RemoteDir) Attr(ctx context.Context, a *fuse.Attr) error {
	mode := d.mode
	if mode == 0 {
		if d.fsys.readOnly {
			mode = os.ModeDir | 0555
		} else {
			mode = os.ModeDir | 0755
		}
	}
	a.Mode = mode
	a.Uid = d.uid
	a.Gid = d.gid
	return nil
}

func (d *RemoteDir) Lookup(ctx context.Context, req *fuse.LookupRequest, resp *fuse.LookupResponse) (fs.Node, error) {
	if err := d.fsys.refreshTreeIfNeeded(); err != nil {
		return nil, err
	}
	d.refreshEntriesFromRoot()

	d.mu.Lock()
	node, ok := d.entries[req.Name]
	d.mu.Unlock()
	if !ok {
		return nil, fuse.ENOENT
	}
	resp.EntryValid = d.fsys.cacheTTL()
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
		mode: 0644,
		uid:  d.fsys.defaultUid,
		gid:  d.fsys.defaultGid,
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
		mode:    os.ModeDir | 0755,
		uid:     d.fsys.defaultUid,
		gid:     d.fsys.defaultGid,
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
	mode := f.mode
	if mode == 0 {
		if f.fsys.readOnly {
			mode = 0444
		} else {
			mode = 0644
		}
	}
	a.Mode = mode
	a.Size = f.size
	a.Mtime = f.mtime
	a.Uid = f.uid
	a.Gid = f.gid
	return nil
}

func (f *RemoteFile) Getattr(ctx context.Context, req *fuse.GetattrRequest, resp *fuse.GetattrResponse) error {
	if err := f.Attr(ctx, &resp.Attr); err != nil {
		return err
	}
	resp.Attr.Valid = f.fsys.cacheTTL()
	return nil
}

func (f *RemoteFile) Setattr(ctx context.Context, req *fuse.SetattrRequest, resp *fuse.SetattrResponse) error {
	if f.fsys.readOnly {
		return fuse.Errno(syscall.EROFS)
	}

	changed := false

	// Handle size changes (truncate)
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
		changed = true
	}

	// Handle mode changes (e.g., chmod, chmod +x)
	if req.Valid.Mode() {
		f.mode = req.Mode
		changed = true
	}

	// Handle uid/gid changes (chown)
	if req.Valid.Uid() {
		f.uid = req.Uid
		changed = true
	}
	if req.Valid.Gid() {
		f.gid = req.Gid
		changed = true
	}

	if err := f.Attr(ctx, &resp.Attr); err != nil {
		return err
	}
	_ = changed // metadata changes are tracked locally; future enhancement could sync to server
	return nil
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
	mountCmd.Flags().BoolVar(&mountReadOnly, "readonly", false, "Mount Drop read-only")
	mountCmd.Flags().DurationVar(&mountRefreshInterval, "refresh-interval", 500*time.Millisecond, "How often the mounted Drop checks for remote changes")
	rootCmd.AddCommand(mountCmd)
}

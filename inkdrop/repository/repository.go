package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"inkdrop/config"
	userModel "inkdrop/model"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
)

var ErrItemExists = errors.New("item already exists")

const (
	TrashDisplayName  = "Trash"
	TrashInternalRoot = ".Trash-1000"
	TrashFilesPath    = ".Trash-1000/files"
	TrashInfoPath     = ".Trash-1000/info"
)

var (
	GlobalInkDropRootDir    = "/srv/ftr"
	GlobalInkDropRootDirMac = "/srv/ftr"

	RootDir     string
	DropDir     string
	DropMetaDir string
	TempDir     string
	SessionDir  string
	UserPFPDir  string
)

func init() {
	rootDir := strings.TrimSpace(os.Getenv("FTR_ROOT_DIR"))
	defaultRootDir := GlobalInkDropRootDir
	if runtime.GOOS == "darwin" {
		defaultRootDir = GlobalInkDropRootDirMac
	}
	if rootDir == "" {
		rootDir = defaultRootDir
	}

	RootDir = filepath.Clean(rootDir)
	DropDir = filepath.Join(RootDir, "userRepositories")
	DropMetaDir = filepath.Join(DropDir, "_meta")
	TempDir = filepath.Join(RootDir, "tmp")
	SessionDir = filepath.Join(RootDir, "sessions")
	userPFPDir := strings.TrimSpace(os.Getenv("FTR_PFP_DIR"))
	if userPFPDir == "" {
		UserPFPDir = "/ftr/userData"
	} else {
		UserPFPDir = filepath.Clean(userPFPDir)
	}
}

func EnsureStorageLayout() error {
	for _, dir := range []string{DropDir, DropMetaDir, TempDir, SessionDir, UserPFPDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return nil
}

func MigrateLegacyPFPs() error {
	db := config.GetDB()
	if db == nil {
		return errors.New("database not initialized")
	}
	ctx := context.Background()
	var users []userModel.User
	if err := db.NewSelect().Model(&users).Column("id", "name", "pfp").Scan(ctx); err != nil {
		return err
	}
	for _, user := range users {
		oldPFP := strings.TrimSpace(user.PFP)
		resolved := userModel.ResolveProfilePicture(oldPFP, user.Name)
		if err := migrateLegacyPFPFile(oldPFP, resolved); err != nil {
			return err
		}
		if resolved != oldPFP {
			if _, err := db.NewUpdate().Model((*userModel.User)(nil)).Set("pfp = ?", resolved).Where("id = ?", user.ID).Exec(ctx); err != nil {
				return err
			}
		}
	}
	return nil
}

func migrateLegacyPFPFile(oldPFP string, resolvedPFP string) error {
	if resolvedPFP == "/pfp/default.png" {
		return nil
	}
	targetRel := strings.TrimPrefix(resolvedPFP, "/pfp/")
	targetPath := filepath.Join(UserPFPDir, filepath.FromSlash(targetRel))
	if _, err := os.Stat(targetPath); err == nil {
		return nil
	}

	candidates := []string{}
	if oldPFP == "" {
		return nil
	}
	if filepath.IsAbs(oldPFP) {
		candidates = append(candidates, oldPFP)
	}
	if strings.HasPrefix(oldPFP, "/pfp/") {
		rel := strings.TrimPrefix(oldPFP, "/pfp/")
		candidates = append(candidates,
			filepath.Join(UserPFPDir, filepath.FromSlash(rel)),
			filepath.Join(".", filepath.FromSlash(rel)),
		)
	}
	if strings.HasPrefix(oldPFP, "/ftr/userpfp/") || strings.HasPrefix(oldPFP, "/ftr/userData/") {
		candidates = append(candidates, oldPFP)
	}
	if !filepath.IsAbs(oldPFP) && !strings.HasPrefix(oldPFP, "/pfp/") {
		candidates = append(candidates,
			filepath.Join(UserPFPDir, filepath.FromSlash(oldPFP)),
			filepath.Join(".", filepath.FromSlash(oldPFP)),
		)
	}

	for _, src := range candidates {
		if src == "" {
			continue
		}
		if _, err := os.Stat(src); err == nil {
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return err
			}
			in, err := os.Open(src)
			if err != nil {
				return err
			}
			defer in.Close()
			out, err := os.Create(targetPath)
			if err != nil {
				return err
			}
			defer out.Close()
			if _, err := io.Copy(out, in); err != nil {
				return err
			}
			return nil
		}
	}
	return nil
}

func DirExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if err == nil {
		return info.IsDir(), nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func ProcessDropName(raw string) (string, error) {
	rawWithoutSpaces := strings.ReplaceAll(raw, " ", "_")
	pass, err := regexp.MatchString("^[a-zA-Z0-9_-]+$", rawWithoutSpaces)
	if err != nil {
		return "", err
	}
	if pass != true {
		return "", errors.New("drop name contains special characters")
	}
	return rawWithoutSpaces, nil
}

func CreateNewUserDrop(username string, dropname string) error {
	var userDropDir string = fmt.Sprintf("%s/%s/%s", DropDir, username, dropname)
	var userDropMetaDir string = fmt.Sprintf("%s/%s/%s", DropMetaDir, username, dropname)
	exists1, err1 := DirExists(userDropDir)
	exists2, err2 := DirExists(userDropMetaDir)
	if err1 != nil || err2 != nil {
		return fmt.Errorf("%s\n%s", err1, err2)
	}
	if exists1 || exists2 {
		return errors.New("a path for this Drop already exists. Consider renaming the Drop")
	}
	err1 = os.MkdirAll(userDropDir, 0755)
	err2 = os.MkdirAll(userDropMetaDir, 0755)
	if err1 != nil || err2 != nil {
		return fmt.Errorf("%s\n%s", err1, err2)
	}
	meta := &RepoMeta{
		Owners:      []string{username},
		Description: "",
		Public:      false,
	}
	if err := SaveRepoMeta(username, dropname, meta); err != nil {
		return fmt.Errorf("drop directory created but meta init failed: %w", err)
	}
	return nil
}

func CreateNewDirectory(userName, dropName, workingDir, folderName string) error {
	root := RepositoryRoot(userName, dropName)
	target, err := resolvePathInRepo(root, workingDir, folderName)
	if err != nil {
		return err
	}
	err = os.MkdirAll(target, 0755)
	if err != nil {
		return err
	}
	return nil
}

func ListUserDrops(userName string) ([]string, error) {
	var userDropDir string = fmt.Sprintf("%s/%s", DropDir, userName)
	entries, err := os.ReadDir(userDropDir)
	var clean []string
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			clean = append(clean, entry.Name())
		}
	}
	return clean, nil
}

func SearchDrops(query string, currentUser string) ([]map[string]string, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, nil
	}
	all := q == "/" || q == "*"
	q = strings.ToLower(q)
	searchUser := ""
	searchDrop := ""
	if strings.Contains(q, "/") {
		parts := strings.SplitN(q, "/", 2)
		searchUser = strings.TrimSpace(parts[0])
		searchDrop = strings.TrimSpace(parts[1])
		searchDrop = strings.TrimSuffix(searchDrop, "/")
	}
	var results []map[string]string
	users, err := os.ReadDir(DropDir)
	if err != nil {
		return nil, err
	}
	for _, userDir := range users {
		if !userDir.IsDir() {
			continue
		}
		userName := userDir.Name()
		if strings.HasPrefix(userName, "_") {
			continue
		}
		dropDirPath := filepath.Join(DropDir, userName)
		drops, err := os.ReadDir(dropDirPath)
		if err != nil {
			continue
		}
		for _, dropDirEntry := range drops {
			if !dropDirEntry.IsDir() {
				continue
			}
			if strings.HasPrefix(dropDirEntry.Name(), "_") {
				continue
			}
			dropName := dropDirEntry.Name()
			meta, err := LoadRepoMeta(userName, dropName)
			if err != nil {
				continue
			}
			isPublic := meta != nil && meta.Public
			isOwnDrop := currentUser != "" && currentUser == userName
			isShared := false
			if currentUser != "" && meta != nil {
				for _, o := range meta.Owners {
					if o == currentUser {
						isShared = true
						break
					}
				}
			}
			if !isPublic && !isOwnDrop && !isShared {
				continue
			}
			desc := ""
			if meta != nil {
				desc = meta.Description
			}
			matches := all || strings.Contains(strings.ToLower(userName), q) || strings.Contains(strings.ToLower(dropName), q) || strings.Contains(strings.ToLower(desc), q)
			if searchUser != "" {
				if searchDrop != "" {
					matches = strings.Contains(strings.ToLower(userName), searchUser) && strings.Contains(strings.ToLower(dropName), searchDrop)
				} else {
					matches = strings.Contains(strings.ToLower(userName), searchUser)
				}
			}
			if matches {
				results = append(results, map[string]string{"user": userName, "repo": dropName, "description": desc})
			}
		}
	}
	return results, nil
}

func ListPublicDrops() ([]map[string]string, error) {
	var results []map[string]string
	users, err := os.ReadDir(DropDir)
	if err != nil {
		return nil, err
	}
	for _, u := range users {
		if !u.IsDir() {
			continue
		}
		userName := u.Name()
		if strings.HasPrefix(userName, "_") {
			continue
		}
		drops, err := os.ReadDir(filepath.Join(DropDir, userName))
		if err != nil {
			continue
		}
		for _, re := range drops {
			if !re.IsDir() {
				continue
			}
			dropName := re.Name()
			if strings.HasPrefix(dropName, "_") {
				continue
			}
			meta, err := LoadRepoMeta(userName, dropName)
			if err != nil {
				continue
			}
			if meta != nil && meta.Public {
				results = append(results, map[string]string{"user": userName, "repo": dropName, "description": meta.Description})
			}
		}
	}
	return results, nil
}

func ListSharedDrops(username string) ([]map[string]string, error) {
	var results []map[string]string
	seen := map[string]bool{}
	users, err := os.ReadDir(DropDir)
	if err != nil {
		return nil, err
	}
	for _, u := range users {
		if !u.IsDir() {
			continue
		}
		userName := u.Name()
		if strings.HasPrefix(userName, "_") {
			continue
		}
		drops, err := os.ReadDir(filepath.Join(DropDir, userName))
		if err != nil {
			continue
		}
		for _, re := range drops {
			if !re.IsDir() {
				continue
			}
			dropName := re.Name()
			if strings.HasPrefix(dropName, "_") {
				continue
			}
			key := userName + "/" + dropName
			if seen[key] {
				continue
			}
			meta, err := LoadRepoMeta(userName, dropName)
			if err != nil || meta == nil {
				continue
			}
			// Only show drops where the user is explicitly listed as an owner
			// or the drop is public. Contact relationship alone is insufficient.
			isOwner := false
			for _, o := range meta.Owners {
				if o == username {
					isOwner = true
					break
				}
			}
			if isOwner || meta.Public {
				results = append(results, map[string]string{"user": userName, "repo": dropName, "description": meta.Description})
				seen[key] = true
			}
		}
	}
	return results, nil
}

func ListDropFilesRecursive(userName, dropName string) ([]map[string]interface{}, error) {
	root := RepositoryRoot(userName, dropName)
	if ok, err := DirExists(root); err != nil || !ok {
		return nil, fmt.Errorf("drop %s/%s not found", userName, dropName)
	}
	var files []map[string]interface{}
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		files = append(files, map[string]interface{}{"path": filepath.ToSlash(rel), "size": info.Size(), "modified": info.ModTime().Unix()})
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	return files, nil
}

func ListDropEntriesRecursive(userName, dropName string) ([]map[string]interface{}, error) {
	root := RepositoryRoot(userName, dropName)
	if ok, err := DirExists(root); err != nil || !ok {
		return nil, fmt.Errorf("drop %s/%s not found", userName, dropName)
	}
	var entries []map[string]interface{}
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if path == root {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		kind := "file"
		if d.IsDir() {
			kind = "dir"
		}
		entry := map[string]interface{}{
			"path":     filepath.ToSlash(rel),
			"type":     kind,
			"size":     info.Size(),
			"modified": info.ModTime().Unix(),
		}
		if !d.IsDir() {
			if hash, err := CalculateFileHash(path); err == nil {
				entry["hash"] = hash
			}
		}
		entries = append(entries, entry)
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	return entries, nil
}

// RepositoryRoot returns the absolute filesystem path for a drop.
func RepositoryRoot(userName, dropName string) string {
	return filepath.Clean(filepath.Join(DropDir, userName, dropName))
}

// StatDropEntry returns metadata for a single file/directory within a drop.
// Returns nil without error if the path does not exist.
func StatDropEntry(userName, dropName, itemPath string) (map[string]interface{}, error) {
	root := RepositoryRoot(userName, dropName)
	fullPath, err := GetItemPath(userName, dropName, "/", itemPath)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	rel, err := filepath.Rel(root, fullPath)
	if err != nil {
		return nil, err
	}
	kind := "file"
	if info.IsDir() {
		kind = "dir"
	}
	entry := map[string]interface{}{
		"path":     filepath.ToSlash(rel),
		"type":     kind,
		"size":     info.Size(),
		"modified": info.ModTime().Unix(),
		"name":     info.Name(),
	}
	if !info.IsDir() {
		if hash, err := CalculateFileHash(fullPath); err == nil {
			entry["hash"] = hash
		}
	}
	return entry, nil
}

func CreateDirectoryAtDropPath(userName, dropName, dropPath string) (string, error) {
	rawPath := strings.TrimSpace(dropPath)
	if rawPath == "" || rawPath == "/" {
		return "", errors.New("invalid directory path")
	}
	cleanDropPath := normalizeWorkingDir(rawPath)
	relativePath := strings.TrimPrefix(cleanDropPath, "/")
	if relativePath == "" {
		return "", errors.New("invalid directory path")
	}
	targetPath, err := GetItemPath(userName, dropName, "/", relativePath)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(targetPath, 0755); err != nil {
		return "", err
	}
	return targetPath, nil
}

func CalculateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func GetDirectoryListing(userName string, dropName string, path string) ([]string, error) {
	root := RepositoryRoot(userName, dropName)
	directoryToList, err := resolvePathInRepo(root, path, ".")
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(directoryToList)
	if err != nil {
		return nil, err
	}
	var clean []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			name += "/"
		}
		clean = append(clean, name)
	}
	sort.Strings(clean)
	return clean, nil
}

func RenameItem(userName, dropName, workingDir, oldName, newName string) error {
	root := RepositoryRoot(userName, dropName)
	oldPath, err := resolvePathInRepo(root, workingDir, oldName)
	if err != nil {
		return err
	}
	newPath, err := resolvePathInRepo(root, workingDir, newName)
	if err != nil {
		return err
	}
	if oldPath == root || newPath == root {
		return errors.New("invalid path")
	}
	if _, err := os.Stat(oldPath); err != nil {
		return err
	}
	if _, err := os.Stat(newPath); err == nil {
		return nil
	}
	err = os.MkdirAll(filepath.Dir(newPath), 0755)
	if err != nil {
		return err
	}
	err = os.Rename(oldPath, newPath)
	if err != nil {
		return err
	}
	return nil
}

func DeleteItem(userName, dropName, workingDir, name string) error {
	if IsProtectedTrashTarget(workingDir, name) {
		return errors.New("trash cannot be deleted; empty it instead")
	}
	root := RepositoryRoot(userName, dropName)
	target, err := resolvePathInRepo(root, workingDir, name)
	if err != nil {
		return err
	}
	if target == root {
		return errors.New("cannot delete drop root")
	}
	if repositoryPathContainsTrash(root, target) {
		return os.RemoveAll(target)
	}
	if err := EnsureTrash(userName, dropName); err != nil {
		return err
	}
	trashPath, err := nextAvailableTrashPath(root, filepath.Base(target))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(trashPath), 0755); err != nil {
		return err
	}
	if err := os.Rename(target, trashPath); err != nil {
		return err
	}
	infoPath := filepath.Join(root, filepath.FromSlash(TrashInfoPath), filepath.Base(trashPath)+".trashinfo")
	originalPath := normalizeWorkingDir(filepath.ToSlash(filepath.Join(workingDir, name)))
	originalPath = strings.TrimPrefix(originalPath, "/")
	trashInfo := fmt.Sprintf("Path=%s\n", originalPath)
	if err := os.WriteFile(infoPath, []byte(trashInfo), 0644); err != nil {
		return err
	}
	return nil
}

func repositoryPathContainsTrash(root, target string) bool {
	trashDir := filepath.Join(root, filepath.FromSlash(TrashFilesPath))
	return isWithinRoot(trashDir, target)
}

func nextAvailableTrashPath(root, baseName string) (string, error) {
	trashRoot := filepath.Join(root, filepath.FromSlash(TrashFilesPath))
	target := filepath.Join(trashRoot, baseName)
	if _, err := os.Stat(target); errors.Is(err, os.ErrNotExist) {
		return target, nil
	}
	ext := filepath.Ext(target)
	base := strings.TrimSuffix(target, ext)
	for index := 1; index < 1000; index++ {
		candidate := fmt.Sprintf("%s (%d)%s", base, index, ext)
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate, nil
		}
	}
	return "", errors.New("could not find an available trash path")
}

func IsTrashBrowserPath(rawPath string) bool {
	clean := normalizeWorkingDir(rawPath)
	return clean == "/"+TrashDisplayName || strings.HasPrefix(clean, "/"+TrashDisplayName+"/")
}

func MapTrashBrowserPath(rawPath string) string {
	clean := normalizeWorkingDir(rawPath)
	if clean == "/"+TrashDisplayName {
		return "/" + TrashFilesPath
	}
	if strings.HasPrefix(clean, "/"+TrashDisplayName+"/") {
		suffix := strings.TrimPrefix(clean, "/"+TrashDisplayName+"/")
		return "/" + strings.TrimSuffix(TrashFilesPath, "/") + "/" + suffix
	}
	return clean
}

func IsTrashStoragePath(rawPath string) bool {
	clean := normalizeWorkingDir(rawPath)
	return clean == "/"+TrashInternalRoot || strings.HasPrefix(clean, "/"+TrashInternalRoot+"/")
}

func IsProtectedTrashTarget(workingDir, name string) bool {
	target := normalizeWorkingDir(filepath.ToSlash(filepath.Join(workingDir, name)))
	return target == "/"+TrashDisplayName || target == "/"+TrashInternalRoot
}

func HasTrash(userName, dropName string) bool {
	filesDir := filepath.Join(RepositoryRoot(userName, dropName), filepath.FromSlash(TrashFilesPath))
	ok, err := DirExists(filesDir)
	return err == nil && ok
}

func EnsureTrash(userName, dropName string) error {
	root := RepositoryRoot(userName, dropName)
	if ok, err := DirExists(root); err != nil || !ok {
		return fmt.Errorf("drop %s/%s not found", userName, dropName)
	}
	if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(TrashFilesPath)), 0755); err != nil {
		return err
	}
	return os.MkdirAll(filepath.Join(root, filepath.FromSlash(TrashInfoPath)), 0755)
}

func EmptyTrash(userName, dropName string) error {
	root := RepositoryRoot(userName, dropName)
	for _, trashPath := range []string{TrashFilesPath, TrashInfoPath} {
		target := filepath.Join(root, filepath.FromSlash(trashPath))
		if !isWithinRoot(root, target) {
			return errors.New("trash path escapes drop root")
		}
		if err := os.RemoveAll(target); err != nil {
			return err
		}
	}
	return EnsureTrash(userName, dropName)
}

func RestoreTrashItem(userName, dropName, itemName string) (string, error) {
	cleanName := filepath.ToSlash(strings.TrimSpace(itemName))
	cleanName = strings.Trim(cleanName, "/")
	if cleanName == "" || cleanName == "." || cleanName == ".." || strings.Contains(cleanName, "../") {
		return "", errors.New("invalid trash item")
	}
	root := RepositoryRoot(userName, dropName)
	source, err := resolvePathInRepo(root, "/"+TrashFilesPath, cleanName)
	if err != nil {
		return "", err
	}
	if !isWithinRoot(filepath.Join(root, filepath.FromSlash(TrashFilesPath)), source) {
		return "", errors.New("invalid trash item")
	}
	if _, err := os.Stat(source); err != nil {
		return "", err
	}
	originalPath := readTrashOriginalPath(root, cleanName)
	if originalPath == "" || IsTrashStoragePath(originalPath) {
		originalPath = "/" + filepath.Base(cleanName)
	}
	dest, err := nextAvailableRestorePath(root, originalPath)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return "", err
	}
	if err := os.Rename(source, dest); err != nil {
		return "", err
	}
	_ = os.Remove(filepath.Join(root, filepath.FromSlash(TrashInfoPath), filepath.Base(cleanName)+".trashinfo"))
	_ = pruneEmptyDirs(filepath.Dir(source), filepath.Join(root, filepath.FromSlash(TrashFilesPath)))
	return filepath.ToSlash(strings.TrimPrefix(dest, root+string(filepath.Separator))), nil
}

func readTrashOriginalPath(root, itemName string) string {
	infoPath := filepath.Join(root, filepath.FromSlash(TrashInfoPath), filepath.Base(itemName)+".trashinfo")
	data, err := os.ReadFile(infoPath)
	if err != nil {
		return ""
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Path=") {
			value := strings.TrimSpace(strings.TrimPrefix(line, "Path="))
			value = strings.TrimPrefix(filepath.ToSlash(value), "/")
			if value == "" {
				return ""
			}
			parts := strings.Split(value, "/")
			for index := 0; index < len(parts)-1; index++ {
				if parts[index] == filepath.Base(filepath.Dir(root)) && parts[index+1] == filepath.Base(root) {
					return "/" + strings.Join(parts[index+2:], "/")
				}
			}
			return "/" + value
		}
	}
	return ""
}

func nextAvailableRestorePath(root, originalPath string) (string, error) {
	cleanOriginal := strings.TrimPrefix(normalizeWorkingDir(originalPath), "/")
	if cleanOriginal == "" {
		cleanOriginal = "Restored Item"
	}
	target, err := resolvePathInRepo(root, "/", cleanOriginal)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(target); errors.Is(err, os.ErrNotExist) {
		return target, nil
	}
	ext := filepath.Ext(target)
	base := strings.TrimSuffix(target, ext)
	for index := 1; index < 1000; index++ {
		candidate := fmt.Sprintf("%s (restored %d)%s", base, index, ext)
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate, nil
		}
	}
	return "", errors.New("could not find an available restore path")
}

func pruneEmptyDirs(start, stop string) error {
	start = filepath.Clean(start)
	stop = filepath.Clean(stop)
	for start != stop && isWithinRoot(stop, start) {
		if err := os.Remove(start); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return nil
		}
		start = filepath.Dir(start)
	}
	return nil
}

// repositoryRoot is a private wrapper kept for internal backward compatibility.
// New code should use RepositoryRoot instead.
func repositoryRoot(userName, dropName string) string {
	return RepositoryRoot(userName, dropName)
}

func normalizeWorkingDir(path string) string {
	if path == "" {
		return "/"
	}
	p := filepath.ToSlash(filepath.Clean("/" + strings.TrimSpace(path)))
	if p == "." || p == "" {
		return "/"
	}
	return p
}

func resolvePathInRepo(root, workingDir, name string) (string, error) {
	rootClean := filepath.Clean(root)
	wd := normalizeWorkingDir(workingDir)
	workingAbs := filepath.Clean(filepath.Join(rootClean, strings.TrimPrefix(wd, "/")))
	if !isWithinRoot(rootClean, workingAbs) {
		return "", errors.New("working directory is outside drop root")
	}
	if name == "" || name == "." {
		return workingAbs, nil
	}
	name = filepath.ToSlash(strings.TrimSpace(name))
	if name == "." || name == ".." || strings.HasPrefix(name, "/") {
		return "", errors.New("invalid target path")
	}
	target := filepath.Clean(filepath.Join(workingAbs, name))
	if !isWithinRoot(rootClean, target) {
		return "", errors.New("target path is outside drop root")
	}
	return target, nil
}

func GetItemPath(userName, dropName, workingDir, itemName string) (string, error) {
	if itemName == "" || itemName == "." || itemName == ".." {
		return "", errors.New("invalid file name")
	}
	root := RepositoryRoot(userName, dropName)
	return resolvePathInRepo(root, workingDir, itemName)
}

func WriteTextFile(userName, dropName, workingDir, fileName string, data []byte) (string, error) {
	return WriteFile(userName, dropName, workingDir, fileName, data)
}

func WriteFile(userName, dropName, workingDir, fileName string, data []byte) (string, error) {
	targetPath, err := GetItemPath(userName, dropName, workingDir, fileName)
	if err != nil {
		return "", err
	}
	if err := writeFileAtomic(targetPath, data, true); err != nil {
		return "", err
	}
	return targetPath, nil
}

func WriteTextFileAtDropPath(userName, dropName, dropPath string, data []byte, overwrite bool) (string, error) {
	return WriteFileAtDropPath(userName, dropName, dropPath, data, overwrite)
}

func WriteFileAtDropPath(userName, dropName, dropPath string, data []byte, overwrite bool) (string, error) {
	rawPath := strings.TrimSpace(dropPath)
	if rawPath == "" || strings.HasSuffix(rawPath, "/") {
		return "", errors.New("invalid file path")
	}
	cleanDropPath := normalizeWorkingDir(rawPath)
	relativePath := strings.TrimPrefix(cleanDropPath, "/")
	if relativePath == "" {
		return "", errors.New("invalid file path")
	}
	targetPath, err := GetItemPath(userName, dropName, "/", relativePath)
	if err != nil {
		return "", err
	}
	if err := writeFileAtomic(targetPath, data, overwrite); err != nil {
		return "", err
	}
	return targetPath, nil
}

func writeFileAtomic(targetPath string, data []byte, overwrite bool) error {
	perm := os.FileMode(0644)
	info, err := os.Stat(targetPath)
	if err == nil {
		if info.IsDir() {
			return errors.New("target path is a directory")
		}
		if !overwrite {
			return ErrItemExists
		}
		perm = info.Mode().Perm()
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return err
	}
	tempFile, err := os.CreateTemp(filepath.Dir(targetPath), ".inkdrop-live-*")
	if err != nil {
		return err
	}
	tempPath := tempFile.Name()
	cleanup := func() {
		tempFile.Close()
		os.Remove(tempPath)
	}
	if _, err := tempFile.Write(data); err != nil {
		cleanup()
		return err
	}
	if err := tempFile.Chmod(perm); err != nil {
		cleanup()
		return err
	}
	if err := tempFile.Sync(); err != nil {
		cleanup()
		return err
	}
	if err := tempFile.Close(); err != nil {
		os.Remove(tempPath)
		return err
	}
	if err := os.Rename(tempPath, targetPath); err != nil {
		os.Remove(tempPath)
		return err
	}
	return nil
}

func isWithinRoot(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func MoveUploadedFile(userName, dropName, workingDir, filename, tusFilePath string) error {
	root := RepositoryRoot(userName, dropName)
	if ok, _ := DirExists(root); !ok {
		return fmt.Errorf("drop %s/%s does not exist", userName, dropName)
	}
	destDir, err := resolvePathInRepo(root, workingDir, ".")
	if err != nil {
		return fmt.Errorf("invalid working directory: %w", err)
	}
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}
	destPath := filepath.Join(destDir, filepath.Base(filename))
	if !isWithinRoot(root, destPath) {
		return fmt.Errorf("destination path escapes drop root")
	}
	if err := os.Rename(tusFilePath, destPath); err != nil {
		return fmt.Errorf("failed to move uploaded file: %w", err)
	}
	infoFile := tusFilePath + ".info"
	os.Remove(infoFile)
	return nil
}

func DeleteUserDrop(userName string, dropName string) error {
	var mainDirToRemove string = fmt.Sprintf("%s/%s/%s", DropDir, userName, dropName)
	var metaDirToRemove string = fmt.Sprintf("%s/%s/%s", DropMetaDir, userName, dropName)
	err1 := os.RemoveAll(mainDirToRemove)
	err2 := os.RemoveAll(metaDirToRemove)
	if err1 != nil || err2 != nil {
		return fmt.Errorf("%s\n%s", err1, err2)
	}
	return nil
}

func PackSQAR(in, user, out string) (string, error) {
	archiveDir := filepath.Join(TempDir, user)
	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		return "", err
	}
	fullArchivePath := filepath.Join(archiveDir, out+".sqar")
	err := filepath.WalkDir(in, func(path string, d fs.DirEntry, err2 error) error {
		if err2 != nil {
			return nil
		}
		if !d.IsDir() {
			return fmt.Errorf("File found.")
		}
		return nil
	})
	if err == nil {
		return "", fmt.Errorf("No files in drop.")
	}
	_, err = exec.Command("sqar", "pack", "-PCI", in, "-O", fullArchivePath).Output()
	if err != nil {
		return "", err
	}
	_, err = os.Stat(fullArchivePath)
	if err != nil {
		return "", err
	}
	return fullArchivePath, nil
}

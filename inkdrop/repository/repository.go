package repository

import (
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
	// Shared FtR storage root. Override with FTR_ROOT_DIR when needed.
	GlobalInkDropRootDir = "/srv/ftr"

	// macOS fallback uses the same shared layout by default.
	GlobalInkDropRootDirMac = "/srv/ftr"

	// Resolved at startup based on OS.
	RootDir     string
	RepoDir     string
	RepoMetaDir string
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
	RepoDir = filepath.Join(RootDir, "userRepositories")
	// RepoMetaDir stores per-repository metadata (meta.json files)
	RepoMetaDir = filepath.Join(RepoDir, "_meta")
	TempDir = filepath.Join(RootDir, "tmp")
	SessionDir = filepath.Join(RootDir, "sessions")
	UserPFPDir = filepath.Clean(strings.TrimSpace(os.Getenv("FTR_PFP_DIR")))
	if UserPFPDir == "" {
		UserPFPDir = "/ftr/userpfp"
	}
}

func EnsureStorageLayout() error {
	for _, dir := range []string{RepoDir, RepoMetaDir, TempDir, SessionDir, UserPFPDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
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

func ProcessRepoName(raw string) (string, error) {
	rawWithoutSpaces := strings.ReplaceAll(raw, " ", "_")
	pass, err := regexp.MatchString("^[a-zA-Z0-9_-]+$", rawWithoutSpaces)
	if err != nil {
		return "", err
	}
	if pass != true {
		return "", errors.New("repository name contains special characters")
	}
	return rawWithoutSpaces, nil
}

func CreateNewUserRepository(username string, reponame string) error {
	var userRepoDir string = fmt.Sprintf("%s/%s/%s", RepoDir, username, reponame)
	var userRepoMetaDir string = fmt.Sprintf("%s/%s/%s", RepoMetaDir, username, reponame)
	exists1, err1 := DirExists(userRepoDir)
	exists2, err2 := DirExists(userRepoMetaDir)
	if err1 != nil || err2 != nil {
		return fmt.Errorf("%s\n%s", err1, err2)
	}
	if exists1 || exists2 {
		return errors.New("a path for this repository already exists. Consider renaming the repository")
	}
	err1 = os.MkdirAll(userRepoDir, 0755)
	err2 = os.MkdirAll(userRepoMetaDir, 0755)
	if err1 != nil || err2 != nil {
		return fmt.Errorf("%s\n%s", err1, err2)
	}
	return nil
}

func CreateNewDirectory(userName, repoName, workingDir, folderName string) error {
	root := repositoryRoot(userName, repoName)
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

func ListUserRepositories(userName string) ([]string, error) {
	var userRepoDir string = fmt.Sprintf("%s/%s", RepoDir, userName)
	entries, err := os.ReadDir(userRepoDir)
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

func SearchRepositories(query string, currentUser string) ([]map[string]string, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, nil
	}

	all := q == "/" || q == "*"
	q = strings.ToLower(q)
	searchUser := ""
	searchRepo := ""
	if strings.Contains(q, "/") {
		parts := strings.SplitN(q, "/", 2)
		searchUser = strings.TrimSpace(parts[0])
		searchRepo = strings.TrimSpace(parts[1])
		searchRepo = strings.TrimSuffix(searchRepo, "/")
	}

	var results []map[string]string

	users, err := os.ReadDir(RepoDir)
	if err != nil {
		return nil, err
	}

	for _, userDir := range users {
		if !userDir.IsDir() {
			continue
		}
		userName := userDir.Name()
		if strings.HasPrefix(userName, "_") {
			// skip internal/system dirs such as _meta
			continue
		}
		repoDir := filepath.Join(RepoDir, userName)
		repos, err := os.ReadDir(repoDir)
		if err != nil {
			continue
		}
		for _, repoDirEntry := range repos {
			if !repoDirEntry.IsDir() {
				continue
			}
			// skip internal dirs
			if strings.HasPrefix(repoDirEntry.Name(), "_") {
				continue
			}
			repoName := repoDirEntry.Name()
			meta, err := LoadRepoMeta(userName, repoName)
			if err != nil {
				continue
			}

			isPublic := meta == nil || meta.Public
			isOwnRepo := currentUser != "" && currentUser == userName
			isShared := false
			if currentUser != "" && meta != nil {
				for _, o := range meta.Owners {
					if o == currentUser {
						isShared = true
						break
					}
				}
			}
			if !isPublic && !isOwnRepo && !isShared {
				continue
			}

			desc := ""
			if meta != nil {
				desc = meta.Description
			}

			matches := all || strings.Contains(strings.ToLower(userName), q) || strings.Contains(strings.ToLower(repoName), q) || strings.Contains(strings.ToLower(desc), q)
			if searchUser != "" {
				if searchRepo != "" {
					matches = strings.Contains(strings.ToLower(userName), searchUser) && strings.Contains(strings.ToLower(repoName), searchRepo)
				} else {
					matches = strings.Contains(strings.ToLower(userName), searchUser)
				}
			}
			if matches {
				results = append(results, map[string]string{"user": userName, "repo": repoName, "description": desc})
			}
		}
	}

	return results, nil
}

// ListPublicRepositories returns a list of repositories that have their
// metadata Public flag set. Each item contains 'user', 'repo' and 'description'.
func ListPublicRepositories() ([]map[string]string, error) {
	var results []map[string]string

	users, err := os.ReadDir(RepoDir)
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
		repos, err := os.ReadDir(filepath.Join(RepoDir, userName))
		if err != nil {
			continue
		}
		for _, re := range repos {
			if !re.IsDir() {
				continue
			}
			repoName := re.Name()
			if strings.HasPrefix(repoName, "_") {
				continue
			}
			meta, err := LoadRepoMeta(userName, repoName)
			if err != nil {
				continue
			}
			if meta != nil {
				if meta.Public {
					results = append(results, map[string]string{"user": userName, "repo": repoName, "description": meta.Description})
				}
			} else {
				results = append(results, map[string]string{"user": userName, "repo": repoName, "description": ""})
			}
		}
	}

	return results, nil
}

// ListSharedRepositories returns repositories where the given username is
// listed as an owner in the repo metadata. Returns items with 'user', 'repo', 'description'.
func ListSharedRepositories(username string) ([]map[string]string, error) {
	var results []map[string]string
	seen := map[string]bool{}
	users, err := os.ReadDir(RepoDir)
	if err != nil {
		return nil, err
	}
	contactOwners, _ := userModel.ListAcceptedContactOwners(config.GetDB(), username)
	contactOwnerSet := map[string]bool{}
	for _, owner := range contactOwners {
		contactOwnerSet[owner] = true
	}

	for _, u := range users {
		if !u.IsDir() {
			continue
		}
		userName := u.Name()
		if strings.HasPrefix(userName, "_") {
			continue
		}
		repos, err := os.ReadDir(filepath.Join(RepoDir, userName))
		if err != nil {
			continue
		}
		for _, re := range repos {
			if !re.IsDir() {
				continue
			}
			repoName := re.Name()
			if strings.HasPrefix(repoName, "_") {
				continue
			}
			key := userName + "/" + repoName
			if contactOwnerSet[userName] {
				desc := ""
				if meta, err := LoadRepoMeta(userName, repoName); err == nil && meta != nil {
					desc = meta.Description
				}
				results = append(results, map[string]string{"user": userName, "repo": repoName, "description": desc})
				seen[key] = true
				continue
			}
			if meta, err := LoadRepoMeta(userName, repoName); err == nil && meta != nil {
				for _, o := range meta.Owners {
					if o == username {
						if seen[key] {
							break
						}
						results = append(results, map[string]string{"user": userName, "repo": repoName, "description": meta.Description})
						seen[key] = true
						break
					}
				}
			}
		}
	}

	return results, nil
}

func ListRepositoryFilesRecursive(userName, repoName string) ([]map[string]interface{}, error) {
	root := repositoryRoot(userName, repoName)
	if ok, err := DirExists(root); err != nil || !ok {
		return nil, fmt.Errorf("repository %s/%s not found", userName, repoName)
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

func ListRepositoryEntriesRecursive(userName, repoName string) ([]map[string]interface{}, error) {
	root := repositoryRoot(userName, repoName)
	if ok, err := DirExists(root); err != nil || !ok {
		return nil, fmt.Errorf("repository %s/%s not found", userName, repoName)
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

func CreateDirectoryAtRepoPath(userName, repoName, repoPath string) (string, error) {
	rawPath := strings.TrimSpace(repoPath)
	if rawPath == "" || rawPath == "/" {
		return "", errors.New("invalid directory path")
	}

	cleanRepoPath := normalizeWorkingDir(rawPath)
	relativePath := strings.TrimPrefix(cleanRepoPath, "/")
	if relativePath == "" {
		return "", errors.New("invalid directory path")
	}

	targetPath, err := GetItemPath(userName, repoName, "/", relativePath)
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

func GetDirectoryListing(userName string, repoName string, path string) ([]string, error) {
	root := repositoryRoot(userName, repoName)
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

func RenameItem(userName, repoName, workingDir, oldName, newName string) error {
	root := repositoryRoot(userName, repoName)
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

func DeleteItem(userName, repoName, workingDir, name string) error {
	if IsProtectedTrashTarget(workingDir, name) {
		return errors.New("trash cannot be deleted; empty it instead")
	}
	root := repositoryRoot(userName, repoName)
	target, err := resolvePathInRepo(root, workingDir, name)
	if err != nil {
		return err
	}
	if target == root {
		return errors.New("cannot delete repository root")
	}

	if repositoryPathContainsTrash(root, target) {
		return os.RemoveAll(target)
	}

	if err := EnsureTrash(userName, repoName); err != nil {
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

func HasTrash(userName, repoName string) bool {
	filesDir := filepath.Join(repositoryRoot(userName, repoName), filepath.FromSlash(TrashFilesPath))
	ok, err := DirExists(filesDir)
	return err == nil && ok
}

func EnsureTrash(userName, repoName string) error {
	root := repositoryRoot(userName, repoName)
	if ok, err := DirExists(root); err != nil || !ok {
		return fmt.Errorf("repository %s/%s not found", userName, repoName)
	}
	if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(TrashFilesPath)), 0755); err != nil {
		return err
	}
	return os.MkdirAll(filepath.Join(root, filepath.FromSlash(TrashInfoPath)), 0755)
}

func EmptyTrash(userName, repoName string) error {
	root := repositoryRoot(userName, repoName)
	for _, trashPath := range []string{TrashFilesPath, TrashInfoPath} {
		target := filepath.Join(root, filepath.FromSlash(trashPath))
		if !isWithinRoot(root, target) {
			return errors.New("trash path escapes repository root")
		}
		if err := os.RemoveAll(target); err != nil {
			return err
		}
	}
	return EnsureTrash(userName, repoName)
}

func RestoreTrashItem(userName, repoName, itemName string) (string, error) {
	cleanName := filepath.ToSlash(strings.TrimSpace(itemName))
	cleanName = strings.Trim(cleanName, "/")
	if cleanName == "" || cleanName == "." || cleanName == ".." || strings.Contains(cleanName, "../") {
		return "", errors.New("invalid trash item")
	}

	root := repositoryRoot(userName, repoName)
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

func repositoryRoot(userName, repoName string) string {
	return filepath.Clean(filepath.Join(RepoDir, userName, repoName))
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
		return "", errors.New("working directory is outside repository root")
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
		return "", errors.New("target path is outside repository root")
	}

	return target, nil
}

func GetItemPath(userName, repoName, workingDir, itemName string) (string, error) {
	if itemName == "" || itemName == "." || itemName == ".." {
		return "", errors.New("invalid file name")
	}
	root := repositoryRoot(userName, repoName)
	return resolvePathInRepo(root, workingDir, itemName)
}

func WriteTextFile(userName, repoName, workingDir, fileName string, data []byte) (string, error) {
	return WriteFile(userName, repoName, workingDir, fileName, data)
}

func WriteFile(userName, repoName, workingDir, fileName string, data []byte) (string, error) {
	targetPath, err := GetItemPath(userName, repoName, workingDir, fileName)
	if err != nil {
		return "", err
	}
	if err := writeFileAtomic(targetPath, data, true); err != nil {
		return "", err
	}
	return targetPath, nil
}

func WriteTextFileAtRepoPath(userName, repoName, repoPath string, data []byte, overwrite bool) (string, error) {
	return WriteFileAtRepoPath(userName, repoName, repoPath, data, overwrite)
}

func WriteFileAtRepoPath(userName, repoName, repoPath string, data []byte, overwrite bool) (string, error) {
	rawPath := strings.TrimSpace(repoPath)
	if rawPath == "" || strings.HasSuffix(rawPath, "/") {
		return "", errors.New("invalid file path")
	}

	cleanRepoPath := normalizeWorkingDir(rawPath)
	relativePath := strings.TrimPrefix(cleanRepoPath, "/")
	if relativePath == "" {
		return "", errors.New("invalid file path")
	}

	targetPath, err := GetItemPath(userName, repoName, "/", relativePath)
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

// MoveUploadedFile moves a TUS-uploaded file into the user's repository at the
// given working directory, using the original filename. It validates that the
// destination stays within the repository root and cleans up the TUS .info sidecar.
func MoveUploadedFile(userName, repoName, workingDir, filename, tusFilePath string) error {
	root := repositoryRoot(userName, repoName)

	// Ensure root exists
	if ok, _ := DirExists(root); !ok {
		return fmt.Errorf("repository %s/%s does not exist", userName, repoName)
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
		return fmt.Errorf("destination path escapes repository root")
	}

	// Move (rename) the uploaded blob to the final destination
	if err := os.Rename(tusFilePath, destPath); err != nil {
		return fmt.Errorf("failed to move uploaded file: %w", err)
	}

	// Clean up the TUS .info sidecar file
	infoFile := tusFilePath + ".info"
	os.Remove(infoFile)

	return nil
}

func DeleteUserRepository(userName string, repoName string) error {
	var mainDirToRemove string = fmt.Sprintf("%s/%s/%s", RepoDir, userName, repoName)
	var metaDirToRemove string = fmt.Sprintf("%s/%s/%s", RepoMetaDir, userName, repoName)

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
		return "", fmt.Errorf("No files in repository.")
	}
	_, err = exec.Command("sqar", "pack", "-PCI", in, "-O", fullArchivePath).Output()
	if err != nil {
		return "", err
	}

	// Test if archive exists
	_, err = os.Stat(fullArchivePath)
	if err != nil {
		return "", err
	}

	return fullArchivePath, nil
}

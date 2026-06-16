package repository

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"inkdrop/config"
	userModel "inkdrop/model"
	"inkdrop/repository"
	viewBackend "inkdrop/view/connector"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

func IndexMain(w http.ResponseWriter, r *http.Request) {
	SS := config.GetSessionManager()

	isLoggedIn := SS.GetBool(r.Context(), "isLoggedIn")
	userName := SS.GetString(r.Context(), "name")

	// Support alternate views via ?view=public and ?view=shared
	viewParam := r.URL.Query().Get("view")
	if viewParam == "public" {
		// Public browsing: allow anonymous access
		matches, err := repository.ListPublicDrops()
		p := viewBackend.FrontEndParams{
			Title:           "InkDrop Browser",
			Message:         "Browse public Drops",
			Name:            userName,
			IsViewingPublic: true,
			RepoMatches:     matches,
			Error:           make(map[string]string),
		}
		if err != nil {
			p.Error["general"] = fmt.Sprintf("Failed to get public Drops: %s", err)
		}
		// Friendly name for unauthenticated viewers
		if p.Name == "" {
			p.Name = "Guest"
		}
		enrichAccountParams(&p, userName)
		viewBackend.IndexMain(w, p)
		return
	}
	if viewParam == "shared" {
		// Shared with me: require login
		if isLoggedIn != true || userName == "" {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		matches, err := repository.ListSharedDrops(userName)
		p := viewBackend.FrontEndParams{
			Title:           "InkDrop Browser",
			Message:         "Drops shared with you",
			Name:            userName,
			IsViewingPublic: true,
			RepoMatches:     matches,
			Error:           make(map[string]string),
		}
		if err != nil {
			p.Error["general"] = fmt.Sprintf("Failed to get shared Drops: %s", err)
		}
		enrichAccountParams(&p, userName)
		viewBackend.IndexMain(w, p)
		return
	}

	if isLoggedIn != true || userName == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	dropList, err := repository.ListUserDrops(userName)

	p := viewBackend.FrontEndParams{
		Title:           "InkDrop Browser",
		Message:         "Browse the InkDrop machine",
		Name:            userName,
		IsViewingPublic: false,
		RepoList:        dropList,
		Error:           make(map[string]string),
	}

	if err != nil {
		p.Error["general"] = fmt.Sprintf("Failed to get Drop listing: %s", err)
	}

	enrichAccountParams(&p, userName)
	viewBackend.IndexMain(w, p)
}

func RepositoryListAPI(w http.ResponseWriter, r *http.Request) {
	SS := config.GetSessionManager()
	userName := SS.GetString(r.Context(), "name")
	if SS.GetBool(r.Context(), "isLoggedIn") != true || userName == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{"success": false, "error": "login required"})
		return
	}

	dropList, err := repository.ListUserDrops(userName)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "drops": dropList})
}

func enrichAccountParams(p *viewBackend.FrontEndParams, userName string) {
	if userName == "" || userName == "Guest" {
		return
	}
	db := config.GetDB()
	user, err := userModel.GetUserByName(userName, db)
	if err == nil && user != nil {
		p.UserBio = user.Bio
		p.UserPFP = userModel.ResolveProfilePicture(user.PFP, user.Name)
	}
	contacts, err := userModel.ListMutualContacts(db, userName)
	if err == nil {
		p.AcceptedContacts = contacts
	}
	incoming, outgoing, err := userModel.ListContactRequests(db, userName)
	if err == nil {
		p.PendingIncomingContacts = contactRequestParamMaps(incoming)
		p.PendingOutgoingContacts = contactRequestParamMaps(outgoing)
	}
}

func contactRequestParamMaps(contacts []userModel.ContactRequest) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(contacts))
	for _, contact := range contacts {
		out = append(out, map[string]interface{}{
			"id":        contact.ID,
			"requester": contact.Requester,
			"recipient": contact.Recipient,
			"status":    contact.Status,
		})
	}
	return out
}

func IndexMainPost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	SS := config.GetSessionManager()
	userName := SS.GetString(r.Context(), "name")
	isLoggedIn := SS.GetBool(r.Context(), "isLoggedIn")
	if isLoggedIn != true || userName == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	dropList, _ := repository.ListUserDrops(userName)

	err := r.ParseForm()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse form entry: %s", err), http.StatusBadRequest)
		return
	}

	repoName := strings.TrimSpace(r.FormValue("reponame"))

	if repoName == "" {
		paramData := viewBackend.FrontEndParams{
			Title:           "InkDrop Browser",
			Message:         "Browse the InkDrop machine",
			Name:            userName,
			IsViewingPublic: false,
			RepoList:        dropList,
			Error:           make(map[string]string),
		}
		paramData.Error["general"] = "Drop name is required."
		enrichAccountParams(&paramData, userName)
		viewBackend.IndexMain(w, paramData)
		return
	}

	repoNameCooked, err := repository.ProcessDropName(repoName)
	if err != nil {
		fmt.Println("Error processing Drop name:", err)
		paramData := viewBackend.FrontEndParams{
			Title:           "InkDrop Browser",
			Message:         "Browse the InkDrop machine",
			Name:            userName,
			IsViewingPublic: false,
			RepoList:        dropList,
			Error:           make(map[string]string),
		}

		paramData.Error["general"] = fmt.Sprintf("Error creating Drop: %s\n", err)

		enrichAccountParams(&paramData, userName)
		viewBackend.IndexMain(w, paramData)
		return
	}

	err = repository.CreateNewUserDrop(userName, repoNameCooked)
	if err != nil {
		fmt.Println("Error creating new user Drop:", err)
		paramData := viewBackend.FrontEndParams{
			Title:           "InkDrop Browser",
			Message:         "Browse the InkDrop machine",
			Name:            userName,
			IsViewingPublic: false,
			RepoList:        dropList,
			Error:           make(map[string]string),
		}

		paramData.Error["general"] = fmt.Sprintf("Error creating Drop: %s\n", err)

		enrichAccountParams(&paramData, userName)
		viewBackend.IndexMain(w, paramData)
		return
	}

	fmt.Printf("New Drop Created: %s/%s\n", userName, repoNameCooked)

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func DeleteRepository(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	SS := config.GetSessionManager()
	userName := SS.GetString(r.Context(), "name")
	isLoggedIn := SS.GetBool(r.Context(), "isLoggedIn")
	if isLoggedIn != true {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Failed to parse repo delete form", http.StatusServiceUnavailable)
		return
	}
	repoName := r.FormValue("reponame")
	err = repository.DeleteUserDrop(userName, repoName)
	if err != nil {
		_dropList, _ := repository.ListUserDrops(userName)
		paramData := viewBackend.FrontEndParams{
			Title:           "InkDrop Browser",
			Message:         "Browse the InkDrop machine",
			Name:            userName,
			IsViewingPublic: false,
			RepoList:        _dropList,
			Error:           make(map[string]string),
		}

		paramData.Error["general"] = fmt.Sprintf("Failed to delete Drop %s/%s: %s", userName, repoName, err)
		fmt.Printf("Failed to delete Drop %s/%s: %s", userName, repoName, err)
		enrichAccountParams(&paramData, userName)
		viewBackend.IndexMain(w, paramData)
		return
	}

	fmt.Printf("Successfully removed %s/%s\n", userName, repoName)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func IndexMainBrowseRepository(w http.ResponseWriter, r *http.Request) {
	SS := config.GetSessionManager()
	var userOwnsRepo bool

	name := SS.GetString(r.Context(), "name")

	repoName := chi.URLParam(r, "reponame")
	userName := chi.URLParam(r, "user")
	requestedPath := normalizeBrowserPath(chi.URLParam(r, "*"))
	isTrashView := repository.IsTrashBrowserPath(requestedPath)
	listingPath := requestedPath
	if isTrashView {
		listingPath = repository.MapTrashBrowserPath(requestedPath)
	}
	fmt.Println(repoName, userName, requestedPath)

	if name == userName {
		userOwnsRepo = true
	} else {
		userOwnsRepo = false
	}

	// Determine if repository is publicly viewable via metadata; allow access
	// only to public repositories, the path owner, or users explicitly listed
	// in the repo metadata owners list.
	accessAllowed := false
	isPublic := false
	isOwner := name == userName
	if meta, err := repository.LoadRepoMeta(userName, repoName); err == nil && meta != nil {
		if meta.Public {
			isPublic = true
			accessAllowed = true
		}
		for _, o := range meta.Owners {
			if o == name {
				accessAllowed = true
				break
			}
		}
	}
	if isOwner {
		accessAllowed = true
	}
	if !accessAllowed && name != "" && userModel.CanReadContactDrops(config.GetDB(), name, userName) {
		accessAllowed = true
	}

	if !accessAllowed {
		fmt.Printf("User %s tried to access repository %s/%s, but was denied.\n", name, userName, repoName)
		param := viewBackend.FrontEndParams{
			Title:    fmt.Sprintf("%s/%s - InkDrop", userName, repoName),
			Name:     name,
			Error:    make(map[string]string),
			Message:  "Drop Unavailable",
			Message2: userName,
			Message3: repoName,
			Path:     requestedPath,
		}
		if param.Name == "" {
			param.Name = "Guest"
		}
		param.Error["general"] = "You do not have permission to view this Drop."
		w.WriteHeader(http.StatusForbidden)
		viewBackend.IndexMainBrowseRepository(w, param)
		return
	}

	paramData := viewBackend.FrontEndParams{
		Title:    fmt.Sprintf("%s/%s - InkDrop", userName, repoName),
		Name:     name,
		Error:    make(map[string]string),
		Message2: userName,
		Message3: repoName,
		Path:     requestedPath,
	}

	if paramData.Name == "" {
		paramData.Name = "Guest"
	}

	paramData.IsViewingPublic = isPublic
	paramData.IsTrashView = isTrashView

	if userOwnsRepo == true {
		if isTrashView {
			paramData.Message = fmt.Sprintf("Trash for '%s'", repoName)
		} else {
			paramData.Message = fmt.Sprintf("Browsing your repository '%s'", repoName)
		}
		paramData.UserOwnsDrop = true
	} else {
		paramData.Message = "You are viewing this Drop in read-only mode."
		paramData.UserOwnsDrop = false
	}

	if isTrashView && userOwnsRepo {
		_ = repository.EnsureTrash(userName, repoName)
	}

	directoryListing, err := repository.GetDirectoryListing(userName, repoName, listingPath)
	if err != nil {
		paramData.Error["general"] = fmt.Sprintf("Failed to get directory listing of %s/%s%s: %s", userName, repoName, requestedPath, err)
	}
	if directoryListing == nil {
		if requestedPath == "/" {
			paramData.Error["general"] = "The Drop is empty. If you are the owner, consider uploading files."
		}
	}

	if err == nil {
		if directoryListing == nil {
			directoryListing = []string{}
		}
		directoryListing = decorateRepositoryListing(directoryListing, requestedPath, userOwnsRepo || repository.HasTrash(userName, repoName), isTrashView)
	}
	paramData.RepoList = directoryListing

	// Load drop metadata if present
	if meta, err := repository.LoadRepoMeta(userName, repoName); err == nil && meta != nil {
		paramData.RepoDescription = meta.Description
		paramData.RepoOwners = strings.Join(meta.Owners, ",")
		paramData.RepoPublic = meta.Public
	}

	enrichAccountParams(&paramData, name)
	fmt.Printf("User %s tried to access repository %s/%s%s", name, userName, repoName, requestedPath)
	viewBackend.IndexMainBrowseRepository(w, paramData)
}

// RepositorySettings handles updates to repository metadata (description, owners, permissions)
func RepositorySettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	SS := config.GetSessionManager()
	isLoggedIn := SS.GetBool(r.Context(), "isLoggedIn")
	userName := SS.GetString(r.Context(), "name")
	if isLoggedIn != true || userName == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	repoName := strings.TrimSpace(r.FormValue("repository"))
	if repoName == "" {
		http.Error(w, "Drop required", http.StatusBadRequest)
		return
	}

	// Determine the repository owner from the form (may be different from
	// the session user when editing a shared repository).
	repoOwner := strings.TrimSpace(r.FormValue("user"))
	if repoOwner == "" {
		repoOwner = userName
	}

	// load existing meta for the repo owner and enforce ownership
	meta, _ := repository.LoadRepoMeta(repoOwner, repoName)
	authorized := false
	if meta != nil {
		for _, o := range meta.Owners {
			if o == userName {
				authorized = true
				break
			}
		}
	} else {
		// If no meta exists, only the path owner may update settings
		if repoOwner == userName {
			authorized = true
		}
	}

	if !authorized {
		http.Error(w, "Forbidden - not an owner", http.StatusForbidden)
		return
	}

	description := strings.TrimSpace(r.FormValue("description"))
	ownersRaw := strings.TrimSpace(r.FormValue("owners"))
	publicFlag := r.FormValue("public") == "1" || r.FormValue("public") == "on"

	// Validate owners; if none valid, keep existing owners or fall back to repoOwner
	owners, err := repository.ValidateOwners(ownersRaw)
	if err != nil {
		if meta != nil && len(meta.Owners) > 0 {
			owners = meta.Owners
		} else {
			owners = []string{repoOwner}
		}
	}
	ownerExists := false
	for _, o := range owners {
		if o == repoOwner {
			ownerExists = true
			break
		}
	}
	if !ownerExists {
		owners = append(owners, repoOwner)
	}

	newMeta := &repository.RepoMeta{
		Owners:      owners,
		Description: description,
		Public:      publicFlag,
	}
	if err := repository.SaveRepoMeta(repoOwner, repoName, newMeta); err != nil {
		http.Error(w, "Failed to save metadata", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, buildBrowseRoutePath(repoOwner, repoName, "/"), http.StatusSeeOther)
}

func RepositoryCreateNewDirectory(w http.ResponseWriter, r *http.Request) {
	SS := config.GetSessionManager()

	isLoggedIn := SS.GetBool(r.Context(), "isLoggedIn")
	userName := SS.GetString(r.Context(), "name")

	if isLoggedIn != true || userName == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Failed to parse user form. Try again.", http.StatusBadRequest)
		return
	}

	folderName := r.FormValue("folderName")
	repoName := r.FormValue("repository")
	// 'user' form field is the repository owner (may differ from session user)
	repoOwner := strings.TrimSpace(r.FormValue("user"))
	workingDir := r.FormValue("working-directory")
	workingDir = normalizeBrowserPath(workingDir)
	folderName = strings.TrimSpace(folderName)
	folderName = strings.ReplaceAll(folderName, " ", "_")

	if pass, _ := regexp.MatchString("^[A-Za-z0-9_-]+$", folderName); !pass {
		http.Error(w, "Invalid folder name. Only letters, numbers, underscores and hyphens allowed.", http.StatusBadRequest)
		return
	}

	// Determine repo owner; fallback to session user
	if repoOwner == "" {
		repoOwner = userName
	}

	// Ensure repository exists
	root := fmt.Sprintf("%s/%s/%s", repository.DropDir, repoOwner, repoName)
	ok, derr := repository.DirExists(root)
	if derr != nil {
		http.Error(w, "Failed to verify Drop existence", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "Drop not found", http.StatusNotFound)
		return
	}

	// Permission check: allow if session user is the repo owner, or if listed in owners
	allowed := false
	if repoOwner == userName {
		allowed = true
	} else {
		if meta, _ := repository.LoadRepoMeta(repoOwner, repoName); meta != nil {
			for _, o := range meta.Owners {
				if o == userName {
					allowed = true
					break
				}
			}
		}
	}
	if !allowed {
		http.Error(w, "You do not have permission to modify this Drop", http.StatusForbidden)
		return
	}

	err = repository.CreateNewDirectory(repoOwner, repoName, workingDir, folderName)
	if err != nil {
		http.Error(w, "Failed to create new folder. Go back and try again later.", http.StatusServiceUnavailable)
		return
	}

	fmt.Printf("User %s created a new folder: '%s' at '%s' in %s/%s\n", userName, folderName, workingDir, repoOwner, repoName)

	wd := strings.TrimPrefix(workingDir, "/")
	wd = strings.TrimSuffix(wd, "/")
	if wd == "" {
		http.Redirect(w, r, buildBrowseRoutePath(repoOwner, repoName, "/"), http.StatusSeeOther)
		return
	} else {
		http.Redirect(w, r, buildBrowseRoutePath(repoOwner, repoName, wd), http.StatusSeeOther)
		return
	}
}

func RepositoryCreateNewFile(w http.ResponseWriter, r *http.Request) {
	SS := config.GetSessionManager()

	isLoggedIn := SS.GetBool(r.Context(), "isLoggedIn")
	userName := SS.GetString(r.Context(), "name")

	if isLoggedIn != true || userName == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse user form. Try again.", http.StatusBadRequest)
		return
	}

	fileName := strings.TrimSpace(r.FormValue("fileName"))
	fileType := strings.TrimSpace(r.FormValue("fileType"))
	fileExtension := strings.TrimSpace(r.FormValue("fileExtension"))
	repoName := r.FormValue("repository")
	repoOwner := strings.TrimSpace(r.FormValue("user"))
	workingDir := normalizeBrowserPath(r.FormValue("working-directory"))

	if repoOwner == "" {
		repoOwner = userName
	}

	root := fmt.Sprintf("%s/%s/%s", repository.DropDir, repoOwner, repoName)
	ok, derr := repository.DirExists(root)
	if derr != nil {
		http.Error(w, "failed to verify Drop existence", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "Drop not found", http.StatusNotFound)
		return
	}

	allowed := false
	if repoOwner == userName {
		allowed = true
	} else {
		if meta, _ := repository.LoadRepoMeta(repoOwner, repoName); meta != nil {
			for _, owner := range meta.Owners {
				if owner == userName {
					allowed = true
					break
				}
			}
		}
	}
	if !allowed {
		http.Error(w, "You do not have permission to modify this Drop", http.StatusForbidden)
		return
	}

	createdFileName, err := repository.CreateEmptyEditableFile(repoOwner, repoName, workingDir, fileName, fileType, fileExtension)
	if err != nil {
		statusCode := http.StatusServiceUnavailable
		switch {
		case errors.Is(err, repository.ErrItemExists):
			statusCode = http.StatusConflict
		case strings.Contains(strings.ToLower(err.Error()), "required"),
			strings.Contains(strings.ToLower(err.Error()), "invalid"),
			strings.Contains(strings.ToLower(err.Error()), "unsupported"):
			statusCode = http.StatusBadRequest
		}
		http.Error(w, fmt.Sprintf("Failed to create new file: %s", err), statusCode)
		return
	}

	fmt.Printf("User %s created a new %s file: '%s' at '%s' in %s/%s\n", userName, fileType, createdFileName, workingDir, repoOwner, repoName)

	wd := strings.TrimPrefix(workingDir, "/")
	wd = strings.TrimSuffix(wd, "/")
	if wd == "" {
		http.Redirect(w, r, buildBrowseRoutePath(repoOwner, repoName, "/"), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, buildBrowseRoutePath(repoOwner, repoName, wd), http.StatusSeeOther)
}

func RepositoryRenameItem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	SS := config.GetSessionManager()
	isLoggedIn := SS.GetBool(r.Context(), "isLoggedIn")
	userName := SS.GetString(r.Context(), "name")

	if isLoggedIn != true || userName == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Failed to parse form data.", http.StatusBadRequest)
		return
	}

	repoName := r.FormValue("repository")
	repoOwner := strings.TrimSpace(r.FormValue("user"))
	workingDir := r.FormValue("working-directory")
	oldName := r.FormValue("oldName")
	newName := r.FormValue("newName")
	workingDir = normalizeBrowserPath(workingDir)

	if oldName == "" || oldName == ".." {
		http.Error(w, "Invalid source name.", http.StatusBadRequest)
		return
	}
	if !isValidMovePath(newName) {
		http.Error(w, "Invalid destination path.", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(repoOwner) == "" {
		repoOwner = userName
	}

	// Ensure repo exists
	repoPath := fmt.Sprintf("%s/%s/%s", repository.DropDir, repoOwner, repoName)
	if ok, derr := repository.DirExists(repoPath); derr != nil {
		http.Error(w, "failed to verify Drop existence", http.StatusInternalServerError)
		return
	} else if !ok {
		http.Error(w, "Drop not found", http.StatusNotFound)
		return
	}

	// Permission: allow if session user is owner or listed in repo metadata
	allowed := false
	if repoOwner == userName {
		allowed = true
	} else {
		if meta, _ := repository.LoadRepoMeta(repoOwner, repoName); meta != nil {
			for _, o := range meta.Owners {
				if o == userName {
					allowed = true
					break
				}
			}
		}
	}
	if !allowed {
		http.Error(w, "You do not have permission to modify this Drop", http.StatusForbidden)
		return
	}

	err = repository.RenameItem(repoOwner, repoName, workingDir, oldName, newName)
	if err != nil {
		http.Error(w, fmt.Sprintf("Rename failed: %s", err), http.StatusServiceUnavailable)
		return
	}

	wd := strings.TrimPrefix(workingDir, "/")
	wd = strings.TrimSuffix(wd, "/")
	var redirectPath string
	if wd == "" {
		redirectPath = buildBrowseRoutePath(repoOwner, repoName, "/")
	} else {
		redirectPath = buildBrowseRoutePath(repoOwner, repoName, wd)
	}
	http.Redirect(w, r, redirectPath, http.StatusSeeOther)
}

func RepositoryDeleteItem(w http.ResponseWriter, r *http.Request) {
	SS := config.GetSessionManager()

	isLoggedIn := SS.GetBool(r.Context(), "isLoggedIn")
	userName := SS.GetString(r.Context(), "name")
	if isLoggedIn != true || userName == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form data.", http.StatusBadRequest)
		return
	}

	repoName := r.FormValue("repository")
	repoOwner := strings.TrimSpace(r.FormValue("user"))
	workingDir := normalizeBrowserPath(r.FormValue("working-directory"))
	itemNames := r.Form["itemName"]
	if len(itemNames) == 0 {
		trimmed := strings.TrimSpace(r.FormValue("itemName"))
		if trimmed != "" {
			itemNames = []string{trimmed}
		}
	}
	if len(itemNames) == 0 {
		http.Error(w, "Invalid item for deletion.", http.StatusBadRequest)
		return
	}

	// Determine the repository owner (path owner or provided owner)
	owner := userName
	if repoOwner != "" {
		owner = repoOwner
	}

	// Ensure repository exists
	repoPath := fmt.Sprintf("%s/%s/%s", repository.DropDir, owner, repoName)
	if ok, derr := repository.DirExists(repoPath); derr != nil {
		http.Error(w, "failed to verify Drop existence", http.StatusInternalServerError)
		return
	} else if !ok {
		http.Error(w, "Drop not found", http.StatusNotFound)
		return
	}

	// Permission check: owners or listed owners can delete items
	allowed := false
	if owner == userName {
		allowed = true
	} else {
		if meta, _ := repository.LoadRepoMeta(owner, repoName); meta != nil {
			for _, o := range meta.Owners {
				if o == userName {
					allowed = true
					break
				}
			}
		}
	}
	if !allowed {
		http.Error(w, "You do not have permission to modify this Drop", http.StatusForbidden)
		return
	}

	for _, itemName := range itemNames {
		itemName = strings.TrimSpace(itemName)
		if itemName == "" || itemName == ".." {
			http.Error(w, "Invalid item for deletion.", http.StatusBadRequest)
			return
		}
		if err := repository.DeleteItem(owner, repoName, workingDir, itemName); err != nil {
			http.Error(w, "Delete failed. Try again later.", http.StatusServiceUnavailable)
			return
		}
	}

	wd := strings.TrimPrefix(workingDir, "/")
	wd = strings.TrimSuffix(wd, "/")
	var redirectPath string
	if wd == "" {
		redirectPath = buildBrowseRoutePath(owner, repoName, "/")
	} else {
		redirectPath = buildBrowseRoutePath(owner, repoName, wd)
	}
	http.Redirect(w, r, redirectPath, http.StatusSeeOther)
}

func RepositoryEmptyTrash(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	SS := config.GetSessionManager()
	userName := SS.GetString(r.Context(), "name")
	if SS.GetBool(r.Context(), "isLoggedIn") != true || userName == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse trash form.", http.StatusBadRequest)
		return
	}
	repoName := r.FormValue("repository")
	repoOwner := strings.TrimSpace(r.FormValue("user"))
	if repoOwner == "" {
		repoOwner = userName
	}
	if !canModifyRepository(userName, repoOwner, repoName) {
		http.Error(w, "You do not have permission to modify this Drop", http.StatusForbidden)
		return
	}
	if err := repository.EmptyTrash(repoOwner, repoName); err != nil {
		http.Error(w, fmt.Sprintf("Failed to empty trash: %s", err), http.StatusServiceUnavailable)
		return
	}
	http.Redirect(w, r, buildBrowseRoutePath(repoOwner, repoName, repository.TrashDisplayName), http.StatusSeeOther)
}

func RepositoryRestoreTrash(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	SS := config.GetSessionManager()
	userName := SS.GetString(r.Context(), "name")
	if SS.GetBool(r.Context(), "isLoggedIn") != true || userName == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse trash restore form.", http.StatusBadRequest)
		return
	}
	repoName := r.FormValue("repository")
	repoOwner := strings.TrimSpace(r.FormValue("user"))
	if repoOwner == "" {
		repoOwner = userName
	}
	if !canModifyRepository(userName, repoOwner, repoName) {
		http.Error(w, "You do not have permission to modify this Drop", http.StatusForbidden)
		return
	}
	itemNames := r.Form["itemName"]
	workingDir := normalizeBrowserPath(r.FormValue("working-directory"))
	trashPrefix := ""
	if repository.IsTrashBrowserPath(workingDir) {
		trashPrefix = strings.TrimPrefix(workingDir, "/"+repository.TrashDisplayName)
		trashPrefix = strings.Trim(trashPrefix, "/")
	}
	if len(itemNames) == 0 {
		itemName := strings.TrimSpace(r.FormValue("itemName"))
		if itemName != "" {
			itemNames = []string{itemName}
		}
	}
	if len(itemNames) == 0 {
		http.Error(w, "Select at least one trash item to restore.", http.StatusBadRequest)
		return
	}
	for _, itemName := range itemNames {
		if trashPrefix != "" {
			itemName = path.Join(trashPrefix, itemName)
		}
		if _, err := repository.RestoreTrashItem(repoOwner, repoName, itemName); err != nil {
			http.Error(w, fmt.Sprintf("Restore failed: %s", err), http.StatusServiceUnavailable)
			return
		}
	}
	http.Redirect(w, r, buildBrowseRoutePath(repoOwner, repoName, repository.TrashDisplayName), http.StatusSeeOther)
}

func canModifyRepository(sessionUser, repoOwner, repoName string) bool {
	if sessionUser == "" || repoName == "" {
		return false
	}
	if sessionUser == repoOwner {
		return true
	}
	if meta, _ := repository.LoadRepoMeta(repoOwner, repoName); meta != nil {
		for _, owner := range meta.Owners {
			if owner == sessionUser {
				return true
			}
		}
	}
	return false
}

func canReadRepository(sessionUser, repoOwner, repoName string, isPublic bool) bool {
	if isPublic {
		return true
	}
	if sessionUser == "" || repoName == "" {
		return false
	}
	if sessionUser == repoOwner {
		return true
	}
	if meta, _ := repository.LoadRepoMeta(repoOwner, repoName); meta != nil {
		for _, owner := range meta.Owners {
			if owner == sessionUser {
				return true
			}
		}
	}
	return userModel.CanReadContactDrops(config.GetDB(), sessionUser, repoOwner)
}

func normalizeBrowserPath(raw string) string {
	if raw == "" {
		return "/"
	}
	clean := path.Clean("/" + raw)
	if clean == "." || clean == "" {
		return "/"
	}
	return clean
}

func buildBrowseRoutePath(userName string, repoName string, rawPath string) string {
	base := fmt.Sprintf("/browse/%s/%s", url.PathEscape(userName), url.PathEscape(repoName))
	safePath := normalizeBrowserPath(rawPath)
	if safePath == "/" {
		return base
	}

	segments := strings.Split(strings.Trim(safePath, "/"), "/")
	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}

	return base + "/" + strings.Join(segments, "/")
}

func decorateRepositoryListing(entries []string, browserPath string, includeTrash bool, isTrashView bool) []string {
	cleaned := make([]string, 0, len(entries)+1)
	for _, entry := range entries {
		if entry == repository.TrashInternalRoot+"/" || entry == repository.TrashDisplayName+"/" {
			continue
		}
		cleaned = append(cleaned, entry)
	}
	if normalizeBrowserPath(browserPath) != "/" {
		return append([]string{".."}, cleaned...)
	}
	if includeTrash {
		cleaned = append([]string{repository.TrashDisplayName + "/"}, cleaned...)
	}
	return cleaned
}

func isValidMovePath(raw string) bool {
	candidate := strings.TrimSpace(raw)
	if candidate == "" || strings.HasPrefix(candidate, "/") {
		return false
	}
	// if strings.Contains(candidate, "..") {
	// 	return false
	// }

	parts := strings.Split(candidate, "/")
	// for _, part := range parts {
	// 	if part == "" {
	// 		return false
	// 	}
	// 	// if pass, _ := regexp.MatchString("^[A-Za-z0-9_-]+$", part); !pass {
	// 	// 	return false
	// 	// }
	// }
	if slices.Contains(parts, "") {
		return false
	}
	return true
}

func RepositoryDownloadFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	SS := config.GetSessionManager()
	userName := SS.GetString(r.Context(), "name")
	isLoggedIn := SS.GetBool(r.Context(), "isLoggedIn")

	err := r.ParseForm()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse download form: %s", err), http.StatusServiceUnavailable)
		return
	}

	repoName := r.FormValue("repository")
	workingDir := normalizeBrowserPath(r.FormValue("working-directory"))
	if repository.IsTrashBrowserPath(workingDir) {
		workingDir = repository.MapTrashBrowserPath(workingDir)
	}
	itemNames := r.Form["itemName"]
	if len(itemNames) == 0 {
		itemName := strings.TrimSpace(r.FormValue("itemName"))
		if itemName != "" {
			itemNames = []string{itemName}
		}
	}
	if len(itemNames) == 0 {
		http.Error(w, "Invalid file requested.", http.StatusBadRequest)
		return
	}
	owner := r.FormValue("user")
	if owner == "" {
		owner = userName
	}

	// Check repo metadata for public access
	isPublic := false
	if meta, err := repository.LoadRepoMeta(owner, repoName); err == nil && meta != nil {
		if meta.Public {
			isPublic = true
		}
	}

	if !canReadRepository(userName, owner, repoName, isPublic) {
		if isLoggedIn != true || userName == "" {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	fmt.Printf("User %s tried to download %v at %s at %s\n", userName, itemNames, workingDir, repoName)

	if len(itemNames) == 1 {
		fileToDownload, err := repository.GetItemPath(owner, repoName, workingDir, itemNames[0])
		if err != nil {
			http.Error(w, "Invalid file requested.", http.StatusBadRequest)
			return
		}

		file, err := os.Open(fileToDownload)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to open file: %s", err), http.StatusNotFound)
			return
		}
		defer file.Close()

		stat, err := file.Stat()
		if err != nil {
			http.Error(w, "Requested item is not downloadable.", http.StatusBadRequest)
			return
		}
		if stat.Mode().IsRegular() {
			buf := make([]byte, 512)
			n, _ := file.Read(buf)
			contentType := http.DetectContentType(buf[:n])
			if _, err := file.Seek(0, io.SeekStart); err != nil {
				http.Error(w, "Unable to read file.", http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Disposition", "attachment; filename="+strconv.Quote(itemNames[0]))
			w.Header().Set("Content-Type", contentType)
			w.Header().Set("X-Content-Type-Options", "nosniff")
			http.ServeContent(w, r, stat.Name(), stat.ModTime(), file)
			return
		}
	}

	archiveName := fmt.Sprintf("%s-selection.zip", repoName)
	if len(itemNames) == 1 {
		archiveName = strings.TrimSuffix(itemNames[0], "/") + ".zip"
	}

	w.Header().Set("Content-Disposition", "attachment; filename="+strconv.Quote(archiveName))
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	for _, itemName := range itemNames {
		itemName = strings.TrimSpace(itemName)
		if itemName == "" {
			continue
		}
		itemPath, err := repository.GetItemPath(owner, repoName, workingDir, itemName)
		if err != nil {
			http.Error(w, "Invalid file requested.", http.StatusBadRequest)
			return
		}
		if err := addPathToZip(zipWriter, itemPath, strings.TrimSuffix(itemName, "/")); err != nil {
			http.Error(w, fmt.Sprintf("Failed to archive selection: %s", err), http.StatusInternalServerError)
			return
		}
	}
}

func addPathToZip(zipWriter *zip.Writer, sourcePath string, archiveBase string) error {
	info, err := os.Stat(sourcePath)
	if err != nil {
		return err
	}
	if archiveBase == "" {
		archiveBase = filepath.Base(sourcePath)
	}

	if info.Mode().IsRegular() {
		return addFileToZip(zipWriter, sourcePath, archiveBase, info)
	}

	if !info.IsDir() {
		return fmt.Errorf("unsupported download item: %s", archiveBase)
	}

	return filepath.Walk(sourcePath, func(currentPath string, currentInfo os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relPath, err := filepath.Rel(sourcePath, currentPath)
		if err != nil {
			return err
		}
		zipPath := filepath.ToSlash(filepath.Join(archiveBase, relPath))
		if relPath == "." {
			header, err := zip.FileInfoHeader(currentInfo)
			if err != nil {
				return err
			}
			header.Name = strings.TrimSuffix(zipPath, "/") + "/"
			_, err = zipWriter.CreateHeader(header)
			return err
		}
		if currentInfo.IsDir() {
			header, err := zip.FileInfoHeader(currentInfo)
			if err != nil {
				return err
			}
			header.Name = strings.TrimSuffix(zipPath, "/") + "/"
			_, err = zipWriter.CreateHeader(header)
			return err
		}
		return addFileToZip(zipWriter, currentPath, filepath.ToSlash(zipPath), currentInfo)
	})
}

func addFileToZip(zipWriter *zip.Writer, sourcePath string, archivePath string, info os.FileInfo) error {
	file, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer file.Close()

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = archivePath
	header.Method = zip.Deflate
	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = io.Copy(writer, file)
	return err
}

func RepositoryPreviewFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	SS := config.GetSessionManager()
	userName := SS.GetString(r.Context(), "name")
	isLoggedIn := SS.GetBool(r.Context(), "isLoggedIn")

	query := r.URL.Query()
	repoName := query.Get("repository")
	workingDir := normalizeBrowserPath(query.Get("working-directory"))
	if repository.IsTrashBrowserPath(workingDir) {
		workingDir = repository.MapTrashBrowserPath(workingDir)
	}
	itemName := query.Get("itemName")
	owner := query.Get("user")

	if owner == "" {
		// fall back to session user if owner not provided
		owner = userName
	}

	// Check repo metadata for public access
	isPublic := false
	if meta, err := repository.LoadRepoMeta(owner, repoName); err == nil && meta != nil {
		if meta.Public {
			isPublic = true
		}
	}

	if !canReadRepository(userName, owner, repoName, isPublic) {
		if isLoggedIn != true || userName == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if itemName == "" {
		http.Error(w, "Invalid file requested.", http.StatusBadRequest)
		return
	}

	fileToPreview, err := repository.GetItemPath(owner, repoName, workingDir, itemName)
	if err != nil {
		http.Error(w, "Invalid file requested.", http.StatusBadRequest)
		return
	}

	file, err := os.Open(fileToPreview)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to open file: %s", err), http.StatusNotFound)
		return
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil || !stat.Mode().IsRegular() {
		http.Error(w, "Requested item is not a previewable file.", http.StatusBadRequest)
		return
	}

	buf := make([]byte, 512)
	n, _ := file.Read(buf)
	contentType := http.DetectContentType(buf[:n])
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		http.Error(w, "Unable to read file.", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Disposition", "inline")
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, max-age=60")
	http.ServeContent(w, r, stat.Name(), stat.ModTime(), file)
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func SessionConfirm(w http.ResponseWriter, r *http.Request) {
	SS := config.GetSessionManager()
	isLoggedIn := SS.GetBool(r.Context(), "isLoggedIn")
	name := SS.GetString(r.Context(), "name")

	// If called by an API client (FtR sets X-FTR-CLIENT) or requests JSON,
	// return structured JSON including the username so CLI clients can
	// reliably discover who the session belongs to.
	if r.Header.Get("X-FTR-CLIENT") != "" || strings.Contains(r.Header.Get("Accept"), "application/json") || r.URL.Query().Get("api") == "1" {
		if isLoggedIn {
			writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "username": name})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": false})
		return
	}

	if isLoggedIn {
		w.Write([]byte("true"))
		return
	}
	w.Write([]byte("false"))
}

func RepositoryIndex(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	searchQuery := strings.TrimSpace(query.Get("search"))
	apiMode := query.Get("api") == "1"
	if apiMode && searchQuery != "" {
		SS := config.GetSessionManager()
		currentUser := SS.GetString(r.Context(), "name")
	matches, err := repository.SearchDrops(searchQuery, currentUser)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": err.Error()})
			return
		}
		// Filter out internal/system users (e.g. _meta) from search results
		filtered := []map[string]string{}
		for _, m := range matches {
			if u, ok := m["user"]; ok {
				if strings.HasPrefix(u, "_") {
					continue
				}
			}
			filtered = append(filtered, m)
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "matches": filtered})
		return
	}
	IndexMain(w, r)
}

func RepositoryAPI(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	repoName := query.Get("name")
	userName := query.Get("user")
	if repoName == "" || userName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "user and repo are required"})
		return
	}

	if query.Get("list") == "1" {
		files, err := repository.ListDropFilesRecursive(userName, repoName)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "files": files})
		return
	}

	if query.Get("filemeta") == "1" {
		fileName := query.Get("file")
		if fileName == "" {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "file parameter is required"})
			return
		}
		filePath, err := repository.GetItemPath(userName, repoName, "/", fileName)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "invalid file path"})
			return
		}
		hash, err := repository.CalculateFileHash(filePath)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": "failed to compute file metadata"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "hash": hash, "flagged": "0", "encrypted": "0", "signature": ""})
		return
	}

	if query.Get("meta") == "1" {
		m, err := repository.LoadRepoMeta(userName, repoName)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": err.Error()})
			return
		}
		// Ensure repository directory exists; return not-found if missing so clients
		// can decide to create the repository when authorized.
		repoPath := fmt.Sprintf("%s/%s/%s", repository.DropDir, userName, repoName)
		if ok, _ := repository.DirExists(repoPath); !ok {
			writeJSON(w, http.StatusNotFound, map[string]interface{}{"success": false, "error": "drop not found"})
			return
		}
		if m == nil {
			writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "meta": map[string]interface{}{}})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "meta": m})
		return
	}

	if downloadName := query.Get("download"); downloadName != "" {
		// allow both POST and GET for compatibility
		filePath, err := repository.GetItemPath(userName, repoName, "/", downloadName)
		if err != nil {
			http.Error(w, "Invalid file requested.", http.StatusBadRequest)
			return
		}
		file, err := os.Open(filePath)
		if err != nil {
			http.Error(w, "Failed to open file.", http.StatusNotFound)
			return
		}
		defer file.Close()

		stat, err := file.Stat()
		if err != nil || !stat.Mode().IsRegular() {
			http.Error(w, "Requested item is not a downloadable file.", http.StatusBadRequest)
			return
		}

		buf := make([]byte, 512)
		n, _ := file.Read(buf)
		contentType := http.DetectContentType(buf[:n])
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			http.Error(w, "Unable to read file.", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Disposition", "attachment; filename="+strconv.Quote(downloadName))
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		http.ServeContent(w, r, stat.Name(), stat.ModTime(), file)
		return
	}

	if r.Method == http.MethodPost {
		if err := r.ParseMultipartForm(500 << 20); err == nil {
			file, header, err := r.FormFile("upload")
			if err == nil {
				defer file.Close()

				// Require authentication for uploads
				SS := config.GetSessionManager()
				sessionUser := SS.GetString(r.Context(), "name")
				isLoggedIn := SS.GetBool(r.Context(), "isLoggedIn")
				if !isLoggedIn || sessionUser == "" {
					writeJSON(w, http.StatusUnauthorized, map[string]interface{}{"success": false, "error": "authentication required"})
					return
				}

				destDir := fmt.Sprintf("%s/%s/%s", repository.DropDir, userName, repoName)

				ok, derr := repository.DirExists(destDir)
				if derr != nil {
					writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": "failed to verify Drop"})
					return
				}

				if !ok {
					// repository missing: only path owner may create it via upload
					if sessionUser != userName {
						writeJSON(w, http.StatusNotFound, map[string]interface{}{"success": false, "error": "drop does not exist"})
						return
					}
					if err := os.MkdirAll(destDir, 0755); err != nil {
						writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": "failed to create Drop"})
						return
					}
				} else {
					// repository exists: enforce owner or listed-owner write permission
					if meta, err := repository.LoadRepoMeta(userName, repoName); err == nil && meta != nil {
						if !meta.Public {
							if sessionUser != userName {
								allowed := false
								for _, o := range meta.Owners {
									if o == sessionUser {
										allowed = true
										break
									}
								}
								if !allowed {
									writeJSON(w, http.StatusForbidden, map[string]interface{}{"success": false, "error": "permission denied"})
									return
								}
							}
						}
					} else {
						// no metadata: default to path owner only
						if sessionUser != userName {
							writeJSON(w, http.StatusForbidden, map[string]interface{}{"success": false, "error": "permission denied"})
							return
						}
					}
				}

				destPath := fmt.Sprintf("%s/%s", destDir, path.Base(header.Filename))
				if !strings.HasPrefix(path.Clean(destPath), path.Clean(destDir)) {
					writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "invalid file name"})
					return
				}

				out, err := os.Create(destPath)
				if err != nil {
					writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": "failed to save file"})
					return
				}
				defer out.Close()
				if _, err := io.Copy(out, file); err != nil {
					writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": "failed to save file"})
					return
				}
				writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "message": "upload complete"})
				return
			}
		}
	}

	writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "unsupported api operation"})
}

func RepositoryDownloadRepositoryAsSQAR(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	SS := config.GetSessionManager()
	isLoggedIn := SS.GetBool(r.Context(), "isLoggedIn")
	name := SS.GetString(r.Context(), "name")

	if isLoggedIn != true || name == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	repoName := chi.URLParam(r, "reponame")

	userName := chi.URLParam(r, "user")

	repoToDownload := fmt.Sprintf("%s/%s/%s", repository.DropDir, userName, repoName)
	fmt.Printf("User %s tried to download Drop at %s\n", userName, repoToDownload)

	archive, err := repository.PackSQAR(repoToDownload, userName, repoName)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to pack archive into SQAR file: %s", err), http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename="+strconv.Quote(repoName+".sqar"))
	http.ServeFile(w, r, archive)
	err = os.Remove(archive)
	if err != nil {
		fmt.Printf("Failed to cleanup file %s: %s\n", archive, err)
	}
}

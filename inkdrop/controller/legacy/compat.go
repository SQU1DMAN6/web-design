package legacy

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"inkdrop/config"
	"inkdrop/model"
	"inkdrop/service"
	viewBackend "inkdrop/view/connector"
)

type LegacyCompatHandler struct{ dropSvc *service.DropService }

func NewLegacyCompatHandler(dropSvc *service.DropService) *LegacyCompatHandler {
	return &LegacyCompatHandler{dropSvc: dropSvc}
}

func (h *LegacyCompatHandler) V4Shell(w http.ResponseWriter, r *http.Request) {
	ss := config.GetSessionManager()
	userName := ss.GetString(r.Context(), "name")
	isLoggedIn := ss.GetBool(r.Context(), "isLoggedIn")
	fmt.Fprintf(os.Stderr, "[v4shell] path=%s logged_in=%v name=%q cookies=%q\n", r.URL.Path, isLoggedIn, userName, r.Header.Get("Cookie"))

	p := viewBackend.FrontEndParams{Title: "InkDrop", Name: userName, Error: make(map[string]string)}

	if !isLoggedIn || userName == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	db := config.GetDB()
	user, err := model.GetUserByName(userName, db)
	if err == nil && user != nil {
		p.UserBio = user.Bio
		p.UserPFP = model.ResolveProfilePicture(user.PFP, user.Name)
	}

	path := strings.TrimPrefix(r.URL.Path, "/v4/drop/")
	if path == r.URL.Path {
		path = ""
	}

	if path != "" {
		drop, err := h.dropSvc.GetDrop(path)
		if err == nil && drop != nil {
			p.CurrentDropID = drop.ID
			p.CurrentDropName = drop.Name
			p.Title = drop.Name + " — InkDrop"
		}
	}

	viewBackend.V4Shell(w, p)
}

func RedirectToAccountSettings(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/legacy/account/settings", http.StatusTemporaryRedirect)
}

func RedirectToAccountAPI(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/legacy/api/account", http.StatusTemporaryRedirect)
}

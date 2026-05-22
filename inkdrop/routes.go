package routes

import (
	"inkdrop/controller/account"
	"inkdrop/controller/login"
	"inkdrop/controller/register"
	"inkdrop/controller/repository"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func RegisterRoutes(r chi.Router) {
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Server is up"))
	})

	r.Get("/", repository.IndexMain)
	r.Post("/", repository.IndexMainPost)
	r.Get("/login", login.LoginMain)
	r.Post("/login", login.LoginMainPost)
	r.Get("/register", register.RegisterMain)
	r.Post("/register", register.RegisterMainPost)
	r.Get("/logout", login.LoginLogout)
	r.Get("/logout.php", login.LoginLogout)
	r.Post("/login.php", login.LoginMainPost)
	r.Get("/login.php", login.LoginMain)
	r.Post("/inkdrop/login.php", login.LoginMainPost)
	r.Get("/inkdrop/login.php", login.LoginMain)
	r.Get("/sessionconfirm", repository.SessionConfirm)
	r.Get("/inkdrop/sessionconfirm", repository.SessionConfirm)
	r.Get("/api/account", account.CurrentAccountAPI)
	r.Post("/account/settings", account.UpdateAccountSettings)
	r.Get("/api/community", account.CommunityAPI)
	r.Get("/api/contacts", account.ContactsAPI)
	r.Post("/contacts/request", account.RequestContact)
	r.Post("/contacts/respond", account.RespondContact)
	r.Get("/index.php", repository.RepositoryIndex)
	r.Get("/inkdrop/index.php", repository.RepositoryIndex)
	r.Get("/api/repos", repository.RepositoryListAPI)
	r.Get("/inkdrop/api/repos", repository.RepositoryListAPI)
	r.Get("/repo.php", repository.RepositoryAPI)
	r.Post("/repo.php", repository.RepositoryAPI)
	r.Get("/inkdrop/repo.php", repository.RepositoryAPI)
	r.Post("/inkdrop/repo.php", repository.RepositoryAPI)
	r.Get("/api/fs/{user}/{reponame}", repository.RepositoryFSAPI)
	r.Get("/api/fs/{user}/{reponame}/*", repository.RepositoryFSAPI)
	r.Head("/api/fs/{user}/{reponame}/*", repository.RepositoryFSAPI)
	r.Put("/api/fs/{user}/{reponame}/*", repository.RepositoryFSAPI)
	r.Post("/api/fs/{user}/{reponame}/*", repository.RepositoryFSAPI)
	r.Delete("/api/fs/{user}/{reponame}/*", repository.RepositoryFSAPI)
	r.Get("/inkdrop/api/fs/{user}/{reponame}", repository.RepositoryFSAPI)
	r.Get("/inkdrop/api/fs/{user}/{reponame}/*", repository.RepositoryFSAPI)
	r.Head("/inkdrop/api/fs/{user}/{reponame}/*", repository.RepositoryFSAPI)
	r.Put("/inkdrop/api/fs/{user}/{reponame}/*", repository.RepositoryFSAPI)
	r.Post("/inkdrop/api/fs/{user}/{reponame}/*", repository.RepositoryFSAPI)
	r.Delete("/inkdrop/api/fs/{user}/{reponame}/*", repository.RepositoryFSAPI)
	r.Post("/deleterepo", repository.DeleteRepository)
	r.Get("/browse/{user}/{reponame}/*", repository.IndexMainBrowseRepository)
	r.Get("/browse/{user}/{reponame}", repository.IndexMainBrowseRepository)
	r.Post("/new/dir", repository.RepositoryCreateNewDirectory)
	r.Post("/new/file", repository.RepositoryCreateNewFile)
	r.Post("/rename", repository.RepositoryRenameItem)
	r.Post("/delete/item", repository.RepositoryDeleteItem)
	r.Post("/trash/empty", repository.RepositoryEmptyTrash)
	r.Post("/trash/restore", repository.RepositoryRestoreTrash)
	r.Post("/repo/settings", repository.RepositorySettings)
	r.Post("/download", repository.RepositoryDownloadFile)
	r.Get("/preview", repository.RepositoryPreviewFile)
	r.Get("/downloadrepo/{user}/{reponame}", repository.RepositoryDownloadRepositoryAsSQAR)
	r.Get("/edit/{filename}/{user}/{reponame}", repository.RepositoryLiveEditTextFile)
	r.Get("/edit/{filename}/{user}/{reponame}/*", repository.RepositoryLiveEditTextFile)
	r.Post("/edit/{filename}/{user}/{reponame}", repository.RepositoryLiveEditTextFile)
	r.Post("/edit/{filename}/{user}/{reponame}/*", repository.RepositoryLiveEditTextFile)
}

func NewTUSHandler() http.Handler {
	tusH := repository.TUSHandler()
	return http.StripPrefix("/upload", tusH)
}

package api

import (
	"net/http"

	"inkdrop/apicommon"
	"inkdrop/controller/collaboration"
	"inkdrop/controller/contacts"
	"inkdrop/controller/drop"
	"inkdrop/controller/file"
	"inkdrop/model"
	"inkdrop/service"
	"inkdrop/ws"

	"github.com/go-chi/chi/v5"
)

// RegisterV4Routes registers all API v4 routes on the given router.
func RegisterV4Routes(r chi.Router, dropSvc *service.DropService, fileSvc *service.FileService, permSvc *service.PermissionService, _ *service.SearchService) {
	// Initialize controllers with services
	drop.Init(dropSvc)
	file.Init(fileSvc)
	collaboration.Init(permSvc)

	// All v4 routes are under /api/v4
	r.Route("/api/v4", func(r chi.Router) {
		// Health check (no auth required)
		r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			apicommon.WriteSuccess(w, map[string]string{"status": "ok", "version": "4.0"})
		})

		// Authenticated routes
		r.Group(func(r chi.Router) {
			r.Use(AuthMiddleware)

			// Drops
			r.Get("/drops", drop.ListDrops)
			r.Post("/drops", drop.CreateDrop)
			r.Post("/drops/migrate", drop.MigrateDrop)

			// Single drop endpoint
			r.Route("/drop/{id}", func(r chi.Router) {
				// Viewer-level access for reading drop metadata, files, members, activity
				r.Group(func(r chi.Router) {
					r.Use(RequirePermission(permSvc, model.DropRoleViewer))
					r.Get("/", drop.GetDrop)
					r.Get("/files", file.ListFiles)
					r.Get("/files/{fileId}", file.GetFile)
					r.Get("/files/{fileId}/content", file.ReadFileContent)
					r.Get("/files/{fileId}/download", file.DownloadFile)
					r.Get("/files/{fileId}/preview", file.PreviewFile)
					r.Get("/trash", file.ListTrash)
					r.Get("/members", collaboration.GetMembers)
					r.Get("/activity", GetDropActivity)
				})

				// Editor-level access for file modifications
				r.Group(func(r chi.Router) {
					r.Use(RequirePermission(permSvc, model.DropRoleEditor))
					r.Post("/files", file.CreateFile)
					r.Post("/folders", file.CreateFolder)
					r.Post("/upload", file.UploadFile)
					r.Put("/files/{fileId}", file.UpdateFile)
					r.Delete("/files/{fileId}", file.DeleteFile)
					r.Post("/files/{fileId}/rename", file.RenameFile)
					r.Post("/files/{fileId}/move", file.MoveFile)
					r.Post("/files/{fileId}/trash", file.TrashFile)
					r.Post("/files/{fileId}/restore", file.RestoreFile)
					r.Post("/trash/empty", file.EmptyTrash)
				})

				// Owner-level access for drop management and sharing
				r.Group(func(r chi.Router) {
					r.Use(RequirePermission(permSvc, model.DropRoleOwner))
					r.Put("/", drop.UpdateDrop)
					r.Delete("/", drop.DeleteDrop)
					r.Post("/members", collaboration.AddMember)
					r.Delete("/members/{userId}", collaboration.RemoveMember)
					r.Put("/members/{userId}/role", collaboration.UpdateRole)
					r.Post("/share", collaboration.ShareDrop)
				})
			})
		})

		// Contacts (authenticated)
		r.Group(func(r chi.Router) {
			r.Use(AuthMiddleware)
			r.Get("/contacts", contacts.ListContactsV4)
			r.Post("/contacts", contacts.AddContactV4)
			r.Delete("/contacts/{name}", contacts.RemoveContactV4)
			r.Get("/contacts/search", contacts.SearchUsersV4)
		})

		// Search (authenticated)
		r.Group(func(r chi.Router) {
			r.Use(AuthMiddleware)
			r.Get("/search", SearchAll)
		})

		// WebSocket (authenticated)
		r.Get("/ws", ws.GlobalHub.HandleWebSocket)
	})
}

// GetDropActivity handles GET /api/v4/drop/{id}/activity
func GetDropActivity(w http.ResponseWriter, r *http.Request) {
	dropID := r.PathValue("id")
	if dropID == "" {
		apicommon.WriteError(w, http.StatusBadRequest, "MISSING_ID", "Drop ID is required")
		return
	}

	// Parse limit/offset from query params
	limit := 50
	offset := 0

	activitySvc := service.NewActivityService()
	logs, err := activitySvc.GetDropActivity(dropID, limit, offset)
	if err != nil {
		apicommon.WriteError(w, http.StatusInternalServerError, "FETCH_ERROR", err.Error())
		return
	}

	apicommon.WriteSuccess(w, logs)
}

// SearchAll handles GET /api/v4/search
func SearchAll(w http.ResponseWriter, r *http.Request) {
	userID := GetUserID(r)
	query := r.URL.Query().Get("q")
	if query == "" {
		apicommon.WriteError(w, http.StatusBadRequest, "MISSING_QUERY", "Search query is required")
		return
	}

	searchSvc := service.NewSearchService(service.NewPermissionService())
	results, err := searchSvc.GlobalSearch(query, userID)
	if err != nil {
		apicommon.WriteError(w, http.StatusInternalServerError, "SEARCH_ERROR", err.Error())
		return
	}

	apicommon.WriteSuccess(w, results)
}

package api

import (
	"net/http"

	"inkdrop/apicommon"
	"inkdrop/controller/collaboration"
	"inkdrop/controller/drop"
	"inkdrop/controller/file"
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

			// Single drop operations
			r.Route("/drop/{id}", func(r chi.Router) {
				r.Get("/", drop.GetDrop)
				r.Put("/", drop.UpdateDrop)
				r.Delete("/", drop.DeleteDrop)

				// Files
				r.Get("/files", file.ListFiles)
				r.Post("/files", file.CreateFile)
				r.Get("/files/{fileId}", file.GetFile)
				r.Get("/files/{fileId}/content", file.ReadFileContent)
				r.Put("/files/{fileId}", file.UpdateFile)
				r.Delete("/files/{fileId}", file.DeleteFile)
				r.Post("/files/{fileId}/rename", file.RenameFile)
				r.Post("/files/{fileId}/move", file.MoveFile)

				// Members / sharing
				r.Get("/members", collaboration.GetMembers)
				r.Post("/members", collaboration.AddMember)
				r.Delete("/members/{userId}", collaboration.RemoveMember)
				r.Put("/members/{userId}/role", collaboration.UpdateRole)
				r.Post("/share", collaboration.ShareDrop)

				// Activity
				r.Get("/activity", GetDropActivity)
			})
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

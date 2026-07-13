package app

import (
	routes "inkdrop"
	"inkdrop/api"
	"inkdrop/config"
	"inkdrop/repository"
	"inkdrop/service"
	"inkdrop/storage/filesystem"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

func BootApp() {
	r := chi.NewRouter()
	config.ConnectDatabase()
	if err := repository.EnsureStorageLayout(); err != nil {
		log.Fatalf("failed to initialize InkDrop storage under %s: %v", repository.RootDir, err)
	}
	if err := repository.MigrateLegacyPFPs(); err != nil {
		log.Fatalf("failed to migrate legacy profile pictures: %v", err)
	}
	if err := config.InitSession(); err != nil {
		log.Fatalf("failed to initialize session storage under %s: %v", repository.SessionDir, err)
	}

	ss := config.GetSessionManager()
	r.Use(ss.LoadAndSave)

	RegisterMiddleWares(r)
	RegisterStatic(r)

	// Register legacy routes (InkDrop 3.x compatibility)
	routes.RegisterRoutes(r)

	// Initialize v4 services
	inkdropRoot := repository.RootDir
	store := filesystem.NewStore(inkdropRoot)
	dropSvc := service.NewDropService(store)
	permSvc := service.NewPermissionService()
	fileSvc := service.NewFileService(store, permSvc)
	searchSvc := service.NewSearchService(permSvc)

	// Register v4 API routes
	api.RegisterV4Routes(r, dropSvc, fileSvc, permSvc, searchSvc)

	// The TUS upload handler is served outside the main chi router
	// because the TUS protocol requires trailing slashes in URLs,
	// which conflicts with the StripSlashes middleware on the main router.
	// We wrap it with the session manager so auth checks still work.
	tusHandler := ss.LoadAndSave(routes.NewTUSHandler())

	// Top-level dispatcher: route /upload* to the TUS handler,
	// everything else to the main chi router.
	top := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if strings.HasPrefix(req.URL.Path, "/upload") {
			tusHandler.ServeHTTP(w, req)
			return
		}
		r.ServeHTTP(w, req)
	})

	if err := http.ListenAndServe(":6767", top); err != nil {
		log.Fatal(err)
	}
}

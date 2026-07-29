package app

import (
	routes "inkdrop"
	"inkdrop/api"
	"inkdrop/config"
	controller_legacy "inkdrop/controller/legacy"
	"inkdrop/controller/login"
	"inkdrop/controller/register"
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

	// Ensure all database tables and columns exist before anything else
	if err := repository.EnsureDropsSchema(); err != nil {
		log.Fatalf("failed to ensure drops schema: %v", err)
	}

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

	// Initialize v4 services
	inkdropRoot := repository.RootDir
	store := filesystem.NewStore(inkdropRoot)
	dropSvc := service.NewDropService(store)
	permSvc := service.NewPermissionService()
	fileSvc := service.NewFileService(store, permSvc)
	searchSvc := service.NewSearchService(permSvc)

	// v4 frontend at root
	compatHandler := controller_legacy.NewLegacyCompatHandler(dropSvc)
	r.Get("/", compatHandler.V4Shell)
	r.Get("/v4/drop/{dropID}", compatHandler.V4Shell)

	// v4 API routes
	api.RegisterV4Routes(r, dropSvc, fileSvc, permSvc, searchSvc)

	// Auth routes remain at root (must work for both v3/v4)
	r.Get("/login", login.LoginMain)
	r.Post("/login", login.LoginMainPost)
	r.Get("/register", register.RegisterMain)
	r.Post("/register", register.RegisterMainPost)
	r.Get("/logout", login.LoginLogout)
	r.Post("/account/settings", controller_legacy.RedirectToAccountSettings)
	r.Get("/api/account", controller_legacy.RedirectToAccountAPI)

	// Legacy 3.x app routes under /legacy/ prefix
	r.Route("/legacy", func(r chi.Router) {
		routes.RegisterRoutes(r)
	})

	// TUS upload handler
	tusHandler := ss.LoadAndSave(routes.NewTUSHandler())

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
package app

import (
	"fmt"
	"inkdrop/repository"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"
)

func RegisterStatic(r *chi.Mux) {

	workDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Error getting working directory: %v", err)
	}
	assetsPath := filepath.Join(workDir, "assets")
	viewStaticPath := filepath.Join(workDir, "view", "static")

	fmt.Println("assetsPath", assetsPath)

	checkDirExists(assetsPath, "assets")

	fileServer := func(path string) http.Handler {
		fs := http.FileServer(http.Dir(path))
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			fs.ServeHTTP(w, r)
		})
	}

	// Handle static files
	r.Handle("/assets/*", http.StripPrefix("/assets/", fileServer(assetsPath)))
	r.Handle("/pfp/*", http.StripPrefix("/pfp/", fileServer(repository.UserPFPDir)))

	// Serve v4 static assets (CSS, JS) from view/static/
	// These are served under /assets/ as well, but from a different source directory
	// We mount them at a higher priority by adding them first
	if _, err := os.Stat(viewStaticPath); err == nil {
		r.Handle("/assets/v4.css", http.StripPrefix("/assets/", fileServer(viewStaticPath+"/css")))
		r.Handle("/assets/v4.js", http.StripPrefix("/assets/", fileServer(viewStaticPath+"/js")))
	}
}

func checkDirExists(path string, name string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		log.Printf("Warning: %s directory not found at %s", name, path)
	}
}

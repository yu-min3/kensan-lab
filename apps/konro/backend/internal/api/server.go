// Package api serves the konro REST API and the built SPA.
package api

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"

	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/recipe"
)

// New builds the handler. dataDir holds recipe .md files + images/;
// staticDir (optional) holds the built SPA, served with fallback to
// index.html for client-side routing.
func New(dataDir, staticDir string) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/v1/recipes", func(w http.ResponseWriter, r *http.Request) {
		metas, err := recipe.List(dataDir)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, metas)
	})

	mux.HandleFunc("GET /api/v1/recipes/{name}", func(w http.ResponseWriter, r *http.Request) {
		rec, err := recipe.Load(dataDir, r.PathValue("name"))
		if errors.Is(err, fs.ErrNotExist) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, rec)
	})

	mux.Handle("GET /images/", http.StripPrefix("/images/",
		http.FileServer(http.Dir(filepath.Join(dataDir, "images")))))

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	if staticDir != "" {
		mux.Handle("/", spaHandler(staticDir))
	}
	return mux
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func spaHandler(dir string) http.Handler {
	files := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(dir, filepath.Clean("/"+r.URL.Path))
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			http.ServeFile(w, r, filepath.Join(dir, "index.html"))
			return
		}
		files.ServeHTTP(w, r)
	})
}

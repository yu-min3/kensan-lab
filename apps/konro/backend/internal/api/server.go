// Package api serves the konro REST API and the built SPA.
package api

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/recipe"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/rkimport"
)

// maxImportBytes caps the uploaded export zip. The archive is held in memory
// (readOnlyRootFilesystem leaves no tmp dir to spill to), so this bound is
// what keeps the pod inside its memory limit. Current real exports are ~14MB.
const maxImportBytes = 64 << 20

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

	// Recipe Keeper 再取込: エクスポート zip をそのまま body で受ける
	// （multipart にしない — ParseMultipartForm はサイズ超過時にディスクへ
	// スピルするが、このコンテナに書ける tmp は無い）
	mux.HandleFunc("POST /api/v1/import", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxImportBytes))
		if err != nil {
			http.Error(w, "zip too large (max 64MB)", http.StatusRequestEntityTooLarge)
			return
		}
		zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
		if err != nil {
			http.Error(w, "not a zip archive", http.StatusBadRequest)
			return
		}
		res, images, err := rkimport.ParseZipReader(zr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		stats, err := rkimport.Materialize(res, images, dataDir, time.Now().Format("2006-01-02"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, importReport{
			WriteStats:   stats,
			Warnings:     res.Warnings,
			UnknownProps: res.SortedUnknownProps(),
		})
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

type importReport struct {
	rkimport.WriteStats
	Warnings     []string `json:"warnings"`
	UnknownProps []string `json:"unknownProps"`
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

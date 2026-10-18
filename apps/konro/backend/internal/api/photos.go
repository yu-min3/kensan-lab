package api

import (
	"errors"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/recipe"
	"io"
	"net/http"
	"strings"
)

func registerPhotos(mux *http.ServeMux, dir string) {
	// Reserve capacity before reading the body. Otherwise concurrent requests
	// could each retain 8MB while waiting for the recipe write/decode lock.
	active := make(chan struct{}, 1)
	mux.HandleFunc("PUT /api/v1/recipe-photos/{name...}", func(w http.ResponseWriter, r *http.Request) {
		select {
		case active <- struct{}{}:
			defer func() { <-active }()
		default:
			w.Header().Set("Retry-After", "1")
			http.Error(w, "別の写真を保存しています。少し待ってからもう一度保存してください", http.StatusTooManyRequests)
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, recipe.MaxPhotoBytes))
		if err != nil {
			var limit *http.MaxBytesError
			if errors.As(err, &limit) {
				http.Error(w, "写真が大きすぎます（8MB以下）", http.StatusRequestEntityTooLarge)
			} else {
				http.Error(w, "写真を読み取れませんでした", http.StatusBadRequest)
			}
			return
		}
		rec, err := recipe.UpdatePhoto(dir, r.PathValue("name"), strings.Trim(r.Header.Get("If-Match"), "\""), body)
		if errors.Is(err, recipe.ErrPhoto) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if recipeWriteError(w, err) {
			return
		}
		writeJSON(w, rec)
	})
}

package api

import (
	"encoding/json"
	"errors"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/recipe"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"net/url"
)

func registerRecipeWrites(mux *http.ServeMux, dir string) {
	decode := func(w http.ResponseWriter, r *http.Request, v any) bool {
		media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || media != "application/json" {
			http.Error(w, "application/json required", 415)
			return false
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			u, e := url.Parse(origin)
			if e != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host != r.Host {
				http.Error(w, "origin rejected", 403)
				return false
			}
		}

		d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		d.DisallowUnknownFields()
		if d.Decode(v) != nil || d.Decode(new(any)) != io.EOF {
			http.Error(w, "入力を確認してください（1MB以内）", 400)
			return false
		}
		return true
	}
	mux.HandleFunc("POST /api/v1/recipes", func(w http.ResponseWriter, r *http.Request) {
		var input recipe.Editable
		if !decode(w, r, &input) {
			return
		}
		rec, err := recipe.Create(dir, r.Header.Get("Idempotency-Key"), input)
		if recipeWriteError(w, err) {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, rec)
	})
	mux.HandleFunc("PUT /api/v1/recipes/{name...}", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			recipe.Editable
			Version string `json:"version"`
		}
		if !decode(w, r, &input) {
			return
		}
		rec, err := recipe.Update(dir, r.PathValue("name"), input.Version, input.Editable)
		if recipeWriteError(w, err) {
			return
		}
		writeJSON(w, rec)
	})
}
func recipeWriteError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, recipe.ErrConflict):
		http.Error(w, err.Error(), http.StatusConflict)
	case errors.Is(err, recipe.ErrInvalid):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, fs.ErrNotExist):
		http.Error(w, "レシピが見つかりません", http.StatusNotFound)
	default:
		http.Error(w, "レシピを保存できませんでした。もう一度試してください", http.StatusInternalServerError)
	}
	return true
}

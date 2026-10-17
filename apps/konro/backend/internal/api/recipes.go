package api

import (
	"errors"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/recipe"
	"io/fs"
	"net/http"
)

func registerRecipeWrites(mux *http.ServeMux, dir string) {

	mux.HandleFunc("POST /api/v1/recipes", func(w http.ResponseWriter, r *http.Request) {
		var input recipe.Editable
		if !decodeJSON(w, r, &input, 1<<20) {
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
		if !decodeJSON(w, r, &input, 1<<20) {
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

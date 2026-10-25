package api

import (
	"errors"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/mealplan"
	"net/http"
	"os"
)

func registerMealplans(mux *http.ServeMux, dir string) *mealplan.Store {
	store := mealplan.New(dir)
	respond := func(w http.ResponseWriter, v any, e error) {
		if e != nil {
			status := 500
			if errors.Is(e, mealplan.ErrConflict) {
				status = 409
			} else if errors.Is(e, mealplan.ErrInvalid) {
				status = 400
			} else if errors.Is(e, os.ErrNotExist) {
				status = 404
			}
			http.Error(w, e.Error(), status)
			return
		}
		writeJSON(w, v)
	}
	mux.HandleFunc("GET /api/v1/meal-plans", func(w http.ResponseWriter, r *http.Request) { v, e := store.List(); respond(w, v, e) })
	mux.HandleFunc("POST /api/v1/meal-plans", func(w http.ResponseWriter, r *http.Request) {
		var in mealplan.Change
		if !decodeJSON(w, r, &in, 1<<20) {
			return
		}
		v, e := store.Create(r.Header.Get("Idempotency-Key"), in)
		respond(w, v, e)
	})
	mux.HandleFunc("GET /api/v1/meal-plans/{id}", func(w http.ResponseWriter, r *http.Request) { v, e := store.Get(r.PathValue("id")); respond(w, v, e) })
	mux.HandleFunc("PATCH /api/v1/meal-plans/{id}", func(w http.ResponseWriter, r *http.Request) {
		var in mealplan.Change
		if !decodeJSON(w, r, &in, 1<<20) {
			return
		}
		v, e := store.Update(r.PathValue("id"), r.Header.Get("Idempotency-Key"), in)
		respond(w, v, e)
	})
	return store
}

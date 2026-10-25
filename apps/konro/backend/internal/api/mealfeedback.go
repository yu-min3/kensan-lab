package api

import (
	"errors"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/mealfeedback"
	"net/http"
	"os"
)

func registerMealFeedback(mux *http.ServeMux, s *mealfeedback.Store) {
	respond := func(w http.ResponseWriter, v any, e error) {
		if e != nil {
			status := 400
			if errors.Is(e, mealfeedback.ErrConflict) {
				status = 409
			} else if errors.Is(e, os.ErrNotExist) {
				status = 404
			}
			http.Error(w, e.Error(), status)
			return
		}
		writeJSON(w, v)
	}
	mux.HandleFunc("GET /api/v1/meal-feedback", func(w http.ResponseWriter, r *http.Request) { v, e := s.Events(); respond(w, v, e) })
	mux.HandleFunc("POST /api/v1/meal-feedback", func(w http.ResponseWriter, r *http.Request) {
		var in mealfeedback.Event
		if !decodeJSON(w, r, &in, 16384) {
			return
		}
		v, e := s.Record(in)
		respond(w, v, e)
	})
	mux.HandleFunc("GET /api/v1/cook-history/{id}", func(w http.ResponseWriter, r *http.Request) { v, e := s.GetCook(r.PathValue("id")); respond(w, v, e) })
	mux.HandleFunc("GET /api/v1/cook-history", func(w http.ResponseWriter, r *http.Request) { v, e := s.History(); respond(w, v, e) })
	mux.HandleFunc("PUT /api/v1/cook-history/{id}", func(w http.ResponseWriter, r *http.Request) {
		var in mealfeedback.CookRecord
		if !decodeJSON(w, r, &in, 1<<20) {
			return
		}
		if in.ID != r.PathValue("id") {
			http.Error(w, "記録IDが一致しません", 400)
			return
		}
		v, e := s.Cook(in)
		respond(w, v, e)
	})
	mux.HandleFunc("GET /api/v1/meal-preferences", func(w http.ResponseWriter, r *http.Request) { v, e := s.Preferences(); respond(w, v, e) })
	mux.HandleFunc("PUT /api/v1/meal-preferences", func(w http.ResponseWriter, r *http.Request) {
		var in mealfeedback.Preferences
		if !decodeJSON(w, r, &in, 32768) {
			return
		}
		v, e := s.SavePreferences(in)
		respond(w, v, e)
	})
}

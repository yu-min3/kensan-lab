package api

import (
	"errors"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/planner"
	"net/http"
	"os"
)

func registerPlanner(mux *http.ServeMux, s *planner.Service) {
	respond := func(w http.ResponseWriter, v any, e error) {
		if e != nil {
			status := 400
			if errors.Is(e, os.ErrNotExist) {
				status = 404
			}
			if errors.Is(e, planner.ErrConflict) {
				status = 409
			}
			http.Error(w, e.Error(), status)
			return
		}
		writeJSON(w, v)
	}
	mux.HandleFunc("POST /api/v1/menu-proposals", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Conditions planner.Conditions `json:"conditions"`
			Mode       string             `json:"mode"`
		}
		if !decodeJSON(w, r, &in, 1<<20) {
			return
		}
		v, e := s.Create(r.Context(), in.Conditions, in.Mode)
		respond(w, v, e)
	})
	mux.HandleFunc("GET /api/v1/menu-proposals/{id}", func(w http.ResponseWriter, r *http.Request) {
		v, e := s.Get(r.Context(), r.PathValue("id"))
		respond(w, v, e)
	})
	mux.HandleFunc("POST /api/v1/menu-proposals/{id}/{action}", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Revision int    `json:"revision"`
			MenuID   string `json:"menuId"`
		}
		if !decodeJSON(w, r, &in, 4096) {
			return
		}
		if r.PathValue("action") == "adopt" {
			v, e := s.Adopt(r.PathValue("id"), in.Revision, in.MenuID, r.Header.Get("Idempotency-Key"))
			respond(w, v, e)
			return
		}
		v, e := s.Action(r.Context(), r.PathValue("id"), in.Revision, r.PathValue("action"))
		respond(w, v, e)
	})
}

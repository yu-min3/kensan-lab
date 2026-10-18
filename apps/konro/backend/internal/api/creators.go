package api

import (
	"errors"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/creatorprefs"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/discovery"
	"net/http"
	"os"
)

func registerCreators(mux *http.ServeMux, store *creatorprefs.Store, service *discovery.Service) {
	result := func(w http.ResponseWriter, value any, err error) {
		w.Header().Set("Cache-Control", "no-store")
		if err != nil {
			code := http.StatusBadRequest
			if errors.Is(err, creatorprefs.ErrConflict) {
				code = http.StatusConflict
			}
			if errors.Is(err, os.ErrNotExist) {
				code = http.StatusNotFound
			}
			http.Error(w, err.Error(), code)
			return
		}
		writeJSON(w, value)
	}
	mux.HandleFunc("GET /api/v1/creators", func(w http.ResponseWriter, r *http.Request) { value, err := store.Get(); result(w, value, err) })
	mux.HandleFunc("POST /api/v1/creators/resolve", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			URL string `json:"url"`
		}
		if !decodeJSON(w, r, &in, 8192) {
			return
		}
		value, err := service.ResolveCreator(r.Context(), in.URL)
		result(w, value, err)
	})
	mux.HandleFunc("POST /api/v1/creators", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			URL      string `json:"url"`
			Revision string `json:"revision"`
		}
		if !decodeJSON(w, r, &in, 8192) {
			return
		}
		creator, err := service.ResolveCreator(r.Context(), in.URL)
		if err != nil {
			result(w, nil, err)
			return
		}
		value, err := store.Add(in.Revision, creator)
		result(w, value, err)
	})
	mux.HandleFunc("DELETE /api/v1/creators/{id}", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Revision string `json:"revision"`
		}
		if !decodeJSON(w, r, &in, 8192) {
			return
		}
		value, err := store.Remove(in.Revision, r.PathValue("id"))
		result(w, value, err)
	})
	mux.HandleFunc("PATCH /api/v1/creators/{id}", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Revision   string `json:"revision"`
			Additional bool   `json:"additional"`
		}
		if !decodeJSON(w, r, &in, 8192) {
			return
		}
		value, err := store.Additional(in.Revision, r.PathValue("id"), in.Additional)
		result(w, value, err)
	})
	mux.HandleFunc("POST /api/v1/creators/undo", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Revision string `json:"revision"`
			Token    string `json:"token"`
		}
		if !decodeJSON(w, r, &in, 8192) {
			return
		}
		value, err := store.Undo(in.Revision, in.Token)
		result(w, value, err)
	})
}

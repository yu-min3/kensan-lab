package api

import (
	"errors"
	"net/http"
	"os"

	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/agentclient"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/importer"
)

type Option func(*options)
type options struct{ agent agentclient.Gateway }

func WithAgent(agent agentclient.Gateway) Option { return func(o *options) { o.agent = agent } }
func registerDrafts(mux *http.ServeMux, s *importer.Service) {

	result := func(w http.ResponseWriter, d *importer.Draft, e error) {
		if e != nil {
			code := 400
			if errors.Is(e, os.ErrNotExist) {
				code = 404
			}
			if errors.Is(e, importer.ErrConflict) {
				code = 409
			}
			http.Error(w, e.Error(), code)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, d)
	}
	mux.HandleFunc("POST /api/v1/import/drafts", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			URL  string `json:"url"`
			Text string `json:"text"`
		}
		if !decodeJSON(w, r, &in, 256<<10) {
			return
		}
		d, e := s.Create(r.Context(), in.URL, in.Text)
		result(w, d, e)
	})
	mux.HandleFunc("GET /api/v1/import/drafts/{id}", func(w http.ResponseWriter, r *http.Request) {
		d, e := s.Get(r.Context(), r.PathValue("id"))
		result(w, d, e)
	})
	mux.HandleFunc("PATCH /api/v1/import/drafts/{id}", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Revision int              `json:"revision"`
			Content  importer.Content `json:"content"`
		}
		if !decodeJSON(w, r, &in, 256<<10) {
			return
		}
		d, e := s.Edit(r.PathValue("id"), in.Revision, in.Content)
		result(w, d, e)
	})
	mux.HandleFunc("POST /api/v1/import/drafts/{id}/{action}", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Revision int    `json:"revision"`
			Mode     string `json:"mode,omitempty"`
		}
		if !decodeJSON(w, r, &in, 256<<10) {
			return
		}
		var d *importer.Draft
		var e error
		id := r.PathValue("id")
		switch r.PathValue("action") {
		case "normalize":
			d, e = s.Start(r.Context(), id, in.Revision, in.Mode)
		case "cancel", "retry":
			d, e = s.Action(r.Context(), id, in.Revision, r.PathValue("action"))
		case "manual":
			d, e = s.Manual(id, in.Revision)
		case "save":
			d, e = s.Save(id, in.Revision)
		default:
			http.NotFound(w, r)
			return
		}
		result(w, d, e)
	})
}

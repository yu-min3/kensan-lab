package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/agentclient"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/importer"
)

type Option func(*options)
type options struct{ agent agentclient.Gateway }

func WithAgent(agent agentclient.Gateway) Option { return func(o *options) { o.agent = agent } }
func registerDrafts(mux *http.ServeMux, s *importer.Service) {
	decode := func(w http.ResponseWriter, r *http.Request, v any) bool {
		if r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "application/json required", 415)
			return false
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			u, e := url.Parse(origin)
			if e != nil || u.Host != r.Host {
				http.Error(w, "origin rejected", 403)
				return false
			}
		}
		d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10))
		d.DisallowUnknownFields()
		if d.Decode(v) != nil || d.Decode(new(any)) != io.EOF {
			http.Error(w, "invalid JSON", 400)
			return false
		}
		return true
	}
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
		if !decode(w, r, &in) {
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
		if !decode(w, r, &in) {
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
		if !decode(w, r, &in) {
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
		case "save":
			d, e = s.Save(id, in.Revision)
		default:
			http.NotFound(w, r)
			return
		}
		result(w, d, e)
	})
}

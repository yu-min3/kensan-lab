package api

import (
	"errors"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/taxonomy"
	"net/http"
)

func registerTaxonomy(mux *http.ServeMux, dir string) {
	store := taxonomy.New(dir)
	mux.HandleFunc("GET /api/v1/tag-taxonomy", func(w http.ResponseWriter, r *http.Request) {
		v, e := store.Get()
		if e != nil {
			http.Error(w, "タグ分類を読み込めません。元のファイルは変更していません", 500)
			return
		}
		writeJSON(w, v)
	})
	mux.HandleFunc("PUT /api/v1/tag-taxonomy", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Revision    string            `json:"revision"`
			Assignments map[string]string `json:"assignments"`
		}
		if !decodeJSON(w, r, &in, 1<<20) {
			return
		}
		v, e := store.Put(in.Revision, in.Assignments)
		if e != nil {
			status := 500
			if errors.Is(e, taxonomy.ErrConflict) {
				status = 409
			} else if errors.Is(e, taxonomy.ErrInvalid) {
				status = 400
			}
			http.Error(w, e.Error(), status)
			return
		}
		writeJSON(w, v)
	})
}

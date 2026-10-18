package api

import (
	"context"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/discovery"
	"net/http"
	"time"
)

func registerDiscovery(mux *http.ServeMux, service *discovery.Service) {
	mux.HandleFunc("GET /api/v1/discovery/preview", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		info, err := service.Preview(r.Context(), r.URL.Query().Get("url"))
		if err != nil {
			http.Error(w, "出典情報を取得できませんでした", http.StatusServiceUnavailable)
			return
		}
		writeJSON(w, info)
	})
	mux.HandleFunc("GET /api/v1/discovery/sites", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, discovery.Sites) })
	mux.HandleFunc("GET /api/v1/discovery/search", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		var result discovery.Response
		var err error
		if creator := r.URL.Query().Get("creator"); creator != "" {
			result, err = service.SearchCreator(ctx, r.URL.Query().Get("q"), creator)
		} else {
			result, err = service.Search(ctx, r.URL.Query().Get("q"), r.URL.Query().Get("site"))
		}
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		writeJSON(w, result)
	})
}

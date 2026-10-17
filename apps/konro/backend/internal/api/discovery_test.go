package api

import (
	"context"
	"encoding/json"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/discovery"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/webpage"
	"net/http/httptest"
	"testing"
)

func TestDiscoveryAPIUsesKnownResultsWithoutWrites(t *testing.T) {
	calls := 0
	h := New(t.TempDir(), "", WithSearchFetcher(func(ctx context.Context, u string) (webpage.Page, error) {
		calls++
		return webpage.Page{URL: u, HTML: `<div class="delish-search"><a href="/recipes/987654">魚料理</a><a href="http://127.0.0.1/private">不正</a></div>`}, nil
	}))
	for _, tc := range []struct {
		path   string
		status int
	}{{"/api/v1/discovery/search?q=&site=delish", 400}, {"/api/v1/discovery/search?q=test&site=unknown", 400}, {"/api/v1/discovery/search?q=test&site=delish", 200}} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", tc.path, nil))
		if w.Code != tc.status {
			t.Fatal(w.Code, w.Body.String())
		}
		if tc.status == 200 {
			var r discovery.Response
			if json.Unmarshal(w.Body.Bytes(), &r) != nil || len(r.Results) != 1 || r.Results[0].URL != "https://delishkitchen.tv/recipes/987654" || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal(w.Body.String())
			}
		}
	}
	if calls != 1 {
		t.Fatal("invalid queries reached public fetch", calls)
	}
}

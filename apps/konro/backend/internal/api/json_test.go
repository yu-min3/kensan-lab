package api

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestJSONWriteBoundariesAgree(t *testing.T) {
	for _, endpoint := range []struct {
		path, body string
		ok         int
	}{
		{"/api/v1/recipes", `{"title":"魚"}`, 201},
		{"/api/v1/import/drafts", `{"text":"魚を煮る"}`, 200},
	} {
		for _, tc := range []struct {
			name, media, origin, suffix string
			status                      int
		}{
			{"charset", "application/json; charset=utf-8", "http://konro.example", "", endpoint.ok},
			{"bad scheme", "application/json", "ftp://konro.example", "", 403},
			{"cross origin", "application/json", "http://other.example", "", 403},
			{"non json", "text/plain", "", "", 415},
			{"trailing value", "application/json", "", " {}", 400},
		} {
			t.Run(endpoint.path+"/"+tc.name, func(t *testing.T) {
				h := New(t.TempDir(), "")
				r := httptest.NewRequest("POST", "http://konro.example"+endpoint.path, strings.NewReader(endpoint.body+tc.suffix))
				r.Header.Set("Content-Type", tc.media)
				if tc.origin != "" {
					r.Header.Set("Origin", tc.origin)
				}
				w := httptest.NewRecorder()
				h.ServeHTTP(w, r)
				if w.Code != tc.status {
					t.Fatalf("status %d: %s", w.Code, w.Body.String())
				}
			})
		}
	}
}

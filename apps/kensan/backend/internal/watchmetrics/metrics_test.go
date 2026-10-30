package watchmetrics

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCollectPeriodsPaginationAndPreserve(t *testing.T) {
	now := time.Date(2026, 11, 2, 0, 30, 0, 0, JST)
	fail := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail {
			w.WriteHeader(503)
			return
		}
		if r.URL.Path == "/api/articles" {
			if r.URL.Query().Get("page") == "1" {
				w.Write([]byte(`{"articles":[{"id":1,"title":"月末公開","published_at":"2026-10-31T15:00:00Z","path":"/writer/articles/a"}],"next_page":2,"total_count":2}`))
			} else {
				w.Write([]byte(`{"articles":[{"id":2,"title":"先月公開","published_at":"2026-10-31T14:59:59Z","path":"/writer/articles/b"}],"next_page":null,"total_count":2}`))
			}
			return
		}
		w.Write([]byte(`[{"number":1,"title":"マージ","html_url":"https://github.com/sample/repo/pull/1","created_at":"2026-11-01T13:00:00Z","updated_at":"2026-11-01T15:10:00Z","merged_at":"2026-11-01T15:00:00Z"},{"number":2,"title":"却下","html_url":"https://github.com/sample/repo/pull/2","created_at":"2026-11-01T13:00:00Z","updated_at":"2026-11-01T15:10:00Z","merged_at":null},{"number":3,"title":"先週","html_url":"https://github.com/sample/repo/pull/3","created_at":"2026-11-01T13:00:00Z","updated_at":"2026-11-01T15:10:00Z","merged_at":"2026-11-01T14:59:59Z"}]`))
	}))
	defer server.Close()
	c := Collector{server.Client(), server.URL, server.URL}
	root := t.TempDir()
	if e := c.Refresh(context.Background(), root, "sample/repo", "writer", now); e != nil {
		t.Fatal(e)
	}
	v, e := Read(root, now)
	if e != nil {
		t.Fatal(e)
	}
	if len(v.Publications.Data.Items) != 1 || !v.Publications.CurrentPeriod || v.Publications.Stale {
		t.Fatalf("articles %+v", v.Publications)
	}
	d := v.Development.Data
	if len(d.Items) != 1 || d.Days[0] != 1 || d.MedianHours == nil || *d.MedianHours != 2 || d.Week != "2026-11-02" {
		t.Fatalf("development %+v", d)
	}
	fail = true
	if e = c.Refresh(context.Background(), root, "sample/repo", "writer", now.Add(time.Hour)); e == nil {
		t.Fatal("expected provider failure")
	}
	v, e = Read(root, now.Add(time.Hour))
	if e != nil || len(v.Publications.Data.Items) != 1 || !v.Publications.Stale || !v.Development.Stale {
		t.Fatalf("previous values lost: %+v %v", v, e)
	}
	v, _ = Read(root, now.AddDate(0, 1, 0))
	if v.Publications.CurrentPeriod || v.Development.CurrentPeriod {
		t.Fatal("old-period values passed as current")
	}
	b1, _ := os.ReadFile(filepath.Join(root, Path))
	Read(root, now)
	b2, _ := os.ReadFile(filepath.Join(root, Path))
	if string(b1) != string(b2) {
		t.Fatal("GET-like read changed snapshot")
	}
}
func TestIncompleteArticlesNotZero(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"articles": []any{}, "total_count": 10, "next_page": nil})
	}))
	defer server.Close()
	c := Collector{server.Client(), server.URL, server.URL}
	if _, e := c.publications(context.Background(), "writer", time.Now()); e == nil {
		t.Fatal("truncated result counted as zero")
	}
}
func TestAbsentMetricsAndSymlink(t *testing.T) {
	root := t.TempDir()
	v, e := Read(root, time.Now())
	if e != nil || v.Publications.Data != nil {
		t.Fatalf("absent %+v %v", v, e)
	}
	outside := t.TempDir()
	os.MkdirAll(filepath.Join(root, "feeds"), 0700)
	os.Symlink(outside, filepath.Join(root, "feeds/watch-metrics"))
	if e = NewCollector().Refresh(context.Background(), root, "sample/repo", "writer", time.Now()); e == nil {
		t.Fatal("external symlink accepted")
	}
}

func TestNullableZennTotal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"articles":[],"total_count":null,"next_page":null}`))
	}))
	defer server.Close()
	c := Collector{server.Client(), server.URL, server.URL}
	v, e := c.publications(context.Background(), "writer", time.Now())
	if e != nil || len(v.Items) != 0 {
		t.Fatalf("legitimate null total: %+v %v", v, e)
	}
}

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newStaticHandler(t *testing.T) http.Handler {
	t.Helper()
	dir := t.TempDir()
	write := func(rel, content string) {
		abs := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("index.html", "<!doctype html><title>kensan</title>")
	write("assets/index-abc123.css", "body{color:red}")

	api := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ok":true}`)
	})
	return WithStatic(api, dir)
}

func doGet(t *testing.T, h http.Handler, path string) *http.Response {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec.Result()
}

// 実在しない /assets/* に index.html を返すと、デプロイ直後に古い index.html を掴んだ
// ブラウザが HTML を CSS/JS として受け取り、スタイル全滅・真っ白になる。
func TestStaticMissingAssetReturns404(t *testing.T) {
	resp := doGet(t, newStaticHandler(t), "/assets/index-OLDHASH.css")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(body), "<!doctype html>") {
		t.Fatalf("index.html が返っている: %q", body)
	}
}

func TestStaticExistingAssetIsImmutable(t *testing.T) {
	resp := doGet(t, newStaticHandler(t), "/assets/index-abc123.css")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Fatalf("Cache-Control = %q", got)
	}
}

// クライアントルートは従来どおり SPA fallback。ただし index.html はキャッシュさせない。
func TestStaticClientRouteFallsBackToIndex(t *testing.T) {
	resp := doGet(t, newStaticHandler(t), "/tasks")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "<!doctype html>") {
		t.Fatalf("index.html が返っていない: %q", body)
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("Cache-Control = %q, want no-cache", got)
	}
}

func TestStaticAPIPassesThrough(t *testing.T) {
	resp := doGet(t, newStaticHandler(t), "/api/anything")
	body, _ := io.ReadAll(resp.Body)
	if string(body) != `{"ok":true}` {
		t.Fatalf("api handler に届いていない: %q", body)
	}
}

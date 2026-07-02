package api

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestReviews(t *testing.T) {
	ts, root := newTestServer(t)
	write := func(rel, content string) {
		abs := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("reviews/2026/W22.html", "<html><body>週次レビュー</body></html>")
	write("reviews/daily/2026-06-05.html", "<html><body>日次</body></html>")
	write("reviews/2026/05-monthly.md", "---\ntype: review\n---\n月次")
	write("reviews/2026/notes.txt", "対象外の拡張子")

	var list struct {
		Reviews []struct {
			Path string `json:"path"`
			Kind string `json:"kind"`
		} `json:"reviews"`
		Total int `json:"total"`
	}
	if code := getJSON(t, ts.URL+"/api/v1/reviews", &list); code != 200 {
		t.Fatalf("reviews list: %d", code)
	}
	if list.Total != 3 {
		t.Fatalf("want 3 reviews (.txt excluded), got %d: %+v", list.Total, list.Reviews)
	}
	kinds := map[string]string{}
	for _, r := range list.Reviews {
		kinds[r.Path] = r.Kind
	}
	if kinds["reviews/2026/W22.html"] != "weekly" ||
		kinds["reviews/daily/2026-06-05.html"] != "daily" ||
		kinds["reviews/2026/05-monthly.md"] != "monthly" {
		t.Errorf("kind classification wrong: %+v", kinds)
	}

	// 配信: HTML がそのまま返り Content-Type が text/html
	resp, err := http.Get(ts.URL + "/api/v1/reviews/2026/W22.html")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || string(body) != "<html><body>週次レビュー</body></html>" {
		t.Errorf("serve: code=%d body=%q", resp.StatusCode, body)
	}
	if ct := resp.Header.Get("Content-Type"); ct == "" || ct[:9] != "text/html" {
		t.Errorf("content-type: %q", ct)
	}

	// reviews/ 外への脱出は拒否
	for _, p := range []string{"../todo.md", "..%2Ftodo.md"} {
		resp, err := http.Get(ts.URL + "/api/v1/reviews/" + p)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode == 200 {
			t.Errorf("traversal not rejected: %s -> %d", p, resp.StatusCode)
		}
	}
}

// symlink 経由の workspace 外配信と、一覧への symlink / 隠しディレクトリ混入を防ぐ
func TestReviewsSymlinkSafety(t *testing.T) {
	ts, root := newTestServer(t)
	if err := os.MkdirAll(filepath.Join(root, "reviews", ".obsidian"), 0o755); err != nil {
		t.Fatal(err)
	}
	// 隠しディレクトリ内の HTML は一覧に出ない
	if err := os.WriteFile(filepath.Join(root, "reviews", ".obsidian", "cache.html"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// workspace 外を指す symlink
	outside := filepath.Join(t.TempDir(), "secret.html")
	if err := os.WriteFile(outside, []byte("外部ファイル"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "reviews", "evil.html")); err != nil {
		t.Fatal(err)
	}

	var list struct{ Total int }
	getJSON(t, ts.URL+"/api/v1/reviews", &list)
	if list.Total != 0 {
		t.Errorf("symlink/hidden entries leaked into list: %d", list.Total)
	}

	resp, err := http.Get(ts.URL + "/api/v1/reviews/evil.html")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode == 200 {
		t.Errorf("symlink escape served content: %q", body)
	}
}

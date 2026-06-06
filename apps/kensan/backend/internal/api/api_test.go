package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func newTestServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	root := t.TempDir()
	write := func(rel, content string) {
		abs := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("todo.md", "---\ntype: todo\nupdated: 2026-01-01\n---\n## Now\n\n- [ ] タスクA\n")
	write("projects/demo/README.md", "---\ntype: project\nstatus: active\n---\n\n## タスク\n\n- [ ] タスクB\n\n## ログ\n")
	write("notes/sample.md", "---\ntype: note\ntags: [go]\nstatus: active\n---\n\n## 概要\nGo の話。\n")
	write("daily/2026/06/05.md", "---\ntype: daily\n---\n# 2026-06-05\n\n## 日記\nテスト日記\n")

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	ts := httptest.NewServer(New(root, log).Handler())
	t.Cleanup(ts.Close)
	return ts, root
}

func getJSON(t *testing.T, url string, out any) int {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatal(err)
		}
	}
	return resp.StatusCode
}

func doJSON(t *testing.T, method, url string, body any, out any) int {
	t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(method, url, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if out != nil {
		_ = json.NewDecoder(resp.Body).Decode(out)
	}
	return resp.StatusCode
}

func TestReadEndpoints(t *testing.T) {
	ts, _ := newTestServer(t)

	var files struct {
		Files []struct {
			Path string `json:"path"`
			Meta struct {
				Type string `json:"type"`
			} `json:"meta"`
		} `json:"files"`
		Total int `json:"total"`
	}
	if code := getJSON(t, ts.URL+"/api/v1/files", &files); code != 200 {
		t.Fatalf("files: %d", code)
	}
	if files.Total != 4 {
		t.Errorf("want 4 files, got %d", files.Total)
	}
	if code := getJSON(t, ts.URL+"/api/v1/files?type=note&tag=go", &files); code != 200 || files.Total != 1 {
		t.Errorf("filtered files: code=%d total=%d", 200, files.Total)
	}

	var detail struct {
		Content string `json:"content"`
	}
	if code := getJSON(t, ts.URL+"/api/v1/files/notes/sample.md", &detail); code != 200 {
		t.Fatalf("detail: %d", code)
	}
	if detail.Content == "" {
		t.Error("empty content")
	}
	if code := getJSON(t, ts.URL+"/api/v1/files/notes/none.md", nil); code != 404 {
		t.Errorf("missing file: want 404, got %d", code)
	}

	if code := getJSON(t, ts.URL+"/api/v1/daily?date=2026-06-05", nil); code != 200 {
		t.Errorf("daily: %d", code)
	}
	if code := getJSON(t, ts.URL+"/api/v1/daily?date=1999-01-01", nil); code != 404 {
		t.Errorf("daily missing: want 404, got %d", code)
	}

	var board struct{ Today, Stock []json.RawMessage }
	if code := getJSON(t, ts.URL+"/api/v1/tasks", &board); code != 200 {
		t.Fatalf("tasks: %d", code)
	}
	if len(board.Today) != 1 || len(board.Stock) != 1 {
		t.Errorf("board: today=%d stock=%d", len(board.Today), len(board.Stock))
	}

	var search struct{ Total int }
	if code := getJSON(t, ts.URL+"/api/v1/search?q=日記", &search); code != 200 || search.Total == 0 {
		t.Errorf("search: code=%d total=%d", 200, search.Total)
	}
}

func TestWriteAndConflict(t *testing.T) {
	ts, _ := newTestServer(t)

	// create
	code := doJSON(t, "POST", ts.URL+"/api/v1/files",
		map[string]string{"path": "inbox/new.md", "content": "---\ntype: memo\n---\nメモ"}, nil)
	if code != 201 {
		t.Fatalf("create: %d", code)
	}
	// 二重作成は 409
	code = doJSON(t, "POST", ts.URL+"/api/v1/files",
		map[string]string{"path": "inbox/new.md", "content": "x"}, nil)
	if code != 409 {
		t.Errorf("duplicate create: want 409, got %d", code)
	}

	// update with stale baseMtime → 409
	var detail struct {
		Doc struct {
			MTime string `json:"mtime"`
		} `json:"doc"`
	}
	getJSON(t, ts.URL+"/api/v1/files/inbox/new.md", &detail)
	code = doJSON(t, "PUT", ts.URL+"/api/v1/files/inbox/new.md",
		map[string]string{"content": "更新", "baseMtime": detail.Doc.MTime}, nil)
	if code != 200 {
		t.Fatalf("update: %d", code)
	}
	// 古い mtime のままもう一度 → conflict
	code = doJSON(t, "PUT", ts.URL+"/api/v1/files/inbox/new.md",
		map[string]string{"content": "競合", "baseMtime": detail.Doc.MTime}, nil)
	if code != 409 {
		t.Errorf("stale update: want 409, got %d", code)
	}

	// delete
	req, _ := http.NewRequest("DELETE", ts.URL+"/api/v1/files/inbox/new.md", nil)
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode != 200 {
		t.Errorf("delete: %d", resp.StatusCode)
	}
	resp.Body.Close()

	// path traversal は 400 系で拒否
	code = doJSON(t, "POST", ts.URL+"/api/v1/files",
		map[string]string{"path": "../escape.md", "content": "x"}, nil)
	if code != 400 {
		t.Errorf("traversal: want 400, got %d", code)
	}
}

func TestTaskMoveAPI(t *testing.T) {
	ts, root := newTestServer(t)

	// board API から実際の位置を取得（手で行番号を数えない）
	var board struct {
		Stock []struct {
			File string `json:"file"`
			Line int    `json:"line"`
			Text string `json:"text"`
		} `json:"stock"`
	}
	getJSON(t, ts.URL+"/api/v1/tasks", &board)
	if len(board.Stock) != 1 || board.Stock[0].Text != "タスクB" {
		t.Fatalf("unexpected board: %+v", board)
	}
	srcLine := board.Stock[0].Line

	// ストック → 今日
	var res struct {
		Task struct {
			File string `json:"file"`
			Line int    `json:"line"`
			Text string `json:"text"`
		} `json:"task"`
	}
	code := doJSON(t, "POST", ts.URL+"/api/v1/tasks/move", map[string]any{
		"file": "projects/demo/README.md", "line": srcLine, "text": "タスクB", "to": "today",
	}, &res)
	if code != 200 {
		t.Fatalf("move: %d", code)
	}
	if res.Task.File != "todo.md" {
		t.Fatalf("unexpected dest: %+v", res.Task)
	}

	// 完了 → daily へ
	code = doJSON(t, "PATCH", ts.URL+"/api/v1/tasks", map[string]any{
		"file": res.Task.File, "line": res.Task.Line, "text": "タスクB", "state": "done",
	}, nil)
	if code != 200 {
		t.Fatalf("state: %d", code)
	}
	code = doJSON(t, "POST", ts.URL+"/api/v1/tasks/move", map[string]any{
		"file": res.Task.File, "line": res.Task.Line, "text": "タスクB", "to": "daily", "date": "2026-06-06",
	}, &res)
	if code != 200 {
		t.Fatalf("move to daily: %d", code)
	}

	content, err := os.ReadFile(filepath.Join(root, "daily", "2026", "06", "06.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(content, []byte("- [x] タスクB")) {
		t.Errorf("daily missing completed task:\n%s", content)
	}

	// 既に動いた行への stale な操作は 409
	code = doJSON(t, "POST", ts.URL+"/api/v1/tasks/move", map[string]any{
		"file": "projects/demo/README.md", "line": srcLine, "text": "タスクB", "to": "today",
	}, nil)
	if code != 409 {
		t.Errorf("stale move: want 409, got %d", code)
	}

	// frontmatter の updated が触られている
	todo, _ := os.ReadFile(filepath.Join(root, "todo.md"))
	if !bytes.Contains(todo, []byte("updated: 20")) || bytes.Contains(todo, []byte("updated: 2026-01-01")) {
		t.Errorf("updated not touched:\n%s", todo)
	}
}

package workspace

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScanAndCache(t *testing.T) {
	root := t.TempDir()
	write := func(rel, content string) string {
		abs := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return abs
	}
	write("notes/a.md", "---\ntype: note\ntags: [x]\n---\nbody")
	write("daily/2026/06/06.md", "---\ntype: daily\n---\n# 2026-06-06")
	write("ignored.txt", "not markdown")
	write(".obsidian/cache.md", "hidden dir should be skipped")
	write("node_modules/pkg/readme.md", "should be skipped")

	ws := New(root)
	docs, err := ws.Scan()
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 2 {
		t.Fatalf("want 2 docs, got %d: %+v", len(docs), docs)
	}
	if docs[1].Meta.Type != "note" || docs[0].Meta.Type != "daily" {
		t.Errorf("unexpected metas: %+v", docs)
	}

	// 変更が次の Scan で必ず反映される（stale しない）
	abs := write("notes/a.md", "---\ntype: note\nstatus: archived\ntags: [x]\n---\nbody2")
	// mtime 解像度対策で明示的に進める
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(abs, future, future); err != nil {
		t.Fatal(err)
	}
	docs, _ = ws.Scan()
	var found bool
	for _, d := range docs {
		if d.Path == "notes/a.md" {
			found = true
			if d.Meta.Status != "archived" {
				t.Errorf("change not picked up: %+v", d.Meta)
			}
		}
	}
	if !found {
		t.Fatal("notes/a.md missing")
	}

	// 削除も次の Scan で消える
	if err := os.Remove(abs); err != nil {
		t.Fatal(err)
	}
	docs, _ = ws.Scan()
	if len(docs) != 1 {
		t.Errorf("want 1 doc after delete, got %d", len(docs))
	}
}

func TestAbsRejectsTraversal(t *testing.T) {
	ws := New(t.TempDir())
	if _, err := ws.Abs("../etc/passwd"); err == nil {
		t.Error("path traversal not rejected")
	}
	if _, err := ws.Abs("notes/ok.md"); err != nil {
		t.Errorf("valid path rejected: %v", err)
	}
}

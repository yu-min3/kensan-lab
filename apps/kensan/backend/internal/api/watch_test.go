package api

import (
	"github.com/yu-min3/kensan-lab/apps/kensan/backend/internal/projectwatch"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWatchReadOnlyAndPrivateDiary(t *testing.T) {
	ts, root := newTestServer(t)
	p := filepath.Join(root, "projects", "watch-example", "README.md")
	if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
		t.Fatal(e)
	}
	content := "---\nstatus: active\n---\n## ログ\n- 2026-10-30: 実機で確認した\n## マイルストーン\n- [ ] 次を試す\n"
	if e := os.WriteFile(p, []byte(content), 0600); e != nil {
		t.Fatal(e)
	}
	diary := filepath.Join(root, "daily", "2026", "10", "30.md")
	os.MkdirAll(filepath.Dir(diary), 0700)
	os.WriteFile(diary, []byte("PRIVATE DIARY"), 0600)
	var v projectwatch.View
	if c := getJSON(t, ts.URL+"/api/v1/watch", &v); c != 200 {
		t.Fatalf("watch status %d", c)
	}
	found := false
	for _, p := range v.Snapshot.Projects {
		if p.Name == "watch-example" {
			found = true
			if p.Date != "2026-10-30" || !strings.Contains(p.Text, "実機") {
				t.Fatalf("bad project: %+v", p)
			}
		}
	}
	if !found {
		t.Fatal("missing project")
	}
	for _, s := range v.Snapshot.Sources {
		if strings.Contains(s.Text, "PRIVATE DIARY") {
			t.Fatal("diary leaked")
		}
	}
	if _, e := os.Stat(filepath.Join(root, projectwatch.StatePath)); !os.IsNotExist(e) {
		t.Fatal("GET wrote batch state")
	}
	b, _ := os.ReadFile(p)
	if string(b) != content {
		t.Fatal("GET modified project")
	}
}

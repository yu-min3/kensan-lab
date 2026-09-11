package workspace

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestActivityAppendPreservesMissingFinalNewline(t *testing.T) {
	w := New(t.TempDir())
	if err := w.Create("todo.md", []byte("original")); err != nil {
		t.Fatal(err)
	}
	// 外部ツールから復元された、末尾改行だけが無い有効なレコード。
	first := `{"ts":"2026-09-10T12:00:00+09:00","kind":"task.state","id":"first","text":"first","date":"2026-09-10","state":"done"}`
	if err := os.WriteFile(filepath.Join(w.Root, ActivityFile), []byte(first), 0644); err != nil {
		t.Fatal(err)
	}
	if events, err := w.Activities(); err != nil || len(events) != 1 {
		t.Fatalf("before: %v %v", events, err)
	}
	err := w.MutateEvent("todo.md", func(_ []byte, _ []Activity) ([]byte, *Activity, error) {
		return []byte("changed"), &Activity{At: time.Now(), ID: "second", Kind: "task.state", State: "done", Date: "2026-09-11"}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	events, err := w.Activities()
	if err != nil || len(events) != 2 {
		t.Fatalf("successful append corrupted history: %v %v", events, err)
	}
	if events[0].ID != "first" || events[1].ID != "second" {
		t.Fatalf("history changed: %+v", events)
	}
}

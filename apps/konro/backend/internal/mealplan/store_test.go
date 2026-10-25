package mealplan

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSnapshotRecalculationAndReplay(t *testing.T) {
	root := t.TempDir()
	raw := "---\ntitle: 玉ねぎ料理\nservings: 2人分\n---\n\n## 材料\n- 玉ねぎ 1/2個\n- 塩 少々\n\n## 手順\n1. 10分煮る\n"
	if e := os.WriteFile(filepath.Join(root, "one.md"), []byte(raw), 0600); e != nil {
		t.Fatal(e)
	}
	s := New(root)
	request := Change{Items: []Selection{{File: "one", Scale: "2"}}, Manual: []Manual{{Text: "牛乳 1本"}}}
	p, e := s.Create("create-1", request)
	if e != nil {
		t.Fatal(e)
	}
	again, e := s.Create("create-1", request)
	if e != nil || p.ID != again.ID {
		t.Fatal(e)
	}
	p, e = s.Update(p.ID, "buy-1", Change{Revision: p.Revision, ItemID: p.Shopping[0].ID, Status: "bought"})
	if e != nil {
		t.Fatal(e)
	}
	oldRevision := p.Revision
	replay, e := s.Update(p.ID, "buy-1", Change{Revision: oldRevision - 1, ItemID: p.Shopping[0].ID, Status: "bought"})
	if e != nil || replay.Revision != oldRevision {
		t.Fatal("replay", e)
	}
	selections := []Selection{{ID: p.Items[0].ID, File: "one", Scale: "3"}}
	preview, e := s.Update(p.ID, "preview", Change{Revision: p.Revision, Items: selections, Preview: true})
	if e != nil || preview.Shopping[0].Status != "review" {
		t.Fatalf("preview %+v %v", preview, e)
	}
	unchanged, _ := s.Get(p.ID)
	if unchanged.Shopping[0].Status != "bought" || unchanged.Revision != p.Revision {
		t.Fatal("preview changed store")
	}
	if e = os.WriteFile(filepath.Join(root, "one.md"), []byte(raw+"2. 新しい工程\n"), 0600); e != nil {
		t.Fatal(e)
	}
	changed, e := s.Update(p.ID, "scale", Change{Revision: p.Revision, Items: selections})
	if e != nil {
		t.Fatal(e)
	}
	if len(changed.Items[0].Recipe.Steps) != 1 || changed.Shopping[0].Text != "玉ねぎ 1.5個" || changed.Shopping[0].Status != "review" || !changed.Shopping[2].Manual {
		t.Fatalf("snapshot/recalc %+v", changed)
	}
	if _, e = s.Update(p.ID, "stale", Change{Revision: oldRevision, Name: "stale"}); !errors.Is(e, ErrConflict) {
		t.Fatal(e)
	}
	if _, e = s.Update(p.ID, "buy-1", Change{Revision: oldRevision - 1, Name: "different payload"}); !errors.Is(e, ErrConflict) {
		t.Fatal(e)
	}
	if _, e = New(root).Get(p.ID); e != nil {
		t.Fatal("restart", e)
	}
}

package taxonomy

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPreserveAndConflict(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	v, e := s.Get()
	if e != nil || len(v.Assignments) != 18 {
		t.Fatalf("%+v %v", v, e)
	}
	v.Assignments["新しいタグ"] = "other"
	saved, e := s.Put(v.Revision, v.Assignments)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Put(v.Revision, v.Assignments); !errors.Is(e, ErrConflict) {
		t.Fatal(e)
	}
	saved.Assignments["野菜"] = "invalid"
	if _, e = s.Put(saved.Revision, saved.Assignments); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	restored, e := New(dir).Get()
	if e != nil || restored.Assignments["新しいタグ"] != "other" || restored.Assignments["野菜"] != "ingredient" {
		t.Fatalf("%+v %v", restored, e)
	}
	if e = os.WriteFile(filepath.Join(dir, "tag-taxonomy.json"), []byte("broken"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Get(); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "tag-taxonomy.json"))
	if string(b) != "broken" {
		t.Fatal("corrupt source was overwritten")
	}
}

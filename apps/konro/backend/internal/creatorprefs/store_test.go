package creatorprefs

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

func TestPersistenceUndoEmptyAndConflict(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	initial, err := s.Get()
	if err != nil || len(initial.Creators) != 3 {
		t.Fatal(initial, err)
	}
	info, err := os.Stat(filepath.Join(dir, ".settings", "creators.json"))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal(info, err)
	}
	removed, err := s.Remove(initial.Revision, initial.Creators[0].ID)
	if err != nil || removed.Undo == "" {
		t.Fatal(removed, err)
	}
	if _, err = s.Remove(initial.Revision, initial.Creators[1].ID); !errors.Is(err, ErrConflict) {
		t.Fatal("stale write allowed", err)
	}
	restored, err := s.Undo(removed.Revision, removed.Undo)
	if err != nil || !reflect.DeepEqual(restored.Creators, initial.Creators) {
		t.Fatal(restored, err)
	}
	if _, err = s.Undo(restored.Revision, removed.Undo); err == nil {
		t.Fatal("undo replay")
	}
	snap := restored.Snapshot
	for _, c := range append([]Creator(nil), snap.Creators...) {
		result, e := s.Remove(snap.Revision, c.ID)
		if e != nil {
			t.Fatal(e)
		}
		snap = result.Snapshot
	}
	loaded, err := New(dir).Get()
	if err != nil || len(loaded.Creators) != 0 {
		t.Fatal("empty list reset to defaults", loaded, err)
	}
	added, err := s.Add(loaded.Revision, initial.Creators[0])
	if err != nil || !added.Creators[0].Additional {
		t.Fatal(added, err)
	}
	undone, err := s.Undo(added.Revision, added.Undo)
	if err != nil || len(undone.Creators) != 0 {
		t.Fatal(undone, err)
	}
}
func TestAdditionalLimitAndConcurrentRevision(t *testing.T) {
	s := New(t.TempDir())
	snap, _ := s.Get()
	c := Creator{ID: "kurashiru:chef:99", Name: "別の料理人", URL: "https://www.kurashiru.com/chefs/99"}
	added, err := s.Add(snap.Revision, c)
	if err != nil || added.Creators[3].Additional {
		t.Fatal(added, err)
	}
	if _, err = s.Additional(added.Revision, c.ID, true); err == nil {
		t.Fatal("fourth additional allowed")
	}
	result, err := s.Additional(added.Revision, added.Creators[0].ID, false)
	if err != nil {
		t.Fatal(err)
	}
	result, err = s.Additional(result.Revision, c.ID, true)
	if err != nil || !result.Creators[3].Additional {
		t.Fatal(result, err)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	success := 0
	for _, creator := range result.Creators[:2] {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			_, err := s.Remove(result.Revision, id)
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				success++
			} else if !errors.Is(err, ErrConflict) {
				t.Error(err)
			}
		}(creator.ID)
	}
	wg.Wait()
	if success != 1 {
		t.Fatal("lost update", success)
	}
}
func TestInvalidFileAndSymlinkAreNotOverwritten(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	_, _ = s.Get()
	path := filepath.Join(dir, ".settings", "creators.json")
	bad := []byte("not json")
	if err := os.WriteFile(path, bad, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(); err == nil {
		t.Fatal("corruption silently reset")
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(bad) {
		t.Fatal("file overwritten")
	}
	dir = t.TempDir()
	target := t.TempDir()
	if err := os.Symlink(target, filepath.Join(dir, ".settings")); err != nil {
		t.Fatal(err)
	}
	if _, err := New(dir).Get(); err == nil {
		t.Fatal("settings symlink accepted")
	}
}

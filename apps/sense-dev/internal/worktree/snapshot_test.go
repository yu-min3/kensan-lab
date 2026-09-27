package worktree

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCaptureIncludesCommittedDirtyAndUntrackedChanges(t *testing.T) {
	m, base := testManager(t)
	path, _, err := m.Ensure(context.Background(), strings.Repeat("a", 32), base)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "committed.txt"), []byte("committed\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "committed.txt"}, {"commit", "-m", "add committed"}} {
		if _, err := git(context.Background(), path, args...); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(path, "committed.txt"), []byte("committed\ndirty\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "new.txt"), []byte("untracked evidence\n"), 0600); err != nil {
		t.Fatal(err)
	}
	snapshot, err := Capture(context.Background(), path, base)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.BaseSHA != base || snapshot.HeadSHA == base || len(snapshot.DiffSHA256) != 64 {
		t.Fatalf("snapshot identity not fixed: %+v", snapshot)
	}
	if snapshot.Clean {
		t.Fatal("dirty checkout was marked clean")
	}
	for _, want := range []string{"committed", "dirty", "untracked evidence", "new.txt"} {
		if !strings.Contains(snapshot.Patch, want) {
			t.Fatalf("snapshot omitted %q", want)
		}
	}
	if again, err := Capture(context.Background(), path, base); err != nil || again != snapshot {
		t.Fatalf("unchanged checkout gave different evidence: %v", err)
	}
	for _, args := range [][]string{{"add", "committed.txt", "new.txt"}, {"commit", "-m", "finish implementation"}} {
		if _, err := git(context.Background(), path, args...); err != nil {
			t.Fatal(err)
		}
	}
	clean, err := Capture(context.Background(), path, base)
	if err != nil || !clean.Clean || clean.HeadSHA == snapshot.HeadSHA || !strings.Contains(clean.Patch, "untracked evidence") {
		t.Fatalf("committed snapshot not reviewable: %+v %v", clean, err)
	}
}

func TestCaptureRejectsEmptyOversizedAndSymlinkChanges(t *testing.T) {
	m, base := testManager(t)
	path, _, err := m.Ensure(context.Background(), strings.Repeat("b", 32), base)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Capture(context.Background(), path, base); err == nil {
		t.Fatal("empty implementation accepted")
	}
	if err := os.Symlink("/etc/passwd", filepath.Join(path, "link")); err != nil {
		t.Fatal(err)
	}
	if _, err := Capture(context.Background(), path, base); err == nil {
		t.Fatal("untracked symlink accepted")
	}
	if err := os.Remove(filepath.Join(path, "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "large.txt"), []byte(strings.Repeat("x", MaxSnapshotBytes+1)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Capture(context.Background(), path, base); err == nil {
		t.Fatal("oversized change accepted")
	}
}

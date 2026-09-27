package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPreflightRequiresHiddenStateAndWritableMounts(t *testing.T) {
	root := t.TempDir()
	worktree := filepath.Join(root, "worktree")
	auth := filepath.Join(root, "auth")
	for _, dir := range []string{worktree, auth} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	binary := filepath.Join(root, "provider")
	if err := os.WriteFile(binary, []byte("test"), 0700); err != nil {
		t.Fatal(err)
	}
	hidden := filepath.Join(root, "controller-state")
	if err := checkPreflight(worktree, auth, hidden, binary, false); err != nil {
		t.Fatal(err)
	}
	if err := checkPreflight(worktree, auth, hidden, binary, true); err == nil {
		t.Fatal("writable review worktree accepted")
	}
	if err := os.Mkdir(hidden, 0700); err != nil {
		t.Fatal(err)
	}
	if err := checkPreflight(worktree, auth, hidden, binary, false); err == nil {
		t.Fatal("visible controller state accepted")
	}
	if err := checkPreflight(worktree, auth, filepath.Join(root, "missing"), filepath.Join(root, "missing-provider"), false); err == nil {
		t.Fatal("missing provider binary accepted")
	}
	if err := checkPreflight(worktree, filepath.Join(root, "missing-auth"), filepath.Join(root, "missing"), binary, false); err == nil {
		t.Fatal("unwritable auth mount accepted")
	}
}

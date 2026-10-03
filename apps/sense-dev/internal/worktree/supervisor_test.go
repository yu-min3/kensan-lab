package worktree

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSupervisorCommitsMixedChangesWithoutHooksAndReopensCandidate(t *testing.T) {
	m, base := testManager(t)
	ctx := context.Background()
	path, _, err := m.Ensure(ctx, strings.Repeat("c", 32), base)
	if err != nil {
		t.Fatal(err)
	}
	// Commit an existing file so additions sort before tracked modifications.
	if err := os.WriteFile(filepath.Join(path, "z.txt"), []byte("old\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "z.txt"}, {"commit", "-m", "baseline"}} {
		if _, err := git(ctx, path, args...); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(path, "z.txt"), []byte("changed\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "a.txt"), []byte("new\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, ".git/hooks/pre-commit"), []byte("#!/bin/sh\nexit 77\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := PrepareSandboxMetadata(path); err != nil {
		t.Fatal(err)
	}
	snapshot, err := CommitImplementation(ctx, path, base)
	if err != nil || !snapshot.Clean || snapshot.HeadSHA == base || !strings.Contains(snapshot.Patch, "changed") || !strings.Contains(snapshot.Patch, "new") {
		t.Fatalf("supervisor did not capture the candidate: %+v %v", snapshot, err)
	}
	again, err := CommitImplementation(ctx, path, base)
	if err != nil || again != snapshot {
		t.Fatalf("retry created a different candidate: %+v %v", again, err)
	}
}

func TestSandboxMetadataRejectsLinksAndPreservesExistingConfiguration(t *testing.T) {
	path := t.TempDir()
	if err := PrepareSandboxMetadata(path); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(path, ".codex", "config.toml")
	if err := os.WriteFile(file, []byte("existing\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := PrepareSandboxMetadata(path); err != nil {
		t.Fatal(err)
	}
	if body, err := os.ReadFile(file); err != nil || string(body) != "existing\n" {
		t.Fatal("existing config was modified")
	}
	other := t.TempDir()
	if err := os.Symlink(path, filepath.Join(other, ".agents")); err != nil {
		t.Fatal(err)
	}
	if err := PrepareSandboxMetadata(other); err == nil {
		t.Fatal("symlink metadata accepted")
	}
}

package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/worktree"
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

func TestTaskGitPreflightUsesSelfContainedCheckout(t *testing.T) {
	base := t.TempDir()
	source, root := filepath.Join(base, "source"), filepath.Join(base, "tasks")
	for _, path := range []string{source, root} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"init", "-q"}, {"config", "user.email", "sense-dev@example.invalid"}, {"config", "user.name", "Sense Dev"}, {"commit", "--allow-empty", "-m", "initial"}} {
		argv := append([]string{"-C", source}, args...)
		if output, err := exec.Command("git", argv...).CombinedOutput(); err != nil {
			t.Fatalf("git setup: %s %v", output, err)
		}
	}
	manager := worktree.Manager{Source: source, Root: root}
	path, _, err := manager.Ensure(context.Background(), strings.Repeat("a", 32), "")
	if err != nil {
		t.Fatal(err)
	}
	if err := checkTaskGit(path); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("git", "-C", path, "remote", "add", "origin", "https://example.invalid/repo.git").CombinedOutput(); err != nil {
		t.Fatalf("git remote setup: %s %v", output, err)
	}
	if err := checkTaskGit(path); err == nil {
		t.Fatal("task checkout with remote passed preflight")
	}
}

func TestTaskGitPreflightRejectsMissingMetadataBeforeModelStarts(t *testing.T) {
	path := t.TempDir()
	if err := checkTaskGit(path); err == nil {
		t.Fatal("missing task Git metadata passed model preflight")
	}
	if err := os.WriteFile(filepath.Join(path, ".git"), []byte("gitdir: /outside\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := checkTaskGit(path); err == nil {
		t.Fatal("external Git metadata passed model preflight")
	}
}

func TestVerifierPreflightRejectsVisibleAuthAndWritableCheckout(t *testing.T) {
	t.Setenv("HOME", "/tmp")
	root := t.TempDir()
	worktree, auth := filepath.Join(root, "worktree"), filepath.Join(root, "auth")
	for _, path := range []string{worktree, auth} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	hidden := filepath.Join(root, "hidden")
	if err := checkVerifierPreflight(worktree, auth, hidden, "host-network-namespace"); err == nil {
		t.Fatal("writable verifier checkout accepted")
	}
	if err := os.WriteFile(filepath.Join(auth, "credential"), []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := checkVerifierPreflight(worktree, auth, hidden, "host-network-namespace"); err == nil {
		t.Fatal("visible provider auth accepted")
	}
	if err := checkVerifierPreflight(worktree, auth, hidden, ""); err == nil {
		t.Fatal("missing host network namespace identity accepted")
	}
}

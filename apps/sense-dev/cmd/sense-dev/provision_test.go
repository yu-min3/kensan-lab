package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/core"
	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/worktree"
)

func TestPrepareReadyWorktreesPinsTaskBase(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	worktrees := filepath.Join(root, "worktrees")
	for _, path := range []string{source, worktrees} {
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
	store, err := core.Open(filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.SeedKnowledge(); err != nil {
		t.Fatal(err)
	}
	task, _, err := store.CreatePlannedTask("mission", core.Platform, "change", "Golden Path", "v1")
	if err != nil {
		t.Fatal(err)
	}
	manager := worktree.Manager{Source: source, Root: worktrees}
	if err := prepareReadyWorktrees(context.Background(), store, manager); err != nil {
		t.Fatal(err)
	}
	base := store.Snapshot().Tasks[task.ID].BaseSHA
	if len(base) != 40 {
		t.Fatalf("task base not pinned: %q", base)
	}
	if _, gotBase, err := manager.Ensure(context.Background(), task.ID, base); err != nil || gotBase != base {
		t.Fatalf("task worktree not recoverable: %s %v", gotBase, err)
	}
	if err := prepareReadyWorktrees(context.Background(), store, manager); err != nil {
		t.Fatalf("idempotent preparation failed: %v", err)
	}
	recovered, _, err := store.CreatePlannedTask("mission", core.App, "analysis", "recovery", "v1")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := manager.Ensure(context.Background(), recovered.ID, ""); err != nil {
		t.Fatal(err)
	}
	if err := prepareReadyWorktrees(context.Background(), store, manager); err != nil || store.Snapshot().Tasks[recovered.ID].BaseSHA == "" {
		t.Fatalf("worktree created before crash was not reconciled: %v", err)
	}
	bad, _, err := store.CreatePlannedTask("mission", core.Platform, "analysis", "bad worktree", "v1")
	if err != nil {
		t.Fatal(err)
	}
	good, _, err := store.CreatePlannedTask("mission", core.App, "analysis", "independent", "v1")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(source, filepath.Join(worktrees, bad.ID)); err != nil {
		t.Fatal(err)
	}
	if err := prepareReadyWorktrees(context.Background(), store, manager); err == nil {
		t.Fatal("unsafe worktree was accepted")
	}
	state := store.Snapshot()
	if state.Tasks[bad.ID].BaseSHA != "" || state.Tasks[good.ID].BaseSHA == "" {
		t.Fatal("one bad task blocked or contaminated another task")
	}
}

func TestLiveWorkerRejectsSimulationHistory(t *testing.T) {
	if err := rejectSimulationHistory(core.State{Attempts: map[string]core.Attempt{"mock": {SessionID: "mock-attempt"}}}); err == nil {
		t.Fatal("mock attempt was promoted to live execution")
	}
	if err := rejectSimulationHistory(core.State{Agents: map[string]core.Agent{"mock": {SessionID: "mock-agent"}}}); err == nil {
		t.Fatal("mock agent session was promoted to live execution")
	}
	if err := rejectSimulationHistory(core.State{}); err != nil {
		t.Fatal(err)
	}
}

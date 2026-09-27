package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/core"
	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/mock"
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

func TestLinkedAcceptanceCheckoutWaitsAndUsesPlatformCommit(t *testing.T) {
	root := t.TempDir()
	source, worktrees := filepath.Join(root, "source"), filepath.Join(root, "worktrees")
	for _, path := range []string{source, worktrees} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"init", "-q"}, {"config", "user.email", "sense-dev@example.invalid"}, {"config", "user.name", "Sense Dev"}, {"commit", "--allow-empty", "-m", "initial"}} {
		if output, err := exec.Command("git", append([]string{"-C", source}, args...)...).CombinedOutput(); err != nil {
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
	platform, _, err := store.CreatePlannedTask("mission", core.Platform, "change", "change", "v1")
	if err != nil {
		t.Fatal(err)
	}
	app, _, err := store.CreateLinkedAcceptanceTask(platform.ID, "accept")
	if err != nil {
		t.Fatal(err)
	}
	manager := worktree.Manager{Source: source, Root: worktrees}
	if err := prepareReadyWorktrees(context.Background(), store, manager); err != nil {
		t.Fatal(err)
	}
	if store.Snapshot().Tasks[app.ID].BaseSHA != "" {
		t.Fatal("App checkout created before handoff")
	}
	platformPath := filepath.Join(worktrees, platform.ID)
	if err := os.WriteFile(filepath.Join(platformPath, "reviewed.txt"), []byte("new contract"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "reviewed.txt"}, {"commit", "-m", "reviewed"}} {
		if output, err := exec.Command("git", append([]string{"-C", platformPath}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("platform commit: %s %v", output, err)
		}
	}
	output, err := exec.Command("git", "-C", platformPath, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	head := strings.TrimSpace(string(output))
	if err := store.SetHeadSHA(platform.ID, head); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if worked, err := store.Tick(context.Background(), mock.Runner{}, []string{"simulation-only"}); err != nil || !worked {
			t.Fatalf("platform stage %d: %t %v", i, worked, err)
		}
	}
	if err := store.SetHeadSHA(app.ID, head); err != nil {
		t.Fatal(err)
	} // Simulate the pinned delivery; evidence policy is tested in core.
	if err := prepareReadyWorktrees(context.Background(), store, manager); err != nil {
		t.Fatal(err)
	}
	if store.Snapshot().Tasks[app.ID].BaseSHA != head {
		t.Fatal("App checkout did not pin Platform head")
	}
	body, err := os.ReadFile(filepath.Join(worktrees, app.ID, "reviewed.txt"))
	if err != nil || string(body) != "new contract" {
		t.Fatalf("App missed Platform change: %q %v", body, err)
	}
}

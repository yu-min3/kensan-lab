package worktree

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testManager(t *testing.T) (Manager, string) {
	t.Helper()
	base := t.TempDir()
	source, root := filepath.Join(base, "source"), filepath.Join(base, "worktrees")
	for _, path := range []string{source, root} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	ctx := context.Background()
	for _, argv := range [][]string{{"init", "-q"}, {"config", "user.email", "sense-dev@example.invalid"}, {"config", "user.name", "Sense Dev"}, {"commit", "--allow-empty", "-m", "initial"}} {
		if _, err := git(ctx, source, argv...); err != nil {
			t.Fatal(err)
		}
	}
	sha, err := git(ctx, source, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	return Manager{Source: source, Root: root}, sha
}

func TestTaskWorktreesAreDistinctAndPinned(t *testing.T) {
	m, sourceSHA := testManager(t)
	ids := []string{strings.Repeat("a", 32), strings.Repeat("b", 32)}
	paths := make([]string, 0, 2)
	for _, id := range ids {
		path, base, err := m.Ensure(context.Background(), id, "")
		if err != nil || base != sourceSHA {
			t.Fatalf("task worktree not pinned: %s %s %v", path, base, err)
		}
		paths = append(paths, path)
		if reopened, gotBase, err := m.Ensure(context.Background(), id, base); err != nil || reopened != path || gotBase != base {
			t.Fatalf("task worktree not idempotent: %s %s %v", reopened, gotBase, err)
		}
	}
	if paths[0] == paths[1] {
		t.Fatal("two tasks share a worktree")
	}
	for _, path := range paths {
		if info, err := os.Lstat(filepath.Join(path, ".git")); err != nil || !info.IsDir() {
			t.Fatal("task checkout has external Git metadata")
		}
		if remotes, err := git(context.Background(), path, "remote"); err != nil || remotes != "" {
			t.Fatalf("task checkout retains a remote: %q %v", remotes, err)
		}
	}
	if _, err := git(context.Background(), paths[0], "commit", "--allow-empty", "-m", "task A"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := m.Ensure(context.Background(), ids[0], sourceSHA); err != nil {
		t.Fatalf("task's descendant commit rejected: %v", err)
	}
	headB, err := git(context.Background(), paths[1], "rev-parse", "HEAD")
	if err != nil || headB != sourceSHA {
		t.Fatalf("task B observed task A commit: %s %v", headB, err)
	}
	hiddenSource := m.Source + "-hidden"
	if err := os.Rename(m.Source, hiddenSource); err != nil {
		t.Fatal(err)
	}
	defer os.Rename(hiddenSource, m.Source)
	if _, err := git(context.Background(), paths[0], "status", "--porcelain"); err != nil {
		t.Fatalf("task Git metadata depends on source repo: %v", err)
	}
}

func TestRejectsPathEscapeAndWrongRepository(t *testing.T) {
	m, sha := testManager(t)
	if _, _, err := m.Ensure(context.Background(), "../outside", sha); err == nil {
		t.Fatal("path traversal accepted")
	}
	id := strings.Repeat("c", 32)
	if err := os.Symlink(m.Source, filepath.Join(m.Root, id)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := m.Ensure(context.Background(), id, sha); err == nil {
		t.Fatal("symlinked worktree accepted")
	}
	if err := os.Remove(filepath.Join(m.Root, id)); err != nil {
		t.Fatal(err)
	}
	wrong := filepath.Join(m.Root, id)
	if err := os.Mkdir(wrong, 0700); err != nil {
		t.Fatal(err)
	}
	if _, _, err := m.Ensure(context.Background(), id, sha); err == nil {
		t.Fatal("unrelated directory accepted")
	}
}

func TestRejectsChangedTaskBranch(t *testing.T) {
	m, sha := testManager(t)
	id := strings.Repeat("d", 32)
	path, _, err := m.Ensure(context.Background(), id, sha)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := git(context.Background(), path, "switch", "-c", "unexpected"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := m.Ensure(context.Background(), id, sha); err == nil {
		t.Fatal("task branch switch accepted")
	}
}

func TestRejectsRemoteAddedToTaskCheckout(t *testing.T) {
	m, sha := testManager(t)
	id := strings.Repeat("e", 32)
	path, _, err := m.Ensure(context.Background(), id, sha)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := git(context.Background(), path, "remote", "add", "origin", "https://example.invalid/repo.git"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := m.Ensure(context.Background(), id, sha); err == nil {
		t.Fatal("task checkout with a remote was accepted")
	}
}

func TestConsumerCheckoutStartsAtReviewedProducerCommit(t *testing.T) {
	m, original := testManager(t)
	producerID, consumerID := strings.Repeat("a", 32), strings.Repeat("b", 32)
	producer, _, err := m.Ensure(context.Background(), producerID, original)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(producer, "change.txt"), []byte("reviewed change\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := git(context.Background(), producer, "add", "change.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := git(context.Background(), producer, "commit", "-m", "reviewed change"); err != nil {
		t.Fatal(err)
	}
	head, err := git(context.Background(), producer, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if head == original {
		t.Fatal("producer did not advance")
	}
	consumer, base, err := m.EnsureFromTask(context.Background(), consumerID, producerID, head)
	if err != nil || base != head {
		t.Fatalf("consumer at wrong revision: %s %v", base, err)
	}
	body, err := os.ReadFile(filepath.Join(consumer, "change.txt"))
	if err != nil || string(body) != "reviewed change\n" {
		t.Fatalf("consumer did not see change: %q %v", body, err)
	}
	if _, _, err := m.EnsureFromTask(context.Background(), consumerID, producerID, original); err == nil {
		t.Fatal("stale reviewed head accepted")
	}
	if err := os.WriteFile(filepath.Join(producer, "change.txt"), []byte("corrected change\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := git(context.Background(), producer, "add", "change.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := git(context.Background(), producer, "commit", "-m", "correction"); err != nil {
		t.Fatal(err)
	}
	corrected, err := git(context.Background(), producer, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.AdvanceFromTask(context.Background(), consumerID, producerID, head, corrected); err != nil {
		t.Fatal(err)
	}
	if _, err := m.AdvanceFromTask(context.Background(), consumerID, producerID, head, corrected); err != nil {
		t.Fatalf("advance not restart-safe: %v", err)
	}
	body, err = os.ReadFile(filepath.Join(consumer, "change.txt"))
	if err != nil || string(body) != "corrected change\n" {
		t.Fatalf("consumer missed correction: %q %v", body, err)
	}
	if err := os.WriteFile(filepath.Join(consumer, "change.txt"), []byte("consumer edit"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := m.AdvanceFromTask(context.Background(), consumerID, producerID, head, corrected); err == nil {
		t.Fatal("dirty consumer checkout accepted")
	}
}

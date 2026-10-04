package worktree

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImageDeploymentSeedsAAndKeepsOriginalBaseAfterB(t *testing.T) {
	ctx := context.Background()
	m, base := testManager(t)
	producerID, childID := strings.Repeat("a", 32), strings.Repeat("b", 32)
	producer, _, err := m.Ensure(ctx, producerID, base)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(producer, "feature.txt"), []byte("v2"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := git(ctx, producer, "add", "feature.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := git(ctx, producer, "commit", "-m", "feature"); err != nil {
		t.Fatal(err)
	}
	a, err := git(ctx, producer, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	child, pinned, err := m.EnsureImageDeployment(ctx, childID, producerID, base, a, true)
	if err != nil || pinned != base {
		t.Fatalf("seed %s %v", pinned, err)
	}
	if h, _ := git(ctx, child, "rev-parse", "HEAD"); h != a {
		t.Fatal("child not seeded at A")
	}
	if err := os.WriteFile(filepath.Join(child, "digest.txt"), []byte("D"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := m.EnsureImageDeployment(ctx, childID, producerID, base, a, true); err == nil {
		t.Fatal("dirty child reopened for review")
	}
	if _, _, err := m.EnsureImageDeployment(ctx, childID, producerID, base, a, false); err != nil {
		t.Fatal(err)
	}
	if _, err := git(ctx, child, "add", "digest.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := git(ctx, child, "commit", "-m", "digest"); err != nil {
		t.Fatal(err)
	}
	if _, pinned, err := m.EnsureImageDeployment(ctx, childID, producerID, base, a, true); err != nil || pinned != base {
		t.Fatalf("B reopened with wrong base %s %v", pinned, err)
	}
	if _, _, err := m.EnsureImageDeployment(ctx, childID, producerID, base, base, true); err == nil {
		t.Fatal("source identity changed")
	}
}

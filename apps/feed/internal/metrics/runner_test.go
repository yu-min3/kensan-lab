package metrics

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeStars struct {
	values map[string]float64
	errFor string
}

func (f fakeStars) Stars(_ context.Context, repo string) (float64, error) {
	if repo == f.errFor {
		return 0, context.DeadlineExceeded
	}
	return f.values[repo], nil
}

func TestUpdateAllContinuesAfterProjectFailureAndDeduplicates(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, "ok", "owner/ok")
	writeConfig(t, root, "failed", "owner/failed")
	now := time.Date(2026, 7, 25, 7, 15, 0, 0, time.FixedZone("JST", 9*60*60))
	runner := Runner{
		Root: root,
		Collector: fakeStars{
			values: map[string]float64{"owner/ok": 12},
			errFor: "owner/failed",
		},
		Now: func() time.Time { return now },
	}
	for run := 0; run < 2; run++ {
		result, err := runner.UpdateAll(context.Background())
		if err == nil {
			t.Fatal("expected partial failure")
		}
		if result.Projects != 2 {
			t.Fatalf("projects=%d", result.Projects)
		}
		if run == 0 && result.Updated != 1 {
			t.Fatalf("updated=%d", result.Updated)
		}
		if run == 1 && result.Updated != 0 {
			t.Fatalf("deduplicated updated=%d", result.Updated)
		}
	}
	content, err := os.ReadFile(filepath.Join(root, "projects", "ok", "metrics.ndjson"))
	if err != nil {
		t.Fatal(err)
	}
	if lines := strings.Count(strings.TrimSpace(string(content)), "\n") + 1; lines != 1 {
		t.Fatalf("lines=%d content=%s", lines, content)
	}
}

func writeConfig(t *testing.T, root, project, repo string) {
	t.Helper()
	dir := filepath.Join(root, "projects", project)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "version: 1\nmetrics:\n  - id: github-stars\n    label: GitHub Stars\n    unit: stars\n    direction: increase\n    display: integer\n    checkpoints:\n      - value: 10\n        label: 最初の10人\n    collector:\n      type: github-stars\n      repo: " + repo + "\n"
	if err := os.WriteFile(filepath.Join(dir, "metrics.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

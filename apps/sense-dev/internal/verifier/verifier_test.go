package verifier

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/core"
)

func TestLoadPlanRejectsWorkerControlledAndMalformedPolicy(t *testing.T) {
	root := t.TempDir()
	worker := filepath.Join(root, "worker")
	if err := os.Mkdir(worker, 0700); err != nil {
		t.Fatal(err)
	}
	valid := []byte(`{"schema_version":1,"checks":[{"name":"unit","cwd":"apps/sense-dev","argv":["/usr/local/go/bin/go","test","./..."],"timeout_seconds":120}]}`)
	path := filepath.Join(root, "plan.json")
	if err := os.WriteFile(path, valid, 0600); err != nil {
		t.Fatal(err)
	}
	plan, err := LoadPlan(path, worker)
	if err != nil || len(plan.Checks) != 1 || len(plan.SHA256) != 64 {
		t.Fatalf("operator plan rejected: %+v %v", plan, err)
	}
	workerPlan := filepath.Join(worker, "plan.json")
	if err := os.WriteFile(workerPlan, valid, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPlan(workerPlan, worker); err == nil {
		t.Fatal("model-writable plan accepted")
	}
	for _, bad := range []string{
		`{"schema_version":1,"checks":[{"name":"x","cwd":"../state","argv":["/bin/sh"],"timeout_seconds":1}]}`,
		`{"schema_version":1,"checks":[{"name":"x","cwd":".","argv":["sh"],"timeout_seconds":1}]}`,
		`{"schema_version":1,"checks":[],"extra":true}`,
		string(valid) + ` {}`,
	} {
		if err := os.WriteFile(path, []byte(bad), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadPlan(path, worker); err == nil {
			t.Fatalf("malformed plan accepted: %s", bad)
		}
	}
	if err := os.WriteFile(path, valid, 0666); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0666); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPlan(path, worker); err == nil {
		t.Fatal("group/world-writable plan accepted")
	}
}

func TestRunnerRejectsNonVerificationDispatch(t *testing.T) {
	r := Runner{}
	for _, role := range []string{"implementation", "verification"} {
		_, err := r.Run(context.Background(), core.Dispatch{Attempt: core.Attempt{Role: role, Provider: "codex", Model: "gpt-6-sol", TaskID: strings.Repeat("a", 32)}})
		if err == nil {
			t.Fatalf("invalid local verifier dispatch accepted: %s", role)
		}
	}
}

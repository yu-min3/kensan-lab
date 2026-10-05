package publisher

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/core"
)

type mergeFixture struct {
	head, base, revision, main, prHead, parentHead             string
	draft, merged, strict, admins, missingProtection           bool
	checkName, checkSHA, checkStatus, checkConclusion, appSlug string
	mergeRequests                                              int
}

func newMergeFixture(t *testing.T) (*mergeFixture, GitHub, core.PublishIntent) {
	t.Helper()
	f := &mergeFixture{head: strings.Repeat("a", 40), base: strings.Repeat("b", 40), revision: strings.Repeat("c", 40), strict: true, admins: true, checkName: canaryRequiredCheck, checkStatus: "completed", checkConclusion: "success", appSlug: "github-actions"}
	f.main, f.prHead, f.parentHead, f.checkSHA = f.base, f.head, f.head, f.head
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		write := func(body any) { _ = json.NewEncoder(w).Encode(body) }
		switch {
		case r.URL.Path == "/repos/"+repository+"/pulls":
			write([]map[string]any{{"number": 42}})
		case r.URL.Path == "/repos/"+repository+"/pulls/42":
			write(map[string]any{"number": 42, "state": "open", "draft": f.draft, "merged": f.merged, "merge_commit_sha": f.revision, "mergeable": true, "mergeable_state": "clean", "head": map[string]any{"sha": f.prHead, "ref": "sense-dev/test", "repo": map[string]string{"full_name": repository}}, "base": map[string]any{"sha": f.base, "ref": "main", "repo": map[string]string{"full_name": repository}}})
		case strings.Contains(r.URL.Path, "/git/ref/heads/"):
			sha := f.head
			if strings.HasSuffix(r.URL.Path, "/main") {
				sha = f.main
			}
			write(map[string]any{"object": map[string]string{"sha": sha}})
		case strings.HasSuffix(r.URL.Path, "/branches/main/protection"):
			if f.missingProtection {
				w.WriteHeader(404)
				return
			}
			write(map[string]any{"enforce_admins": map[string]bool{"enabled": f.admins}, "required_status_checks": map[string]any{"strict": f.strict, "contexts": []string{canaryRequiredCheck}, "checks": []map[string]any{{"context": canaryRequiredCheck, "app_id": 15368}}}})
		case strings.HasSuffix(r.URL.Path, "/check-runs"):
			write(map[string]any{"total_count": 1, "check_runs": []map[string]any{{"name": f.checkName, "head_sha": f.checkSHA, "status": f.checkStatus, "conclusion": f.checkConclusion, "app": map[string]any{"slug": f.appSlug, "id": 15368}}}})
		case strings.HasSuffix(r.URL.Path, "/pulls/42/merge") && r.Method == http.MethodPut:
			f.mergeRequests++
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["sha"] != f.head || body["merge_method"] != "merge" {
				t.Errorf("merge did not pin approved head: %+v", body)
			}
			f.merged = true
			write(map[string]any{"merged": true, "sha": f.revision})
		case strings.Contains(r.URL.Path, "/git/commits/"):
			write(map[string]any{"sha": f.revision, "parents": []map[string]string{{"sha": f.base}, {"sha": f.parentHead}}})
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(server.Close)
	file := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(file, []byte("test-token"), 0600); err != nil {
		t.Fatal(err)
	}
	i := core.PublishIntent{Operation: "merge", Repository: repository, Ref: "refs/heads/sense-dev/test", HeadSHA: f.head, BaseSHA: f.base, TargetEnvironment: "private-canary", PolicyVersion: core.ReleasePolicyVersion, ExpiresAt: time.Now().Add(time.Hour)}
	return f, GitHub{TokenFile: file, PackageTokenFile: fixturePackageToken(t), Client: server.Client(), API: server.URL}, i
}

func TestGitOpsMergeReturnsVerifiedRevisionAndReconciles(t *testing.T) {
	for _, operation := range []string{"merge", "deploy"} {
		t.Run(operation, func(t *testing.T) {
			f, g, i := newMergeFixture(t)
			i.Operation = operation
			if _, exists, err := g.Inspect(context.Background(), i); err != nil || exists {
				t.Fatalf("premerge inspect: %t %v", exists, err)
			}
			sha, err := g.Execute(context.Background(), i)
			if err != nil || sha != f.revision || sha == i.HeadSHA || f.mergeRequests != 1 {
				t.Fatalf("merge: %q %v", sha, err)
			}
			i.ExpiresAt = time.Now().Add(-time.Second)
			if sha, exists, err := g.Inspect(context.Background(), i); err != nil || !exists || sha != f.revision {
				t.Fatalf("unknown merge reconciliation: %q %t %v", sha, exists, err)
			}
			if _, err := g.Execute(context.Background(), i); err == nil || f.mergeRequests != 1 {
				t.Fatal("expired or completed merge repeated")
			}
		})
	}
}

func TestGitOpsMergeRejectsUnsafeOrIncompleteEvidence(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*mergeFixture, *core.PublishIntent)
	}{
		{"draft", func(f *mergeFixture, _ *core.PublishIntent) { f.draft = true }},
		{"wrong PR head", func(f *mergeFixture, _ *core.PublishIntent) { f.prHead = f.base }},
		{"stale main", func(f *mergeFixture, _ *core.PublishIntent) { f.main = f.revision }},
		{"missing CI", func(f *mergeFixture, _ *core.PublishIntent) { f.checkName = "unrelated" }},
		{"CI wrong SHA", func(f *mergeFixture, _ *core.PublishIntent) { f.checkSHA = f.base }},
		{"CI running", func(f *mergeFixture, _ *core.PublishIntent) { f.checkStatus = "in_progress" }},
		{"CI failure", func(f *mergeFixture, _ *core.PublishIntent) { f.checkConclusion = "failure" }},
		{"CI forged app", func(f *mergeFixture, _ *core.PublishIntent) { f.appSlug = "untrusted-app" }},
		{"missing protection", func(f *mergeFixture, _ *core.PublishIntent) { f.missingProtection = true }},
		{"non-strict protection", func(f *mergeFixture, _ *core.PublishIntent) { f.strict = false }},
		{"admin bypass", func(f *mergeFixture, _ *core.PublishIntent) { f.admins = false }},
		{"public environment", func(_ *mergeFixture, i *core.PublishIntent) { i.TargetEnvironment = "github" }},
		{"wrong policy", func(_ *mergeFixture, i *core.PublishIntent) { i.PolicyVersion = "old" }},
		{"expired", func(_ *mergeFixture, i *core.PublishIntent) { i.ExpiresAt = time.Now().Add(-time.Second) }},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			f, g, i := newMergeFixture(t)
			tt.mutate(f, &i)
			if _, err := g.Execute(context.Background(), i); err == nil {
				t.Fatal("unsafe merge accepted")
			}
			if f.mergeRequests != 0 {
				t.Fatalf("merge PUT count=%d", f.mergeRequests)
			}
		})
	}
	t.Run("wrong merged parent", func(t *testing.T) {
		f, g, i := newMergeFixture(t)
		f.merged = true
		f.parentHead = f.base
		if _, exists, err := g.Inspect(context.Background(), i); err == nil || exists {
			t.Fatal("wrong merge lineage reconciled")
		}
	})
}

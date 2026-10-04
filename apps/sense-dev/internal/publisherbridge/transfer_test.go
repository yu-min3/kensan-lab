package publisherbridge

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/committransfer"
	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/core"
)

func transferGit(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, b)
	}
	return strings.TrimSpace(string(b))
}

type bundleMergedTransport struct {
	fakeTransport
	revision string
}

func (m *bundleMergedTransport) Inspect(context.Context, core.PublishIntent) (string, bool, error) {
	return m.revision, m.revision != "", nil
}
func transferFixture(t *testing.T) (bundle committransfer.Bundle, source, target string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	source = filepath.Join(root, "author")
	target = filepath.Join(root, "publisher")
	os.Mkdir(source, 0700)
	transferGit(t, source, "init", "-q")
	transferGit(t, source, "config", "user.name", "Test")
	transferGit(t, source, "config", "user.email", "test@example.invalid")
	os.WriteFile(filepath.Join(source, "README"), []byte("base"), 0600)
	transferGit(t, source, "add", "README")
	transferGit(t, source, "commit", "-qm", "base")
	base := transferGit(t, source, "rev-parse", "HEAD")
	transferGit(t, root, "clone", "-q", source, target)
	os.WriteFile(filepath.Join(source, "feature"), []byte("change"), 0600)
	transferGit(t, source, "add", "feature")
	transferGit(t, source, "commit", "-qm", "feature")
	head := transferGit(t, source, "rev-parse", "HEAD")
	bundle, err = committransfer.Create(context.Background(), source, strings.Repeat("a", 32), base, head)
	if err != nil {
		t.Fatal(err)
	}
	return
}
func transferIntent(b committransfer.Bundle) core.PublishIntent {
	return core.PublishIntent{ID: "intent", DecisionID: "decision", Operation: "branch_push", Repository: "yu-min3/kensan-lab", Ref: "refs/heads/sense-dev/" + b.TaskID, BaseSHA: b.BaseSHA, HeadSHA: b.HeadSHA, PolicyVersion: core.ReleasePolicyVersion, TargetEnvironment: "private-canary", ExpiresAt: time.Now().Add(time.Minute)}
}
func TestUnixBundleRoundTripAndVerifiedMergeReturn(t *testing.T) {
	t.Setenv("TMPDIR", "/private/tmp")
	b, source, target := transferFixture(t)
	transferGit(t, source, "checkout", "-qb", "main-continuation", b.BaseSHA)
	os.WriteFile(filepath.Join(source, "README"), []byte("advanced main"), 0600)
	transferGit(t, source, "add", "README")
	transferGit(t, source, "commit", "-qm", "main advance")
	transferGit(t, source, "merge", "--no-ff", "-qm", "actual merge", b.HeadSHA)
	mergeSHA := transferGit(t, source, "rev-parse", "HEAD")
	transport := &bundleMergedTransport{revision: mergeSHA}
	auth := bridgeAuth(t)
	handler, err := HandlerWithTransfers(auth, transport, nil, Transfers{RepoPath: target, Export: func(ctx context.Context, task, base, head string) (committransfer.Bundle, error) {
		return committransfer.Create(ctx, source, task, base, head)
	}})
	if err != nil {
		t.Fatal(err)
	}
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	socket := filepath.Join(dir, "transfer.sock")
	listener, err := Listen(socket, -1)
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: handler}
	go server.Serve(listener)
	defer server.Close()
	client := Client{Socket: socket, AuthFile: auth}
	intent := transferIntent(b)
	if err := client.ImportCommit(context.Background(), intent, b); err != nil {
		t.Fatal(err)
	}
	if transferGit(t, target, "rev-parse", "HEAD") != b.BaseSHA {
		t.Fatal("publisher checkout changed")
	}
	intent.Operation, intent.Status, intent.ExternalID = "merge", "sent", mergeSHA
	returned, err := client.ExportMerged(context.Background(), intent, b.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if returned.HeadSHA != mergeSHA || returned.HeadSHA == b.HeadSHA {
		t.Fatal("reviewed head confused with actual merge")
	}
	// target also models the credentialless controller source with the old base.
	if err := committransfer.Import(context.Background(), target, returned); err != nil {
		t.Fatal(err)
	}
	if transferGit(t, target, "rev-parse", returned.HeadSHA+"^{commit}") != mergeSHA || transferGit(t, target, "rev-parse", b.HeadSHA+"^{commit}") != b.HeadSHA {
		t.Fatal("merge graph or author commit missing")
	}
	if transport.calls.Load() != 0 {
		t.Fatal("bundle transfer invoked external write")
	}
}
func TestBundleRouteRejectsUnauthorizedTamperedOrUnboundInput(t *testing.T) {
	b, _, target := transferFixture(t)
	transport := &bundleMergedTransport{}
	handler, err := HandlerWithTransfers(bridgeAuth(t), transport, nil, Transfers{RepoPath: target})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, auth string
		mutate     func(*transferRequest)
	}{
		{"missing auth", "", nil},
		{"wrong head", strings.Repeat("x", 40), func(r *transferRequest) { r.Intent.HeadSHA = r.Intent.BaseSHA }},
		{"wrong base", strings.Repeat("x", 40), func(r *transferRequest) { r.Intent.BaseSHA = strings.Repeat("b", 40) }},
		{"wrong task", strings.Repeat("x", 40), func(r *transferRequest) { r.Intent.Ref = "refs/heads/arbitrary" }},
		{"expired", strings.Repeat("x", 40), func(r *transferRequest) { r.Intent.ExpiresAt = time.Now().Add(-time.Minute) }},
		{"tampered", strings.Repeat("x", 40), func(r *transferRequest) { r.Bundle.SHA256 = strings.Repeat("0", 64) }},
		{"wrong operation", strings.Repeat("x", 40), func(r *transferRequest) { r.Intent.Operation = "merge" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			copyBundle := b
			input := transferRequest{Intent: transferIntent(b), Bundle: &copyBundle}
			if tc.mutate != nil {
				tc.mutate(&input)
			}
			body, _ := json.Marshal(input)
			r := httptest.NewRequest(http.MethodPost, "/v1/commits", bytes.NewReader(body))
			r.Header.Set("Authorization", "Bearer "+tc.auth)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code == 200 {
				t.Fatal("invalid bundle imported")
			}
		})
	}
	// The ordinary publish endpoint retains its independent 64 KiB limit.
	r := httptest.NewRequest(http.MethodPost, "/v1/publish", strings.NewReader(strings.Repeat(" ", 65<<10)))
	r.Header.Set("Authorization", "Bearer "+strings.Repeat("x", 40))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code == 200 {
		t.Fatal("normal route body limit removed")
	}
}

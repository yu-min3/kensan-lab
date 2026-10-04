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

func TestGitHubPRRequiresExactRemoteSHAAndCreatesDraft(t *testing.T) {
	sha := strings.Repeat("a", 40)
	remoteSHA := sha
	created := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("missing scoped token")
		}
		switch {
		case strings.HasPrefix(r.URL.Path, "/repos/yu-min3/kensan-lab/git/ref/heads/"):
			_, _ = w.Write([]byte(`{"object":{"sha":"` + remoteSHA + `"}}`))
		case r.URL.Path == "/repos/yu-min3/kensan-lab/pulls" && r.Method == http.MethodGet:
			if r.URL.Query().Get("state") != "all" {
				t.Error("closed PR was omitted from reconciliation")
			}
			_, _ = w.Write([]byte(`[]`))
		case r.URL.Path == "/repos/yu-min3/kensan-lab/pulls" && r.Method == http.MethodPost:
			created++
			body := make([]byte, 4096)
			n, _ := r.Body.Read(body)
			if !strings.Contains(string(body[:n]), `"draft":true`) {
				t.Error("PR was not draft")
			}
			_, _ = w.Write([]byte(`{"number":42}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	file := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(file, []byte("test-token\n"), 0600); err != nil {
		t.Fatal(err)
	}
	g := GitHub{TokenFile: file, Client: server.Client(), API: server.URL}
	i := core.PublishIntent{DecisionID: "gate-1", Operation: "pr_create", Repository: repository, Ref: "refs/heads/feat/canary", HeadSHA: sha, PullRequestSummary: "Add private canary"}
	if _, exists, err := g.Inspect(context.Background(), i); err != nil || exists {
		t.Fatalf("unexpected PR state: %t %v", exists, err)
	}
	remoteSHA = strings.Repeat("b", 40)
	if _, err := g.Execute(context.Background(), i); err == nil {
		t.Fatal("another remote SHA was accepted")
	}
	remoteSHA = sha
	if id, err := g.Execute(context.Background(), i); err != nil || id != "42" || created != 1 {
		t.Fatalf("draft PR was not created: %q %v", id, err)
	}
}

func TestPrivateCanaryPRIsReadyUnderFixedPolicy(t *testing.T) {
	sha := strings.Repeat("a", 40)
	created := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/git/ref/") {
			_, _ = w.Write([]byte(`{"object":{"sha":"` + sha + `"}}`))
			return
		}
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/pulls") {
			var body struct {
				Draft bool   `json:"draft"`
				Base  string `json:"base"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Draft || body.Base != "main" {
				t.Error("private canary did not create ready main PR")
			}
			created = true
			_, _ = w.Write([]byte(`{"number":42}`))
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()
	file := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(file, []byte("test-token"), 0600); err != nil {
		t.Fatal(err)
	}
	g := GitHub{TokenFile: file, Client: server.Client(), API: server.URL}
	i := core.PublishIntent{Operation: "pr_create", Repository: repository, Ref: "refs/heads/sense-dev/task", HeadSHA: sha, TargetEnvironment: "private-canary", PolicyVersion: core.ReleasePolicyVersion, ExpiresAt: time.Now().Add(time.Hour), PullRequestSummary: "Add private canary"}
	if _, err := g.Execute(context.Background(), i); err != nil || !created {
		t.Fatalf("ready private PR: %v", err)
	}
}

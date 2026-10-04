package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/committransfer"
	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/core"
	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/publisherbridge"
)

func commitGit(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	body, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git: %v %s", err, body)
	}
	return strings.TrimSpace(string(body))
}

type transferControllerTransport struct {
	merged string
	writes int
}

func (x *transferControllerTransport) Inspect(context.Context, core.PublishIntent) (string, bool, error) {
	return x.merged, true, nil
}
func (x *transferControllerTransport) Execute(context.Context, core.PublishIntent) (string, error) {
	x.writes++
	return "", nil
}
func TestControllerFixedAuthorTransferAndMergedSourceImport(t *testing.T) {
	t.Setenv("TMPDIR", "/private/tmp")
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	worktrees := filepath.Join(root, "tasks")
	os.Mkdir(worktrees, 0700)
	taskID := strings.Repeat("a", 32)
	repo := filepath.Join(worktrees, taskID)
	os.Mkdir(repo, 0700)
	commitGit(t, repo, "init", "-q")
	commitGit(t, repo, "config", "user.name", "Test")
	commitGit(t, repo, "config", "user.email", "test@example.invalid")
	commitGit(t, repo, "commit", "--allow-empty", "-qm", "base")
	base := commitGit(t, repo, "rev-parse", "HEAD")
	publisherRepo := filepath.Join(root, "publisher")
	sourceRepo := filepath.Join(root, "controller-source")
	commitGit(t, root, "clone", "-q", repo, publisherRepo)
	commitGit(t, root, "clone", "-q", repo, sourceRepo)
	os.WriteFile(filepath.Join(repo, "feature"), []byte("feature"), 0600)
	commitGit(t, repo, "add", "feature")
	commitGit(t, repo, "commit", "-qm", "feature")
	head := commitGit(t, repo, "rev-parse", "HEAD")
	commitGit(t, repo, "commit", "--allow-empty", "-qm", "merged revision")
	merged := commitGit(t, repo, "rev-parse", "HEAD")
	st := core.NewState()
	st.Tasks[taskID] = core.Task{ID: taskID, MissionID: "mission", Team: core.App, Kind: "change", BaseSHA: base, HeadSHA: head}
	st.Agents["author"] = core.Agent{ID: "author", TaskID: taskID}
	intent := core.PublishIntent{ID: "intent", DecisionID: "decision", Operation: "branch_push", Repository: "yu-min3/kensan-lab", Ref: "refs/heads/sense-dev/" + taskID, BaseSHA: base, HeadSHA: head, TargetEnvironment: "private-canary", PolicyVersion: core.ReleasePolicyVersion, ExpiresAt: time.Now().Add(time.Minute)}
	st.Decisions["decision"] = core.ReleaseDecision{ID: "decision", AuthorAgentID: "author", Operation: "branch_push", HeadSHA: head, Ref: intent.Ref}
	mergedIntent := intent
	mergedIntent.ID = "merge-intent"
	mergedIntent.DecisionID = "merge-decision"
	mergedIntent.Operation = "merge"
	mergedIntent.Status = "sent"
	mergedIntent.ExternalID = merged
	st.Intents[mergedIntent.ID] = mergedIntent
	st.Decisions[mergedIntent.DecisionID] = core.ReleaseDecision{ID: mergedIntent.DecisionID, AuthorAgentID: "author", HeadSHA: head, Operation: "merge"}
	stateDir := filepath.Join(root, "state")
	os.Mkdir(stateDir, 0700)
	body, _ := json.Marshal(st)
	os.WriteFile(filepath.Join(stateDir, "state.json"), body, 0600)
	store, err := core.Open(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	auth := filepath.Join(root, "bridge-auth")
	os.WriteFile(auth, []byte(strings.Repeat("x", 40)), 0600)
	transport := &transferControllerTransport{merged: merged}
	handler, err := publisherbridge.HandlerWithTransfers(auth, transport, nil, publisherbridge.Transfers{RepoPath: publisherRepo, Export: func(ctx context.Context, task, base, head string) (committransfer.Bundle, error) {
		return committransfer.Create(ctx, repo, task, base, head)
	}})
	if err != nil {
		t.Fatal(err)
	}
	socket := filepath.Join(root, "bridge.sock")
	listener, err := publisherbridge.Listen(socket, -1)
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: handler}
	go server.Serve(listener)
	defer server.Close()
	client := publisherbridge.Client{Socket: socket, AuthFile: auth}
	publisher := preparedPublisher{Client: client, Store: store, WorktreeRoot: worktrees}
	if err := publisher.Prepare(context.Background(), intent); err != nil {
		t.Fatal(err)
	}
	if !committransfer.Contains(context.Background(), publisherRepo, head) || committransfer.Contains(context.Background(), sourceRepo, merged) {
		t.Fatal("author graph import boundary failed")
	}
	invalid := intent
	invalid.HeadSHA = merged
	if err := publisher.Prepare(context.Background(), invalid); err == nil {
		t.Fatal("changed author binding accepted")
	}
	if err := importMergedCommits(context.Background(), store, client, sourceRepo, "other"); err != nil {
		t.Fatal(err)
	}
	if committransfer.Contains(context.Background(), sourceRepo, merged) {
		t.Fatal("cross-mission merged graph imported")
	}
	if err := importMergedCommits(context.Background(), store, client, sourceRepo, "mission"); err != nil {
		t.Fatal(err)
	}
	if !committransfer.Contains(context.Background(), sourceRepo, merged) || !committransfer.Contains(context.Background(), sourceRepo, head) || commitGit(t, sourceRepo, "rev-parse", "HEAD") != base {
		t.Fatal("merged graph not reachable or checkout changed")
	}
	if err := importMergedCommits(context.Background(), store, client, sourceRepo, "mission"); err != nil || transport.writes != 0 {
		t.Fatal("idempotent import performed external write")
	}
}

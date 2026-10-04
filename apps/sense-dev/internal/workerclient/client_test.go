package workerclient

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/core"
	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/workerwire"
)

func TestRunnerRejectsReviewerWithWritableModel(t *testing.T) {
	runner := Runner{WorkerProgram: "/usr/local/bin/sense-dev-worker", Timeout: time.Minute}
	_, err := runner.Run(context.Background(), core.Dispatch{Attempt: core.Attempt{ID: "attempt-1", Role: "design_review", Provider: "codex", Model: "gpt-6-sol"}, Prompt: "review", BindSession: func(string) error { return nil }})
	if err == nil {
		t.Fatal("review role acquired Sol's writable model")
	}
}

func TestRunnerRejectsUnpinnedTaskWorktree(t *testing.T) {
	runner := Runner{WorkerProgram: "/usr/local/bin/sense-dev-worker", Timeout: time.Minute}
	_, err := runner.Run(context.Background(), core.Dispatch{Attempt: core.Attempt{ID: "attempt-1", TaskID: strings.Repeat("a", 32), Role: "implementation", Provider: "codex", Model: "gpt-6-sol"}, Prompt: "work", BindSession: func(string) error { return nil }})
	if err == nil {
		t.Fatal("isolated turn started without a pinned task worktree")
	}
}

func TestRunnerRejectsMismatchedLinkedConsumerRevision(t *testing.T) {
	runner := Runner{WorkerProgram: "/usr/local/bin/sense-dev-worker", Timeout: time.Minute}
	_, err := runner.Run(context.Background(), core.Dispatch{
		Attempt:  core.Attempt{ID: "attempt-1", TaskID: strings.Repeat("a", 32), Role: "app_acceptance", Provider: "claude", Model: "opus", BaseSHA: strings.Repeat("b", 40), HeadSHA: strings.Repeat("c", 40)},
		Manifest: core.ContextManifest{Team: core.App, SourceTaskID: strings.Repeat("d", 32), HeadSHA: strings.Repeat("c", 40)},
		Prompt:   "accept", BindSession: func(string) error { return nil },
	})
	if err == nil || !strings.Contains(err.Error(), "linked consumer revision") {
		t.Fatalf("mismatched linked checkout accepted: %v", err)
	}
}

func TestReadEventsPersistsSessionBeforeAck(t *testing.T) {
	events := "{\"version\":1,\"type\":\"session\",\"session_id\":\"thread-1\"}\n{\"version\":1,\"type\":\"result\",\"output\":\"done\"}\n"
	var ack bytes.Buffer
	bound := false
	result, err := readEvents(strings.NewReader(events), &ack, "", func(id string) error {
		bound = id == "thread-1"
		if ack.Len() != 0 {
			t.Fatal("ack sent before durable binding callback")
		}
		return nil
	})
	if err != nil || !bound || string(result.Output) != "done" || !strings.Contains(ack.String(), `"type":"continue"`) {
		t.Fatalf("session handshake failed: %q %q %v", result.Output, ack.String(), err)
	}
}

func TestReadEventsRejectsUnboundOrChangedSession(t *testing.T) {
	for _, events := range []string{
		"{\"version\":1,\"type\":\"result\",\"output\":\"done\"}\n",
		"{\"version\":1,\"type\":\"session\",\"session_id\":\"other\"}\n",
		"{\"version\":1,\"type\":\"session\",\"session_id\":\"thread-1\"}\n{\"version\":1,\"type\":\"session\",\"session_id\":\"thread-2\"}\n",
	} {
		var ack bytes.Buffer
		if _, err := readEvents(strings.NewReader(events), &ack, "thread-1", func(string) error { return nil }); err == nil {
			t.Fatal("unsafe event sequence accepted")
		}
	}
	var ack bytes.Buffer
	if _, err := readEvents(strings.NewReader("{\"version\":1,\"type\":\"session\",\"session_id\":\"thread-1\"}\n"), &ack, "", func(string) error { return errors.New("store failed") }); err == nil || ack.Len() != 0 {
		t.Fatalf("unpersisted session was acknowledged: %q %v", ack.String(), err)
	}
}

func TestReadEventsClassifiesFailure(t *testing.T) {
	var ack bytes.Buffer
	_, err := readEvents(strings.NewReader("{\"version\":1,\"type\":\"failure\",\"kind\":\"auth_required\"}\n"), &ack, "", func(string) error { t.Fatal("unexpected session"); return nil })
	var classified core.RunError
	if !errors.As(err, &classified) || classified.Kind != "auth_required" {
		t.Fatalf("failure classification lost: %v", err)
	}
}

func TestRunProcessWaitsForSessionAck(t *testing.T) {
	cmd := exec.Command("sh", "-c", `read request; printf '%s\n' '{"version":1,"type":"session","session_id":"thread-1"}'; read ack; case "$ack" in *'"type":"continue"'*) printf '%s\n' '{"version":1,"type":"result","output":"done"}' ;; *) exit 3 ;; esac`)
	request := workerwire.Request{Version: workerwire.Version, AttemptID: "attempt-1", Role: "implementation", Provider: "codex", Model: "gpt-6-sol", Prompt: "work"}
	result, err := runProcess(cmd, request, func(id string) error {
		if id != "thread-1" {
			t.Fatalf("wrong session: %s", id)
		}
		return nil
	})
	if err != nil || string(result.Output) != "done" {
		t.Fatalf("worker protocol failed: %q %v", result.Output, err)
	}
}

func TestBillingExpiryWithholdsWorkerModelAcknowledgement(t *testing.T) {
	runner := Runner{BeforeModel: func(context.Context) error { return errors.New("billing confirmation expired") }}
	events := `{"version":1,"type":"session","session_id":"authenticated-session"}` + "\n"
	var ack bytes.Buffer
	bound := false
	_, err := readEvents(strings.NewReader(events), &ack, "", runner.admittedSession(context.Background(), func(string) error { bound = true; return nil }))
	var classified core.RunError
	if !errors.As(err, &classified) || classified.Kind != "retry_wait" || bound || ack.Len() != 0 {
		t.Fatalf("billing pause did not withhold model permission: bound=%t ack=%q err=%v", bound, ack.String(), err)
	}
	runner.BeforeModel = func(context.Context) error { return nil }
	if err := runner.admittedSession(context.Background(), func(string) error { bound = true; return nil })("authenticated-session"); err != nil || !bound {
		t.Fatal("valid billing confirmation rejected session")
	}
}

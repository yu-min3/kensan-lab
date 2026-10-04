package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"

	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/core"
	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/workerwire"
)

func TestRelayKillsOnlyAfterRealBoundResult(t *testing.T) {
	for _, terminal := range []string{"result", "failure"} {
		t.Run(terminal, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=TestRelayHelper")
			cmd.Env = append(os.Environ(), "RECOVERY_TEST_MODE=relay", "RECOVERY_TEST_TERMINAL="+terminal)
			req := workerwire.Request{Version: 1, AttemptID: "offline-relay-test", Role: "feedback", Provider: "claude", Model: "claude-opus-5-5", Prompt: "synthetic offline fixture"}
			var input bytes.Buffer
			json.NewEncoder(&input).Encode(req)
			json.NewEncoder(&input).Encode(workerwire.Ack{Version: 1, Type: "continue", SessionID: "offline-test-session"})
			cmd.Stdin = &input
			output, err := cmd.Output()
			if terminal == "result" {
				failure, ok := err.(*exec.ExitError)
				if !ok {
					t.Fatalf("expected SIGKILL, got %v", err)
				}
				status, ok := failure.Sys().(syscall.WaitStatus)
				if !ok || !status.Signaled() || status.Signal() != syscall.SIGKILL {
					t.Fatalf("wrong exit: %v", failure)
				}
				if !strings.Contains(string(output), `"type":"fault_injected"`) {
					t.Fatal("trusted fault proof missing")
				}
				if strings.Contains(string(output), "must-not-reach-controller") || strings.Contains(string(output), `"type":"result"`) {
					t.Fatal("terminal result was forwarded")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(string(output), `"type":"failure"`) {
					t.Fatal("provider failure was hidden")
				}
			}
			if !strings.Contains(string(output), `"type":"session"`) {
				t.Fatal("session binding was lost")
			}
		})
	}
}

func TestRelayHelper(t *testing.T) {
	mode := os.Getenv("RECOVERY_TEST_MODE")
	if mode == "" {
		return
	}
	if mode == "relay" {
		cmd := exec.Command(os.Args[0], "-test.run=TestRelayHelper")
		cmd.Env = append(os.Environ(), "RECOVERY_TEST_MODE=worker")
		if err := relayCommand(cmd, false); err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
	decoder := json.NewDecoder(os.Stdin)
	var req workerwire.Request
	if decoder.Decode(&req) != nil {
		os.Exit(2)
	}
	json.NewEncoder(os.Stdout).Encode(workerwire.Event{Version: 1, Type: "session", SessionID: "offline-test-session"})
	var ack workerwire.Ack
	if decoder.Decode(&ack) != nil || ack.Validate("offline-test-session") != nil {
		os.Exit(3)
	}
	event := workerwire.Event{Version: 1, Type: os.Getenv("RECOVERY_TEST_TERMINAL")}
	if event.Type == "result" {
		event.Output = "must-not-reach-controller"
	} else {
		event.Kind = "quota_wait"
	}
	json.NewEncoder(os.Stdout).Encode(event)
	os.Exit(0)
}

type proofRunner struct{ body []byte }

func (r proofRunner) Run(_ context.Context, d core.Dispatch) (core.RunResult, error) {
	if err := d.BindSession("fixture-old-session"); err != nil {
		return core.RunResult{}, err
	}
	return core.RunResult{Output: r.body}, core.RunError{Kind: "failed", Err: errors.New("offline injected failure")}
}
func TestKillProofRejectsOrdinaryFailures(t *testing.T) {
	for _, mode := range []string{"missing", "wrong-exit", "verified"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			if err := os.Chmod(root, 0700); err != nil {
				t.Fatal(err)
			}
			store, err := core.Open(root)
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			if err = store.SeedKnowledge(); err != nil {
				t.Fatal(err)
			}
			_, _, err = store.CreatePlannedTask(mission, core.Platform, "analysis", "offline proof audit", "recovery-v1")
			if err != nil {
				t.Fatal(err)
			}
			var body []byte
			if mode != "missing" {
				proof := faultProof{Version: 1, Type: "fault_injected", SessionID: "fixture-old-session", ResultSHA256: strings.Repeat("a", 64), ResultBytes: 1, ChildExit: 0, RelayExit: "SIGKILL"}
				if mode == "wrong-exit" {
					proof.RelayExit = "exit-1"
				}
				body, _ = json.Marshal(proof)
			}
			if worked, err := store.Tick(context.Background(), proofRunner{body: body}, nil); err != nil || !worked {
				t.Fatalf("tick %v %v", worked, err)
			}
			err = verifyKillProofs(store)
			if (err == nil) != (mode == "verified") {
				t.Fatalf("%s verification: %v", mode, err)
			}
		})
	}
}

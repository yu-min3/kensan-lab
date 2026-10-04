package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"

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

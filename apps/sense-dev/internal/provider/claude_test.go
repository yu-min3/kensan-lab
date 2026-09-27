package provider

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClaudeRejectsBillingOverrides(t *testing.T) {
	for _, settings := range []string{
		`{"apiKeyHelper":"get-key"}`,
		`{"env":{"ANTHROPIC_API_KEY":"hidden"}}`,
		`{"fallbackModel":"sonnet"}`,
		`{"env":{"CLAUDE_CODE_USE_BEDROCK":"1"}}`,
	} {
		dir := t.TempDir()
		if err := os.Chmod(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(settings), 0600); err != nil {
			t.Fatal(err)
		}
		if err := (Claude{AuthHome: dir}).checkSettings(); err == nil {
			t.Fatalf("accepted billing override: %s", settings)
		}
	}
}

func TestClaudeSubscriptionOnlyAndPromptOnStdin(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	const session = "12345678-1234-4234-8234-123456789abc"
	script := filepath.Join(dir, "fake-claude")
	content := `#!/bin/sh
if [ "$1" = "auth" ]; then
  printf '%s\n' '{"loggedIn":true,"authMethod":"claude.ai","subscriptionType":"pro","apiProvider":"firstParty"}'
  exit 0
fi
if [ -n "${ANTHROPIC_API_KEY:-}" ]; then exit 8; fi
case "$*" in *secret-from-prompt*) exit 9;; esac
read -r line
case "$line" in *secret-from-prompt*) ;; *) exit 10;; esac
printf '%s\n' '{"type":"result","subtype":"success","is_error":false,"result":"requirements","session_id":"12345678-1234-4234-8234-123456789abc","modelUsage":{"claude-fable-5":{}}}'
`
	if err := os.WriteFile(script, []byte(content), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ANTHROPIC_API_KEY", "test-key-must-not-reach-child")
	bound := false
	result, err := (Claude{Binary: script, AuthHome: dir}).Run(context.Background(), Request{Model: "fable", Workdir: dir, Prompt: "secret-from-prompt", ExistingSession: session, OnSession: func(id string) error { bound = id == session; return nil }})
	if err != nil || !bound || result.Text != "requirements" || result.SessionID != session {
		t.Fatalf("result=%+v bound=%t err=%v", result, bound, err)
	}
}

func TestClaudeResultFailsClosedOnReroute(t *testing.T) {
	_, err := parseClaudeResult([]byte(`{"type":"result","subtype":"success","result":"ok","session_id":"id","modelUsage":{"claude-sonnet-5":{}}}`), "id", "fable")
	if !errors.Is(err, ErrModelChanged) {
		t.Fatalf("expected model mismatch, got %v", err)
	}
	_, err = parseClaudeResult([]byte(`{"type":"result","subtype":"success","result":"ok","session_id":"id"}`), "id", "fable")
	if !errors.Is(err, ErrModelChanged) {
		t.Fatalf("missing model usage accepted: %v", err)
	}
	if !strings.Contains(classifyClaudeFailure(errors.New("exit 1"), "usage limit reached").Error(), "quota") {
		t.Fatal("quota not classified")
	}
}

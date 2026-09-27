package workerwire

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDecodeRequestRejectsUnknownAndTrailingFields(t *testing.T) {
	valid := Request{Version: Version, AttemptID: "attempt-1", Provider: "codex", Model: "gpt-6-sol", Prompt: "Implement contract"}
	b, err := json.Marshal(valid)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := DecodeRequest(strings.NewReader(string(b))); err != nil || got.AttemptID != valid.AttemptID {
		t.Fatalf("valid request rejected: %+v %v", got, err)
	}
	for _, payload := range []string{
		`{"version":1,"attempt_id":"a","provider":"claude","model":"opus","prompt":"review","admin_token":"secret"}`,
		string(b) + ` {}`,
		`{"version":1,"attempt_id":"a","provider":"codex","model":"unknown","prompt":"review"}`,
		`{"version":1,"attempt_id":"a","provider":"claude","model":"opus","prompt":"` + strings.Repeat("x", MaxPrompt+1) + `"}`,
	} {
		if _, err := DecodeRequest(strings.NewReader(payload)); err == nil {
			t.Fatal("unsafe worker request accepted")
		}
	}
}

func TestEventsAreConstrained(t *testing.T) {
	good := []Event{
		{Version: Version, Type: "session", SessionID: "session-1"},
		{Version: Version, Type: "result", Output: "accepted"},
		{Version: Version, Type: "failure", Kind: "quota_wait"},
	}
	for _, event := range good {
		if err := event.Validate(); err != nil {
			t.Fatalf("valid event rejected: %+v %v", event, err)
		}
	}
	bad := []Event{
		{Version: Version, Type: "result", Output: ""},
		{Version: Version, Type: "failure", Kind: "publish"},
		{Version: Version, Type: "session", SessionID: ""},
		{Version: Version, Type: "session", SessionID: "s", Output: "secret"},
		{Version: Version, Type: "result", SessionID: "s", Output: "done"},
	}
	for _, event := range bad {
		if err := event.Validate(); err == nil {
			t.Fatalf("invalid event accepted: %+v", event)
		}
	}
}

func TestAckMustMatchPersistedSession(t *testing.T) {
	if err := (Ack{Version: Version, Type: "continue", SessionID: "thread-1"}).Validate("thread-1"); err != nil {
		t.Fatal(err)
	}
	if err := (Ack{Version: Version, Type: "continue", SessionID: "thread-2"}).Validate("thread-1"); err == nil {
		t.Fatal("wrong session acknowledged")
	}
}

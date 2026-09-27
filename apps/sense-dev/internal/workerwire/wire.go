// Package workerwire defines the only data exchanged between the controller
// and an isolated model worker. It deliberately has no filesystem or token
// fields: mount paths and binaries are fixed by the controller's launch policy.
package workerwire

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

const Version = 1
const MaxPrompt = 2 << 20
const MaxOutput = 4 << 20

type Request struct {
	Version         int    `json:"version"`
	AttemptID       string `json:"attempt_id"`
	Provider        string `json:"provider"`
	Model           string `json:"model"`
	Prompt          string `json:"prompt"`
	ExistingSession string `json:"existing_session,omitempty"`
}

type Event struct {
	Version   int    `json:"version"`
	Type      string `json:"type"` // session, result, or failure
	SessionID string `json:"session_id,omitempty"`
	Output    string `json:"output,omitempty"`
	Kind      string `json:"kind,omitempty"`
}

// Ack is sent only after the controller has durably bound the session to its
// agent and attempt. The worker must not start a model turn before receiving it.
type Ack struct {
	Version   int    `json:"version"`
	Type      string `json:"type"`
	SessionID string `json:"session_id"`
}

func (a Ack) Validate(expected string) error {
	if a.Version != Version || a.Type != "continue" || expected == "" || a.SessionID != expected {
		return errors.New("worker session acknowledgement mismatch")
	}
	return nil
}

func (r Request) Validate() error {
	if r.Version != Version || len(r.AttemptID) == 0 || len(r.AttemptID) > 128 || strings.TrimSpace(r.Prompt) == "" || len(r.Prompt) > MaxPrompt || len(r.ExistingSession) > 256 {
		return errors.New("invalid worker request envelope")
	}
	switch r.Provider + ":" + r.Model {
	case "claude:fable", "claude:opus", "codex:gpt-6-astra", "codex:gpt-6-sol":
		return nil
	default:
		return errors.New("unapproved provider/model combination")
	}
}

func (e Event) Validate() error {
	if e.Version != Version || len(e.SessionID) > 256 {
		return errors.New("invalid worker event envelope")
	}
	switch e.Type {
	case "session":
		if e.SessionID == "" || e.Output != "" || e.Kind != "" {
			return errors.New("invalid session event")
		}
	case "result":
		if e.Output == "" || len(e.Output) > MaxOutput || e.Kind != "" || e.SessionID != "" {
			return errors.New("invalid result event")
		}
	case "failure":
		if e.Output != "" || e.SessionID != "" || !validFailureKind(e.Kind) {
			return errors.New("invalid failure event")
		}
	default:
		return errors.New("unknown worker event")
	}
	return nil
}

func validFailureKind(kind string) bool {
	switch kind {
	case "auth_required", "quota_wait", "retry_wait", "failed", "interrupted":
		return true
	default:
		return false
	}
}

// DecodeRequest reads a single bounded JSON message and rejects trailing data.
// A model cannot smuggle arbitrary paths or credentials through extra fields.
func DecodeRequest(input io.Reader) (Request, error) {
	var r Request
	payload, err := io.ReadAll(io.LimitReader(input, MaxPrompt+4097))
	if err != nil {
		return r, err
	}
	if len(payload) > MaxPrompt+4096 {
		return r, errors.New("worker request exceeds size limit")
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&r); err != nil {
		return r, err
	}
	if err := r.Validate(); err != nil {
		return r, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return r, fmt.Errorf("worker request has trailing data: %v", err)
	}
	return r, nil
}

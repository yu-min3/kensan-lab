package provider

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Claude uses a dedicated, user-authenticated Claude Code configuration.
// The adapter never accepts API credentials from the controller environment.
// Its initial tool surface is read-only; write-capable execution requires an
// OS-isolated worker and is intentionally not enabled by this adapter.
type Claude struct {
	Binary   string
	AuthHome string
}

func (c Claude) Run(ctx context.Context, req Request) (Result, error) {
	if c.Binary == "" || c.AuthHome == "" || req.Workdir == "" || req.Prompt == "" || req.OnSession == nil {
		return Result{}, errors.New("Claude binary, auth home, workdir, prompt and session callback required")
	}
	if req.Model != "fable" && req.Model != "opus" {
		return Result{}, errors.New("Claude model must be the configured Fable or Opus stage")
	}
	if err := c.checkSettings(); err != nil {
		return Result{}, err
	}
	if err := c.checkSubscription(ctx); err != nil {
		return Result{}, err
	}
	sessionID := req.ExistingSession
	if sessionID == "" {
		var err error
		sessionID, err = newUUID()
		if err != nil {
			return Result{}, err
		}
	}
	if err := req.OnSession(sessionID); err != nil {
		return Result{}, err
	}
	args := []string{"-p", "--output-format", "json", "--model", req.Model, "--permission-mode", "dontAsk", "--tools", "Read", "--strict-mcp-config", "--safe-mode", "--no-chrome", "--disable-slash-commands"}
	if req.ExistingSession != "" {
		args = append(args, "--resume", sessionID)
	} else {
		args = append(args, "--session-id", sessionID)
	}
	cmd := exec.CommandContext(ctx, c.Binary, args...)
	cmd.Dir = req.Workdir
	cmd.Env = c.cleanEnv()
	cmd.Stdin = strings.NewReader(req.Prompt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return Result{SessionID: sessionID}, classifyClaudeFailure(err, stderr.String())
	}
	return parseClaudeResult(stdout.Bytes(), sessionID, req.Model)
}

func (c Claude) cleanEnv() []string {
	return []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME"), "CLAUDE_CONFIG_DIR=" + c.AuthHome}
}

func (c Claude) checkSettings() error {
	info, err := os.Stat(c.AuthHome)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return errors.New("Claude config directory must exist and be private")
	}
	for _, name := range []string{"settings.json", "settings.local.json"} {
		b, err := os.ReadFile(filepath.Join(c.AuthHome, name))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		var value any
		if err := json.Unmarshal(b, &value); err != nil {
			return errors.New("Claude settings are not valid JSON")
		}
		if containsBillingOverride(value) {
			return errors.New("Claude settings contain billing or model fallback override")
		}
	}
	return nil
}

func containsBillingOverride(v any) bool {
	switch x := v.(type) {
	case map[string]any:
		for key, child := range x {
			switch strings.ToLower(key) {
			case "apikeyhelper", "fallbackmodel", "anthropic_api_key", "anthropic_auth_token", "anthropic_base_url", "claude_code_use_bedrock", "claude_code_use_vertex", "claude_code_use_foundry":
				return true
			}
			if containsBillingOverride(child) {
				return true
			}
		}
	case []any:
		for _, child := range x {
			if containsBillingOverride(child) {
				return true
			}
		}
	}
	return false
}

func (c Claude) checkSubscription(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, c.Binary, "auth", "status")
	cmd.Env = c.cleanEnv()
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return ErrAuthRequired
	}
	var status struct {
		LoggedIn         bool   `json:"loggedIn"`
		AuthMethod       string `json:"authMethod"`
		SubscriptionType string `json:"subscriptionType"`
		APIProvider      string `json:"apiProvider"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &status); err != nil || !status.LoggedIn || status.SubscriptionType == "" || strings.Contains(strings.ToLower(status.AuthMethod), "api") || strings.Contains(strings.ToLower(status.APIProvider), "bedrock") || strings.Contains(strings.ToLower(status.APIProvider), "vertex") || strings.Contains(strings.ToLower(status.APIProvider), "foundry") {
		return ErrAuthRequired
	}
	return nil
}

func parseClaudeResult(b []byte, expectedSession, requestedModel string) (Result, error) {
	var response struct {
		Type       string                     `json:"type"`
		Subtype    string                     `json:"subtype"`
		IsError    bool                       `json:"is_error"`
		Result     string                     `json:"result"`
		SessionID  string                     `json:"session_id"`
		ModelUsage map[string]json.RawMessage `json:"modelUsage"`
	}
	if err := json.Unmarshal(b, &response); err != nil {
		return Result{}, errors.New("invalid Claude JSON response")
	}
	if response.Type != "result" || response.IsError || response.Subtype != "success" || response.SessionID != expectedSession || strings.TrimSpace(response.Result) == "" {
		return Result{SessionID: expectedSession}, errors.New("Claude turn did not complete with expected session")
	}
	if len(response.ModelUsage) == 0 {
		return Result{SessionID: expectedSession}, ErrModelChanged
	}
	for model := range response.ModelUsage {
		if !strings.Contains(strings.ToLower(model), requestedModel) {
			return Result{SessionID: expectedSession}, fmt.Errorf("%w: %s", ErrModelChanged, model)
		}
	}
	return Result{SessionID: expectedSession, Text: response.Result, Status: "completed"}, nil
}

func classifyClaudeFailure(err error, stderr string) error {
	lower := strings.ToLower(stderr)
	if strings.Contains(lower, "rate limit") || strings.Contains(lower, "usage limit") || strings.Contains(lower, "quota") {
		return ErrQuotaWait
	}
	if strings.Contains(lower, "login") || strings.Contains(lower, "authentication") || strings.Contains(lower, "unauthorized") {
		return ErrAuthRequired
	}
	return err
}

func newUUID() (string, error) {
	var b [16]byte
	if _, err := io.ReadFull(rand.Reader, b[:]); err != nil {
		return "", err
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	encoded := hex.EncodeToString(b[:])
	return encoded[:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:], nil
}

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"

	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/provider"
	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/workerwire"
)

const workdir = "/workspace"
const authHome = "/agent-auth"

func main() {
	claude := flag.String("claude-bin", "/usr/local/bin/claude", "Claude CLI path inside sandbox")
	codex := flag.String("codex-bin", "/usr/local/bin/codex", "Codex CLI path inside sandbox")
	maxUsage := flag.Float64("max-codex-usage", 80, "maximum accepted subscription usage percent")
	preflight := flag.Bool("preflight", false, "verify worker mount boundary without running a model")
	hiddenPath := flag.String("hidden-path", "", "controller state path that must be absent inside the sandbox")
	preflightProvider := flag.String("provider", "", "provider binary to verify during preflight")
	flag.Parse()
	if *preflight {
		binary := *codex
		if *preflightProvider == "claude" {
			binary = *claude
		} else if *preflightProvider != "codex" {
			os.Exit(2)
		}
		if err := checkPreflight(workdir, authHome, *hiddenPath, binary); err != nil {
			os.Exit(1)
		}
		_ = json.NewEncoder(os.Stdout).Encode(struct {
			Version int    `json:"version"`
			Type    string `json:"type"`
		}{workerwire.Version, "preflight_ok"})
		return
	}
	if *maxUsage <= 0 || *maxUsage > 100 {
		os.Exit(2)
	}
	input := bufio.NewReaderSize(os.Stdin, workerwire.MaxPrompt+4096)
	line, err := input.ReadSlice('\n')
	if err != nil {
		os.Exit(2)
	}
	request, err := workerwire.DecodeRequest(bytes.NewReader(line))
	if err != nil {
		os.Exit(2)
	}
	if err := execute(context.Background(), request, input, os.Stdout, *claude, *codex, *maxUsage); err != nil {
		os.Exit(1)
	}
}

func checkPreflight(worktree, auth, hidden, binary string) error {
	if hidden == "" || hidden == "/" || binary == "" {
		return errors.New("preflight paths are incomplete")
	}
	if _, err := os.Lstat(hidden); !errors.Is(err, os.ErrNotExist) {
		return errors.New("controller state path is visible or unverifiable")
	}
	for _, dir := range []string{worktree, auth} {
		file, err := os.CreateTemp(dir, ".worker-preflight-")
		if err != nil {
			return errors.New("worker writable mount unavailable")
		}
		name := file.Name()
		if err := file.Close(); err != nil {
			return err
		}
		if err := os.Remove(name); err != nil {
			return err
		}
	}
	info, err := os.Stat(binary)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
		return errors.New("provider binary unavailable in runtime")
	}
	return nil
}

func execute(ctx context.Context, request workerwire.Request, input *bufio.Reader, output io.Writer, claudeBin, codexBin string, maxUsage float64) error {
	if err := request.Validate(); err != nil {
		return err
	}
	encoder := json.NewEncoder(output)
	onSession := func(id string) error {
		event := workerwire.Event{Version: workerwire.Version, Type: "session", SessionID: id}
		if err := event.Validate(); err != nil {
			return err
		}
		if err := encoder.Encode(event); err != nil {
			return err
		}
		line, err := input.ReadSlice('\n')
		if err != nil || len(line) > 512 {
			return errors.New("controller did not acknowledge session")
		}
		var ack workerwire.Ack
		if err := json.Unmarshal(line, &ack); err != nil {
			return err
		}
		return ack.Validate(id)
	}
	providerRequest := provider.Request{Model: request.Model, Workdir: workdir, Prompt: request.Prompt, ExistingSession: request.ExistingSession, OnSession: onSession}
	var result provider.Result
	var err error
	if request.Provider == "claude" {
		result, err = (provider.Claude{Binary: claudeBin, AuthHome: authHome}).Run(ctx, providerRequest)
	} else {
		result, err = (provider.Codex{Binary: codexBin, AuthHome: authHome, MaxUsage: maxUsage}).Run(ctx, providerRequest)
	}
	if err != nil {
		kind := "failed"
		if errors.Is(err, provider.ErrAuthRequired) {
			kind = "auth_required"
		} else if errors.Is(err, provider.ErrQuotaWait) {
			kind = "quota_wait"
		} else if errors.Is(err, context.DeadlineExceeded) {
			kind = "retry_wait"
		}
		return encoder.Encode(workerwire.Event{Version: workerwire.Version, Type: "failure", Kind: kind})
	}
	return encoder.Encode(workerwire.Event{Version: workerwire.Version, Type: "result", Output: result.Text})
}

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
	"os/exec"
	"path/filepath"
	"syscall"

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
	preflightTask := flag.Bool("preflight-task", false, "verify a task checkout inside the sandbox without running a model")
	hiddenPath := flag.String("hidden-path", "", "controller state path that must be absent inside the sandbox")
	preflightProvider := flag.String("provider", "", "provider binary to verify during preflight")
	readOnlyWorktree := flag.Bool("read-only-worktree", false, "assert worktree cannot be written in this sandbox")
	flag.Parse()
	if *preflight || *preflightTask {
		binary := *codex
		if *preflightProvider == "claude" {
			binary = *claude
		} else if *preflightProvider != "codex" {
			os.Exit(2)
		}
		if err := checkPreflight(workdir, authHome, *hiddenPath, binary, *readOnlyWorktree); err != nil {
			os.Exit(1)
		}
		statusType := "preflight_ok"
		if *preflightTask {
			if err := checkTaskGit(workdir); err != nil {
				os.Exit(1)
			}
			statusType = "task_git_ok"
		}
		_ = json.NewEncoder(os.Stdout).Encode(struct {
			Version int    `json:"version"`
			Type    string `json:"type"`
		}{workerwire.Version, statusType})
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

func checkTaskGit(worktree string) error {
	metadata, err := os.Lstat(filepath.Join(worktree, ".git"))
	if err != nil || !metadata.IsDir() || metadata.Mode()&os.ModeSymlink != 0 {
		return errors.New("task Git metadata is not self-contained")
	}
	if _, err := os.Lstat(filepath.Join(worktree, ".git", "objects", "info", "alternates")); !errors.Is(err, os.ErrNotExist) {
		return errors.New("task Git metadata shares external objects")
	}
	git := func(args ...string) (string, error) {
		argv := append([]string{"-c", "core.hooksPath=/dev/null", "-C", worktree}, args...)
		cmd := exec.Command("git", argv...)
		cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME"), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0"}
		output, err := cmd.Output()
		return string(bytes.TrimSpace(output)), err
	}
	top, err := git("rev-parse", "--show-toplevel")
	if err != nil || top != worktree {
		return errors.New("task Git checkout is not rooted at the mounted worktree")
	}
	remotes, err := git("remote")
	if err != nil || remotes != "" {
		return errors.New("task Git checkout has a remote")
	}
	if _, err := git("status", "--porcelain", "--untracked-files=no"); err != nil {
		return errors.New("task Git status failed in worker sandbox")
	}
	return nil
}

func checkPreflight(worktree, auth, hidden, binary string, readOnlyWorktree bool) error {
	if hidden == "" || hidden == "/" || binary == "" {
		return errors.New("preflight paths are incomplete")
	}
	if _, err := os.Lstat(hidden); !errors.Is(err, os.ErrNotExist) {
		return errors.New("controller state path is visible or unverifiable")
	}
	if _, err := os.ReadDir(worktree); err != nil {
		return errors.New("worktree cannot be read")
	}
	authFile, err := os.CreateTemp(auth, ".worker-preflight-")
	if err != nil {
		return errors.New("provider auth mount is not writable")
	}
	authName := authFile.Name()
	if err := authFile.Close(); err != nil {
		return err
	}
	if err := os.Remove(authName); err != nil {
		return err
	}
	worktreeFile, err := os.CreateTemp(worktree, ".worker-preflight-")
	if readOnlyWorktree {
		if err == nil {
			name := worktreeFile.Name()
			_ = worktreeFile.Close()
			_ = os.Remove(name)
			return errors.New("review worktree is unexpectedly writable")
		}
		if !errors.Is(err, syscall.EROFS) {
			return errors.New("review worktree read-only mount could not be verified")
		}
	} else {
		if err != nil {
			return errors.New("implementation worktree is not writable")
		}
		name := worktreeFile.Name()
		if err := worktreeFile.Close(); err != nil {
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

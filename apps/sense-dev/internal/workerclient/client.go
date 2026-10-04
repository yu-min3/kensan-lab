package workerclient

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/core"
	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/isolation"
	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/workerwire"
	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/worktree"
)

// Runner uses a separate bubblewrap process for every provider turn. No
// controller state path, admin token, or publisher credential is sent over the
// protocol. Installation must still preflight the actual host namespaces.
type Runner struct {
	Claude        isolation.Config
	Codex         isolation.Config
	WorkerProgram string
	Timeout       time.Duration
	Worktrees     worktree.Manager
	// Host-only admission check; no billing state or credentials enter worker IPC.
	BeforeModel func(context.Context) error
}

// Preflight executes the trusted worker inside both sandboxes before the
// controller enables automatic dispatch. It proves only this host's current
// mount/namespace setup, not subscription authentication or egress policy.
func (r Runner) Preflight(ctx context.Context) error {
	if r.WorkerProgram == "" || r.Timeout <= 0 {
		return errors.New("isolated worker configuration incomplete")
	}
	if err := worktree.PrepareSandboxMetadata(r.Codex.Worktree); err != nil {
		return fmt.Errorf("Codex sandbox mount targets unavailable: %w", err)
	}
	claudeReadOnly, codexReadOnly := r.Claude, r.Codex
	claudeReadOnly.ReadOnlyWorktree, codexReadOnly.ReadOnlyWorktree = true, true
	codexWritable := r.Codex
	codexWritable.ReadOnlyWorktree = false
	for _, item := range []struct {
		name     string
		config   isolation.Config
		readOnly bool
	}{{"claude", claudeReadOnly, true}, {"codex", codexReadOnly, true}, {"codex", codexWritable, false}} {
		statePath, err := filepath.EvalSymlinks(item.config.ControllerState)
		if err != nil {
			return fmt.Errorf("%s state path unavailable: %w", item.name, err)
		}
		checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		cmd, err := item.config.Command(checkCtx, r.WorkerProgram, "-preflight", "-hidden-path", statePath, "-provider", item.name, fmt.Sprintf("-read-only-worktree=%t", item.readOnly))
		if err != nil {
			cancel()
			return fmt.Errorf("%s isolation configuration rejected: %w", item.name, err)
		}
		cmd.Stderr = io.Discard
		output, err := cmd.Output()
		cancel()
		if err != nil || len(output) > 128 {
			return fmt.Errorf("%s sandbox preflight failed", item.name)
		}
		var status struct {
			Version int    `json:"version"`
			Type    string `json:"type"`
		}
		if json.Unmarshal(output, &status) != nil || status.Version != workerwire.Version || status.Type != "preflight_ok" {
			return fmt.Errorf("%s sandbox preflight returned invalid evidence", item.name)
		}
	}
	return nil
}

func (r Runner) Run(ctx context.Context, dispatch core.Dispatch) (core.RunResult, error) {
	if dispatch.BindSession == nil || r.WorkerProgram == "" || r.Timeout <= 0 {
		return core.RunResult{}, errors.New("isolated worker configuration incomplete")
	}
	request := workerwire.Request{Version: workerwire.Version, AttemptID: dispatch.Attempt.ID, Role: dispatch.Attempt.Role, Provider: dispatch.Attempt.Provider, Model: dispatch.Attempt.Model, Prompt: dispatch.Prompt, ExistingSession: dispatch.Attempt.SessionID}
	if err := request.Validate(); err != nil {
		return core.RunResult{}, err
	}
	if dispatch.Attempt.BaseSHA == "" {
		return core.RunResult{}, errors.New("task base SHA must be pinned before isolated dispatch")
	}
	var taskWorktree, pinnedBase string
	var err error
	if dispatch.Manifest.SourceTaskID != "" {
		acceptance := request.Role == "app_acceptance" && dispatch.Manifest.Team == core.App && dispatch.Attempt.BaseSHA == dispatch.Attempt.HeadSHA
		gate := request.Role == "release_gate" && dispatch.Manifest.Team == core.Platform
		if (!acceptance && !gate) || dispatch.Attempt.HeadSHA != dispatch.Manifest.HeadSHA {
			return core.RunResult{}, errors.New("linked consumer revision is not a pinned App acceptance or independent Platform Gate")
		}
		taskWorktree, pinnedBase, err = r.Worktrees.EnsureFromTask(ctx, dispatch.Attempt.TaskID, dispatch.Manifest.SourceTaskID, dispatch.Attempt.HeadSHA)
	} else if dispatch.Manifest.ImageSourceTaskID != "" {
		if dispatch.Manifest.Team != core.App || dispatch.Manifest.ImageEvidence == nil {
			return core.RunResult{}, errors.New("image deployment requires pinned App CI evidence")
		}
		taskWorktree, pinnedBase, err = r.Worktrees.EnsureImageDeployment(ctx, dispatch.Attempt.TaskID, dispatch.Manifest.ImageSourceTaskID, dispatch.Attempt.BaseSHA, dispatch.Manifest.ImageSourceSHA, true)
	} else if dispatch.Manifest.CheckoutSourceTaskID != "" {
		if dispatch.Manifest.Team != core.Platform || dispatch.Attempt.BaseSHA == "" {
			return core.RunResult{}, errors.New("derived checkout requires a pinned Platform task")
		}
		// The logical App source is a reviewed PR head. The deployed merge
		// revision is supplied by the operator-owned immutable source repo.
		taskWorktree, pinnedBase, err = r.Worktrees.Ensure(ctx, dispatch.Attempt.TaskID, dispatch.Attempt.BaseSHA)
	} else {
		taskWorktree, pinnedBase, err = r.Worktrees.Ensure(ctx, dispatch.Attempt.TaskID, dispatch.Attempt.BaseSHA)
	}
	expectedBase := dispatch.Attempt.BaseSHA
	if dispatch.Manifest.SourceTaskID != "" && request.Role == "release_gate" {
		expectedBase = dispatch.Attempt.HeadSHA
	}
	if err != nil || pinnedBase != expectedBase {
		return core.RunResult{}, errors.New("task-scoped worktree failed validation")
	}
	config := r.Codex
	if request.Provider == "claude" {
		config = r.Claude
	}
	config.ReadOnlyWorktree = request.Model != "gpt-6-sol"
	config.Worktree = taskWorktree
	if !config.ReadOnlyWorktree {
		if err := worktree.PrepareSandboxMetadata(taskWorktree); err != nil {
			return core.RunResult{}, err
		}
	}
	if err := r.preflightTask(ctx, config, request.Provider); err != nil {
		return core.RunResult{}, err
	}
	turnCtx, cancel := context.WithTimeout(ctx, r.Timeout)
	defer cancel()
	cmd, err := config.Command(turnCtx, r.WorkerProgram)
	if err != nil {
		return core.RunResult{}, err
	}
	if r.BeforeModel != nil {
		if err := r.BeforeModel(turnCtx); err != nil {
			return core.RunResult{}, core.RunError{Kind: "retry_wait", Err: err}
		}
	}
	result, err := runProcess(cmd, request, r.admittedSession(turnCtx, dispatch.BindSession))
	if errors.Is(turnCtx.Err(), context.DeadlineExceeded) {
		return core.RunResult{}, core.RunError{Kind: "retry_wait", Err: context.DeadlineExceeded}
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		return core.RunResult{}, core.RunError{Kind: "interrupted", Err: context.Canceled}
	}
	if err != nil {
		return core.RunResult{}, err
	}
	if request.Role == "implementation" {
		var verifyErr error
		if dispatch.Manifest.ImageSourceTaskID != "" {
			_, _, verifyErr = r.Worktrees.EnsureImageDeployment(ctx, dispatch.Attempt.TaskID, dispatch.Manifest.ImageSourceTaskID, dispatch.Attempt.BaseSHA, dispatch.Manifest.ImageSourceSHA, false)
		} else {
			_, _, verifyErr = r.Worktrees.Ensure(ctx, dispatch.Attempt.TaskID, dispatch.Attempt.BaseSHA)
		}
		if verifyErr != nil {
			return core.RunResult{}, errors.New("task checkout changed identity after implementation")
		}
		change, err := worktree.CommitImplementation(ctx, taskWorktree, dispatch.Attempt.BaseSHA)
		if err != nil {
			return core.RunResult{}, fmt.Errorf("implementation change could not be captured: %w", err)
		}
		if !change.Clean || change.HeadSHA == change.BaseSHA {
			return core.RunResult{}, errors.New("supervisor could not fix a clean implementation candidate before review")
		}
		body, err := json.Marshal(struct {
			SchemaVersion int               `json:"schema_version"`
			ModelOutput   string            `json:"model_output"`
			Change        worktree.Snapshot `json:"change"`
		}{SchemaVersion: 1, ModelOutput: string(result.Output), Change: change})
		if err != nil || len(body) > 2<<20 {
			return core.RunResult{}, errors.New("implementation evidence exceeds artifact limit")
		}
		result.Output = body
		result.HeadSHA = change.HeadSHA
	}
	return result, err
}

// A worker asks for an acknowledgement AFTER authenticating and BEFORE sending
// a model turn. Recheck admission here so auth/preflight delays cannot reuse an
// expired billing confirmation.
func (r Runner) admittedSession(ctx context.Context, bind func(string) error) func(string) error {
	return func(id string) error {
		if r.BeforeModel != nil {
			if err := r.BeforeModel(ctx); err != nil {
				return core.RunError{Kind: "retry_wait", Err: err}
			}
		}
		return bind(id)
	}
}

func (r Runner) preflightTask(ctx context.Context, config isolation.Config, provider string) error {
	statePath, err := filepath.EvalSymlinks(config.ControllerState)
	if err != nil {
		return err
	}
	checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd, err := config.Command(checkCtx, r.WorkerProgram, "-preflight-task", "-hidden-path", statePath, "-provider", provider, fmt.Sprintf("-read-only-worktree=%t", config.ReadOnlyWorktree))
	if err != nil {
		return err
	}
	cmd.Stderr = io.Discard
	output, err := cmd.Output()
	if err != nil || len(output) > 128 {
		return errors.New("task sandbox preflight failed")
	}
	var status struct {
		Version int    `json:"version"`
		Type    string `json:"type"`
	}
	if json.Unmarshal(output, &status) != nil || status.Version != workerwire.Version || status.Type != "task_git_ok" {
		return errors.New("task sandbox preflight returned invalid evidence")
	}
	return nil
}

func runProcess(cmd *exec.Cmd, request workerwire.Request, bindSession func(string) error) (core.RunResult, error) {
	input, err := cmd.StdinPipe()
	if err != nil {
		return core.RunResult{}, err
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		return core.RunResult{}, err
	}
	// Provider stderr is deliberately discarded. It can contain prompts,
	// credentials, or model output and must not enter controller logs.
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return core.RunResult{}, err
	}
	reaped := false
	defer func() {
		_ = input.Close()
		if !reaped && cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()
	if err := json.NewEncoder(input).Encode(request); err != nil {
		return core.RunResult{}, err
	}
	result, err := readEvents(output, input, request.ExistingSession, bindSession)
	if err != nil {
		return core.RunResult{}, err
	}
	if err := cmd.Wait(); err != nil {
		reaped = true
		return core.RunResult{}, fmt.Errorf("isolated worker exited unsuccessfully: %w", err)
	}
	reaped = true
	return result, nil
}

func readEvents(input io.Reader, ackOutput io.Writer, existingSession string, bindSession func(string) error) (core.RunResult, error) {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 64*1024), workerwire.MaxOutput+4096)
	bound := false
	terminal := false
	var result core.RunResult
	var failure error
	for scanner.Scan() {
		if terminal {
			return core.RunResult{}, errors.New("worker emitted data after terminal event")
		}
		var event workerwire.Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return core.RunResult{}, errors.New("invalid worker event JSON")
		}
		if err := event.Validate(); err != nil {
			return core.RunResult{}, err
		}
		switch event.Type {
		case "session":
			if bound || existingSession != "" && existingSession != event.SessionID {
				return core.RunResult{}, errors.New("worker session mismatch")
			}
			if err := bindSession(event.SessionID); err != nil {
				return core.RunResult{}, err
			}
			if err := json.NewEncoder(ackOutput).Encode(workerwire.Ack{Version: workerwire.Version, Type: "continue", SessionID: event.SessionID}); err != nil {
				return core.RunResult{}, err
			}
			bound = true
		case "result":
			if !bound {
				return core.RunResult{}, errors.New("worker returned result before session binding")
			}
			result.Output = []byte(event.Output)
			terminal = true
		case "failure":
			failure = core.RunError{Kind: event.Kind, Err: errors.New("isolated provider turn failed")}
			terminal = true
		}
	}
	if err := scanner.Err(); err != nil {
		return core.RunResult{}, err
	}
	if !terminal {
		return core.RunResult{}, errors.New("worker exited without terminal event")
	}
	if failure != nil {
		return core.RunResult{}, failure
	}
	return result, nil
}

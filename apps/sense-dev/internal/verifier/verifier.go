// Package verifier runs fixed, credentialless checks between implementation
// and model review. A model cannot choose commands or mark a failed check green.
package verifier

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/core"
	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/isolation"
	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/workerwire"
	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/worktree"
)

const maxCheckOutput = 96 << 10

type Check struct {
	Name           string   `json:"name"`
	Cwd            string   `json:"cwd"`
	Argv           []string `json:"argv"`
	TimeoutSeconds int      `json:"timeout_seconds"`
}

type Plan struct {
	SchemaVersion int     `json:"schema_version"`
	Checks        []Check `json:"checks"`
	SHA256        string  `json:"-"`
}

// LoadPlan accepts only an operator-controlled JSON file outside the task
// worktree and controller state. No shell parsing or model-supplied argv.
func LoadPlan(path string, forbidden ...string) (Plan, error) {
	if !filepath.IsAbs(path) {
		return Plan{}, errors.New("verification plan must have an absolute path")
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return Plan{}, err
	}
	for _, root := range forbidden {
		canonical, err := filepath.EvalSymlinks(root)
		if err != nil {
			return Plan{}, err
		}
		if within(canonical, resolved) {
			return Plan{}, errors.New("verification plan is inside a worker or state path")
		}
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0022 != 0 || info.Size() > 32<<10 {
		return Plan{}, errors.New("verification plan must be a small non-writable regular file")
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		return Plan{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var plan Plan
	if err := dec.Decode(&plan); err != nil {
		return Plan{}, err
	}
	if err := dec.Decode(new(any)); !errors.Is(err, io.EOF) {
		return Plan{}, errors.New("verification plan has trailing data")
	}
	if plan.SchemaVersion != 1 || len(plan.Checks) == 0 || len(plan.Checks) > 8 {
		return Plan{}, errors.New("verification plan needs 1-8 checks and schema version 1")
	}
	for _, check := range plan.Checks {
		if strings.TrimSpace(check.Name) == "" || len(check.Name) > 80 || check.Cwd == "" || filepath.IsAbs(check.Cwd) || check.Cwd == ".." || strings.HasPrefix(filepath.Clean(check.Cwd), ".."+string(filepath.Separator)) || len(check.Argv) == 0 || len(check.Argv) > 32 || check.TimeoutSeconds < 1 || check.TimeoutSeconds > 1800 {
			return Plan{}, errors.New("invalid verification check")
		}
		if !filepath.IsAbs(check.Argv[0]) || filepath.Clean(check.Argv[0]) != check.Argv[0] {
			return Plan{}, errors.New("verification executable must be an absolute rootfs path")
		}
		for _, arg := range check.Argv {
			if len(arg) > 4096 || strings.ContainsRune(arg, 0) {
				return Plan{}, errors.New("verification argument is invalid")
			}
		}
	}
	hash := sha256.Sum256(data)
	plan.SHA256 = hex.EncodeToString(hash[:])
	return plan, nil
}

func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

type Runner struct {
	Sandbox       isolation.Config
	Worktrees     worktree.Manager
	WorkerProgram string
	Plan          Plan
}

type checkResult struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	ExitCode   int    `json:"exit_code"`
	Output     string `json:"output"`
	DurationMS int64  `json:"duration_ms"`
}

type evidence struct {
	SchemaVersion int           `json:"schema_version"`
	BaseSHA       string        `json:"base_sha"`
	HeadSHA       string        `json:"head_sha"`
	DiffSHA256    string        `json:"diff_sha256"`
	PlanSHA256    string        `json:"plan_sha256"`
	Status        string        `json:"status"`
	Checks        []checkResult `json:"checks"`
}

func (r Runner) Preflight(ctx context.Context) error {
	if r.WorkerProgram == "" {
		return errors.New("verifier preflight worker is not configured")
	}
	hostNetNS, err := os.Readlink("/proc/self/ns/net")
	if err != nil {
		return errors.New("host network namespace cannot be identified")
	}
	statePath, err := filepath.EvalSymlinks(r.Sandbox.ControllerState)
	if err != nil {
		return err
	}
	checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd, err := r.Sandbox.VerifierCommand(checkCtx, ".", r.WorkerProgram, "-preflight-verifier", "-hidden-path", statePath, "-host-net-ns", hostNetNS)
	if err != nil {
		return err
	}
	cmd.Stderr = io.Discard
	output, err := cmd.Output()
	if err != nil || len(output) > 128 {
		return errors.New("credentialless verifier preflight failed")
	}
	var status struct {
		Version int    `json:"version"`
		Type    string `json:"type"`
	}
	if json.Unmarshal(output, &status) != nil || status.Version != workerwire.Version || status.Type != "verifier_preflight_ok" {
		return errors.New("credentialless verifier returned invalid preflight evidence")
	}
	return nil
}

func (r Runner) Run(ctx context.Context, dispatch core.Dispatch) (core.RunResult, error) {
	a := dispatch.Attempt
	if a.Role != "verification" || a.Provider != "system" || a.Model != "local" || a.BaseSHA == "" || a.HeadSHA == "" || r.Plan.SHA256 == "" {
		return core.RunResult{}, errors.New("verification attempt or fixed plan is invalid")
	}
	path, base, err := r.Worktrees.Ensure(ctx, a.TaskID, a.BaseSHA)
	if err != nil || base != a.BaseSHA {
		return core.RunResult{}, errors.New("verification task checkout failed identity check")
	}
	before, err := worktree.Capture(ctx, path, a.BaseSHA)
	if err != nil || !before.Clean || before.HeadSHA != a.HeadSHA {
		return core.RunResult{}, errors.New("verification checkout is dirty or differs from pinned head")
	}
	cfg := r.Sandbox
	cfg.Worktree = path
	record := evidence{SchemaVersion: 1, BaseSHA: a.BaseSHA, HeadSHA: a.HeadSHA, DiffSHA256: before.DiffSHA256, PlanSHA256: r.Plan.SHA256, Status: "passed", Checks: []checkResult{}}
	checks := append([]Check{{Name: "git-diff-check", Cwd: ".", Argv: []string{"/usr/bin/git", "diff", "--check", a.BaseSHA, a.HeadSHA}, TimeoutSeconds: 60}}, r.Plan.Checks...)
	for _, check := range checks {
		result := runCheck(ctx, cfg, check)
		record.Checks = append(record.Checks, result)
		if result.Status != "passed" {
			record.Status = "failed"
			break
		}
	}
	after, err := worktree.Capture(ctx, path, a.BaseSHA)
	if err != nil || !after.Clean || after.HeadSHA != before.HeadSHA || after.DiffSHA256 != before.DiffSHA256 {
		record.Status = "failed"
		record.Checks = append(record.Checks, checkResult{Name: "checkout-unchanged", Status: "failed", ExitCode: -1, Output: "task checkout changed during verification"})
	}
	body, err := json.Marshal(record)
	if err != nil || len(body) > 2<<20 {
		return core.RunResult{}, errors.New("verification evidence could not be encoded")
	}
	result := core.RunResult{Output: body}
	if record.Status != "passed" {
		return result, core.RunError{Kind: "failed", Err: errors.New("fixed verification checks failed")}
	}
	return result, nil
}

func runCheck(ctx context.Context, config isolation.Config, check Check) checkResult {
	start := time.Now()
	checkCtx, cancel := context.WithTimeout(ctx, time.Duration(check.TimeoutSeconds)*time.Second)
	defer cancel()
	cmd, err := config.VerifierCommand(checkCtx, check.Cwd, check.Argv[0], check.Argv[1:]...)
	if err != nil {
		return checkResult{Name: check.Name, Status: "failed", ExitCode: -1, Output: "verifier sandbox rejected check", DurationMS: time.Since(start).Milliseconds()}
	}
	var stdout, stderr boundedBuffer
	stdout.limit, stderr.limit = maxCheckOutput, maxCheckOutput
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err = cmd.Run()
	status, code := "passed", 0
	if err != nil || stdout.exceeded || stderr.exceeded {
		status, code = "failed", -1
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			code = exit.ExitCode()
		}
	}
	output := stdout.String() + stderr.String()
	if stdout.exceeded || stderr.exceeded {
		output += "\n[output exceeded limit]"
	}
	if errors.Is(checkCtx.Err(), context.DeadlineExceeded) {
		status, code = "failed", -1
		output += "\n[check timed out]"
	}
	return checkResult{Name: check.Name, Status: status, ExitCode: code, Output: output, DurationMS: time.Since(start).Milliseconds()}
}

type boundedBuffer struct {
	bytes.Buffer
	limit    int
	exceeded bool
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if len(p) > b.limit-b.Len() {
		b.exceeded = true
		return 0, errors.New("output limit exceeded")
	}
	return b.Buffer.Write(p)
}

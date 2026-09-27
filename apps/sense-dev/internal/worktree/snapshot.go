package worktree

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// MaxSnapshotBytes leaves room for the provider answer in the 2 MiB artifact.
// An oversized change must be split before it can be reviewed as one unit.
const MaxSnapshotBytes = 1 << 20

// Snapshot is evidence from the task checkout after the implementation worker
// has exited. The diff includes committed, staged, and unstaged changes from
// BaseSHA; untracked non-ignored files are appended as Git binary patches.
type Snapshot struct {
	BaseSHA    string `json:"base_sha"`
	HeadSHA    string `json:"head_sha"`
	Clean      bool   `json:"clean"`
	DiffSHA256 string `json:"diff_sha256"`
	Patch      string `json:"patch"`
}

func Capture(ctx context.Context, path, baseSHA string) (Snapshot, error) {
	if !filepath.IsAbs(path) || !shaPattern.MatchString(baseSHA) {
		return Snapshot{}, errors.New("absolute task path and full base SHA required")
	}
	head, err := git(ctx, path, "rev-parse", "HEAD")
	if err != nil || !shaPattern.MatchString(head) {
		return Snapshot{}, errors.New("task HEAD is not a full Git commit SHA")
	}
	if _, err := git(ctx, path, "merge-base", "--is-ancestor", baseSHA, head); err != nil {
		return Snapshot{}, errors.New("task HEAD does not descend from pinned base")
	}
	tracked, err := gitLimited(ctx, path, MaxSnapshotBytes, false, "diff", "--binary", "--no-ext-diff", "--no-textconv", baseSHA, "--")
	if err != nil {
		return Snapshot{}, fmt.Errorf("capture tracked change: %w", err)
	}
	remaining := MaxSnapshotBytes - len(tracked)
	untracked, err := gitLimited(ctx, path, remaining, false, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return Snapshot{}, fmt.Errorf("list untracked files: %w", err)
	}
	var patch bytes.Buffer
	patch.Write(tracked)
	for _, raw := range bytes.Split(untracked, []byte{0}) {
		if len(raw) == 0 {
			continue
		}
		rel := string(raw)
		if filepath.IsAbs(rel) || rel == ".git" || strings.HasPrefix(rel, ".git"+string(filepath.Separator)) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return Snapshot{}, errors.New("untracked path escapes task checkout")
		}
		info, err := os.Lstat(filepath.Join(path, rel))
		if err != nil || !info.Mode().IsRegular() || info.Size() > int64(MaxSnapshotBytes-patch.Len()) {
			return Snapshot{}, errors.New("untracked file is unsafe or exceeds snapshot limit")
		}
		filePatch, err := gitLimited(ctx, path, MaxSnapshotBytes-patch.Len(), true, "diff", "--no-index", "--binary", "--no-ext-diff", "--no-textconv", "--", "/dev/null", rel)
		if err != nil {
			return Snapshot{}, fmt.Errorf("capture untracked change: %w", err)
		}
		patch.Write(filePatch)
	}
	if patch.Len() == 0 {
		return Snapshot{}, errors.New("implementation produced no reviewable change")
	}
	status, err := gitLimited(ctx, path, MaxSnapshotBytes, false, "status", "--porcelain=v1", "--untracked-files=all", "-z")
	if err != nil {
		return Snapshot{}, fmt.Errorf("capture checkout status: %w", err)
	}
	hash := sha256.Sum256(patch.Bytes())
	return Snapshot{BaseSHA: baseSHA, HeadSHA: head, Clean: len(status) == 0, DiffSHA256: hex.EncodeToString(hash[:]), Patch: patch.String()}, nil
}

// gitLimited bounds both normal output and hostile/accidental large diffs.
// --no-index returns 1 for a difference, which is expected for new files.
func gitLimited(ctx context.Context, path string, limit int, allowDifference bool, args ...string) ([]byte, error) {
	if limit <= 0 {
		return nil, errors.New("snapshot size limit reached")
	}
	argv := append([]string{"-c", "core.hooksPath=/dev/null", "-c", "core.fsmonitor=false", "-C", path}, args...)
	cmd := exec.CommandContext(ctx, "git", argv...)
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME"), "GIT_TERMINAL_PROMPT=0", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_OPTIONAL_LOCKS=0"}
	var output boundedBuffer
	output.limit = limit
	cmd.Stdout = &output
	cmd.Stderr = &boundedBuffer{limit: 1024}
	err := cmd.Run()
	if output.exceeded {
		return nil, errors.New("snapshot exceeds size limit")
	}
	if err != nil {
		var exit *exec.ExitError
		if !allowDifference || !errors.As(err, &exit) || exit.ExitCode() != 1 {
			return nil, err
		}
	}
	return output.Bytes(), nil
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

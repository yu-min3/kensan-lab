package worktree

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var taskIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)
var shaPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

// Manager creates and reopens only task-scoped Git working trees. Each task is
// a self-contained local clone: a linked `git worktree` would point outside
// the worker sandbox at the source repository's shared .git directory.
// The source repo and root are operator-owned, never model-supplied paths.
type Manager struct {
	Source string
	Root   string
}

func (m Manager) Ensure(ctx context.Context, taskID, baseSHA string) (string, string, error) {
	if !taskIDPattern.MatchString(taskID) || baseSHA != "" && !shaPattern.MatchString(baseSHA) {
		return "", "", errors.New("invalid task ID or base SHA")
	}
	source, root, err := m.paths()
	if err != nil {
		return "", "", err
	}
	branch := "sense-dev/" + taskID
	path := filepath.Join(root, taskID)
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		if baseSHA == "" {
			baseSHA, err = git(ctx, source, "rev-parse", "HEAD")
			if err != nil || !shaPattern.MatchString(baseSHA) {
				return "", "", errors.New("source HEAD is not a full Git commit SHA")
			}
		}
		if _, err := git(ctx, source, "cat-file", "-e", baseSHA+"^{commit}"); err != nil {
			return "", "", errors.New("task base is not an existing commit")
		}
		if _, err := git(ctx, source, "clone", "--no-local", "--no-checkout", "--no-tags", source, path); err != nil {
			return "", "", errors.New("task checkout could not be cloned")
		}
		if _, err := git(ctx, path, "remote", "remove", "origin"); err != nil {
			return "", "", errors.New("task checkout remote could not be removed")
		}
		if _, err := git(ctx, path, "switch", "-c", branch, baseSHA); err != nil {
			return "", "", errors.New("task branch could not be created")
		}
		if _, err := git(ctx, path, "config", "user.email", "sense-dev@local.invalid"); err != nil {
			return "", "", errors.New("task commit identity could not be set")
		}
		if _, err := git(ctx, path, "config", "user.name", "Sense Dev"); err != nil {
			return "", "", errors.New("task commit identity could not be set")
		}
	} else if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", "", errors.New("task worktree path is not a real directory")
	}
	if err := validate(ctx, source, path, branch, baseSHA); err != nil {
		return "", "", err
	}
	if baseSHA == "" {
		baseSHA, err = git(ctx, path, "rev-parse", "HEAD")
		if err != nil || !shaPattern.MatchString(baseSHA) {
			return "", "", errors.New("task worktree HEAD is not a full Git commit SHA")
		}
		if _, err := git(ctx, source, "cat-file", "-e", baseSHA+"^{commit}"); err != nil {
			return "", "", errors.New("unpinned task checkout HEAD is not from source repository")
		}
	}
	return path, baseSHA, nil
}

// EnsureFromTask gives a read-only consumer its own self-contained checkout of
// a reviewed producer commit, even when that commit is not in the source repo.
func (m Manager) EnsureFromTask(ctx context.Context, taskID, sourceTaskID, headSHA string) (string, string, error) {
	if !taskIDPattern.MatchString(taskID) || !taskIDPattern.MatchString(sourceTaskID) || taskID == sourceTaskID || !shaPattern.MatchString(headSHA) {
		return "", "", errors.New("invalid consumer, producer or reviewed head")
	}
	source, root, err := m.paths()
	if err != nil {
		return "", "", err
	}
	producer := filepath.Join(root, sourceTaskID)
	info, err := os.Lstat(producer)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", "", errors.New("producer checkout is unavailable")
	}
	if err := validate(ctx, source, producer, "sense-dev/"+sourceTaskID, ""); err != nil {
		return "", "", err
	}
	actualHead, err := git(ctx, producer, "rev-parse", "HEAD")
	if err != nil || actualHead != headSHA {
		return "", "", errors.New("producer checkout is not at reviewed head")
	}
	if dirty, err := git(ctx, producer, "status", "--porcelain"); err != nil || dirty != "" {
		return "", "", errors.New("producer checkout is dirty")
	}
	path := filepath.Join(root, taskID)
	branch := "sense-dev/" + taskID
	info, err = os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		if _, err := git(ctx, producer, "clone", "--no-local", "--no-checkout", "--no-tags", producer, path); err != nil {
			return "", "", errors.New("consumer checkout could not be cloned")
		}
		if _, err := git(ctx, path, "remote", "remove", "origin"); err != nil {
			return "", "", errors.New("consumer checkout remote could not be removed")
		}
		if _, err := git(ctx, path, "switch", "-c", branch, headSHA); err != nil {
			return "", "", errors.New("consumer branch could not be created")
		}
	} else if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", "", errors.New("consumer checkout path is not a real directory")
	}
	if err := validate(ctx, producer, path, branch, headSHA); err != nil {
		return "", "", err
	}
	actualHead, err = git(ctx, path, "rev-parse", "HEAD")
	if err != nil || actualHead != headSHA {
		return "", "", errors.New("consumer checkout moved from reviewed head")
	}
	if dirty, err := git(ctx, path, "status", "--porcelain"); err != nil || dirty != "" {
		return "", "", errors.New("consumer checkout is dirty")
	}
	return path, headSHA, nil
}

func (m Manager) paths() (string, string, error) {
	if !filepath.IsAbs(m.Source) || !filepath.IsAbs(m.Root) || m.Source == "/" || m.Root == "/" {
		return "", "", errors.New("source and worktree root must be absolute non-root paths")
	}
	source, err := filepath.EvalSymlinks(m.Source)
	if err != nil {
		return "", "", err
	}
	root, err := filepath.EvalSymlinks(m.Root)
	if err != nil {
		return "", "", err
	}
	for _, path := range []string{source, root} {
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			return "", "", errors.New("source or worktree root is not a directory")
		}
	}
	if nested(source, root) || nested(root, source) {
		return "", "", errors.New("source repo and worktree root must be disjoint")
	}
	return source, root, nil
}

func validate(ctx context.Context, source, path, branch, baseSHA string) error {
	metadata, err := os.Lstat(filepath.Join(path, ".git"))
	if err != nil || !metadata.IsDir() || metadata.Mode()&os.ModeSymlink != 0 {
		return errors.New("task checkout must have self-contained Git metadata")
	}
	if _, err := os.Lstat(filepath.Join(path, ".git", "objects", "info", "alternates")); !errors.Is(err, os.ErrNotExist) {
		return errors.New("task checkout must not share Git objects")
	}
	remotes, err := git(ctx, path, "remote")
	if err != nil || remotes != "" {
		return errors.New("task checkout must not have a Git remote")
	}
	top, err := git(ctx, path, "rev-parse", "--show-toplevel")
	if err != nil || top != path {
		return errors.New("task path is not its own Git worktree")
	}
	actualBranch, err := git(ctx, path, "symbolic-ref", "--short", "HEAD")
	if err != nil || actualBranch != branch {
		return errors.New("task worktree branch mismatch")
	}
	if baseSHA != "" {
		if _, err := git(ctx, source, "cat-file", "-e", baseSHA+"^{commit}"); err != nil {
			return errors.New("task base no longer exists in source repository")
		}
		if _, err := git(ctx, path, "merge-base", "--is-ancestor", baseSHA, "HEAD"); err != nil {
			return errors.New("task base is not an ancestor of worktree HEAD")
		}
	}
	return nil
}

func nested(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func git(ctx context.Context, repo string, args ...string) (string, error) {
	argv := append([]string{"-c", "core.hooksPath=/dev/null", "-c", "core.fsmonitor=false", "-C", repo}, args...)
	cmd := exec.CommandContext(ctx, "git", argv...)
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME"), "GIT_TERMINAL_PROMPT=0", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null"}
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s failed: %w", args[0], err)
	}
	return strings.TrimSpace(string(output)), nil
}

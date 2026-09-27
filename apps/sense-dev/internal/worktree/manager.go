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

// Manager creates and reopens only task-scoped Git worktrees. The source repo
// and root are operator-owned paths, never paths supplied by a model.
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
		if _, err := git(ctx, source, "show-ref", "--verify", "refs/heads/"+branch); err == nil {
			return "", "", errors.New("task branch already exists without its worktree; inspect before recovery")
		}
		if _, err := git(ctx, source, "worktree", "add", "-b", branch, path, baseSHA); err != nil {
			return "", "", errors.New("task worktree could not be created")
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
	}
	return path, baseSHA, nil
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
	top, err := git(ctx, path, "rev-parse", "--show-toplevel")
	if err != nil || top != path {
		return errors.New("task path is not its own Git worktree")
	}
	actualBranch, err := git(ctx, path, "symbolic-ref", "--short", "HEAD")
	if err != nil || actualBranch != branch {
		return errors.New("task worktree branch mismatch")
	}
	sourceCommon, err := commonDir(ctx, source)
	if err != nil {
		return err
	}
	taskCommon, err := commonDir(ctx, path)
	if err != nil || sourceCommon != taskCommon {
		return errors.New("task worktree belongs to another repository")
	}
	if baseSHA != "" {
		if _, err := git(ctx, path, "merge-base", "--is-ancestor", baseSHA, "HEAD"); err != nil {
			return errors.New("task base is not an ancestor of worktree HEAD")
		}
	}
	return nil
}

func commonDir(ctx context.Context, repo string) (string, error) {
	value, err := git(ctx, repo, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(value) {
		value = filepath.Join(repo, value)
	}
	return filepath.EvalSymlinks(value)
}

func nested(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func git(ctx context.Context, repo string, args ...string) (string, error) {
	argv := append([]string{"-C", repo}, args...)
	cmd := exec.CommandContext(ctx, "git", argv...)
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME"), "GIT_TERMINAL_PROMPT=0", "GIT_CONFIG_NOSYSTEM=1"}
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s failed: %w", args[0], err)
	}
	return strings.TrimSpace(string(output)), nil
}

package worktree

import (
	"context"
	"errors"
	"os"
	"path/filepath"
)

// PrepareSandboxMetadata creates empty mount targets before a worker starts.
// Existing project configuration stays intact and will be mounted read-only.
func PrepareSandboxMetadata(path string) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || path == "/" {
		return errors.New("absolute task checkout required")
	}
	if info, err := os.Lstat(path); err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("task checkout must be a real directory")
	}
	for _, name := range []string{".agents", ".codex", ".aws"} {
		target := filepath.Join(path, name)
		info, err := os.Lstat(target)
		if errors.Is(err, os.ErrNotExist) {
			if err := os.Mkdir(target, 0755); err != nil {
				return err
			}
			continue
		}
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("sandbox metadata target is not a real directory")
		}
	}
	return nil
}

// CommitImplementation runs after the isolated worker exits. Git metadata is
// unavailable for model writes; hooks and ambient Git configuration are disabled.
func CommitImplementation(ctx context.Context, path, baseSHA string) (Snapshot, error) {
	before, err := Capture(ctx, path, baseSHA)
	if err != nil {
		return Snapshot{}, err
	}
	if before.Clean {
		return before, nil // Reopening a committed candidate is idempotent.
	}
	if _, err := git(ctx, path, "add", "--all", "--", "."); err != nil {
		return Snapshot{}, err
	}
	staged, err := Capture(ctx, path, baseSHA)
	if err != nil || staged.HeadSHA != before.HeadSHA {
		return Snapshot{}, errors.New("staged implementation changed checkout identity")
	}
	if _, err := git(ctx, path, "-c", "commit.gpgsign=false", "commit", "-m", "自動開発の実装候補を固定"); err != nil {
		return Snapshot{}, err
	}
	after, err := Capture(ctx, path, baseSHA)
	if err != nil || !after.Clean || after.DiffSHA256 != staged.DiffSHA256 {
		return Snapshot{}, errors.New("supervisor commit changed implementation evidence")
	}
	return after, nil
}

package worktree

import (
	"context"
	"errors"
	"os"
	"path/filepath"
)

// EnsureImageDeployment seeds A but retains the original main base for B's
// eventual merge. Subsequent stages may advance B without changing that base.
func (m Manager) EnsureImageDeployment(ctx context.Context, taskID, producerID, baseSHA, seedSHA string, requireClean bool) (string, string, error) {
	if !taskIDPattern.MatchString(taskID) || !taskIDPattern.MatchString(producerID) || taskID == producerID || !shaPattern.MatchString(baseSHA) || !shaPattern.MatchString(seedSHA) {
		return "", "", errors.New("invalid image deployment identity")
	}
	source, root, err := m.paths()
	if err != nil {
		return "", "", err
	}
	producer := filepath.Join(root, producerID)
	if _, err := validateProducer(ctx, source, producer, producerID, seedSHA); err != nil {
		return "", "", err
	}
	if _, err := git(ctx, producer, "merge-base", "--is-ancestor", baseSHA, seedSHA); err != nil {
		return "", "", errors.New("image source does not descend from pinned original base")
	}
	path := filepath.Join(root, taskID)
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		if _, _, err := m.EnsureFromTask(ctx, taskID, producerID, seedSHA); err != nil {
			return "", "", err
		}
		if _, err := git(ctx, path, "config", "user.name", "Sense Dev"); err != nil {
			return "", "", err
		}
		if _, err := git(ctx, path, "config", "user.email", "sense-dev@local.invalid"); err != nil {
			return "", "", err
		}
	} else if err != nil {
		return "", "", err
	}
	if err := validate(ctx, source, path, "sense-dev/"+taskID, baseSHA); err != nil {
		return "", "", err
	}
	if _, err := git(ctx, path, "merge-base", "--is-ancestor", seedSHA, "HEAD"); err != nil {
		return "", "", errors.New("deployment checkout diverged from image source")
	}
	if requireClean {
		if dirty, err := git(ctx, path, "status", "--porcelain"); err != nil || dirty != "" {
			return "", "", errors.New("deployment checkout is dirty")
		}
	}
	return path, baseSHA, nil
}

package main

import (
	"context"
	"errors"
	"strings"

	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/core"
	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/worktree"
)

func rejectSimulationHistory(state core.State) error {
	for _, attempt := range state.Attempts {
		if strings.HasPrefix(attempt.SessionID, "mock-") {
			return errors.New("simulation state cannot be resumed with a live model worker")
		}
	}
	for _, agent := range state.Agents {
		if strings.HasPrefix(agent.SessionID, "mock-") {
			return errors.New("simulation agent session cannot be resumed with a live model worker")
		}
	}
	return nil
}

// prepareReadyWorktrees pins a task's base before Tick can construct its
// immutable input manifest. A failed task stays unpinned and is skipped by
// isolated dispatch without consuming an attempt or blocking other tasks.
func prepareReadyWorktrees(ctx context.Context, store *core.Store, manager worktree.Manager) error {
	var failures []error
	state := store.Snapshot()
	for _, task := range state.Tasks {
		if task.BaseSHA != "" || task.Status == "done" || task.Status == "failed" || task.Status == "publish_wait" {
			continue
		}
		var base string
		var err error
		if task.Team == core.App && task.Kind == "acceptance" && task.SourceTaskID != "" {
			if task.HeadSHA == "" {
				continue // The reviewed producer revision has not been delivered yet.
			}
			source := state.Tasks[task.SourceTaskID]
			if source.HeadSHA != task.HeadSHA || source.Status != "publish_wait" {
				continue // A stale handoff must not create a checkout.
			}
			_, base, err = manager.EnsureFromTask(ctx, task.ID, source.ID, task.HeadSHA)
		} else {
			_, base, err = manager.Ensure(ctx, task.ID, "")
		}
		if err == nil {
			err = store.SetBaseSHA(task.ID, base)
		}
		if err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

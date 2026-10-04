package main

import (
	"context"
	"errors"
	"path/filepath"
	"sort"

	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/committransfer"
	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/core"
	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/publisherbridge"
)

// preparedPublisher fixes the author's path from the controller ledger. No
// worker path or publisher credential enters this input-transfer boundary.
type preparedPublisher struct {
	publisherbridge.Client
	Store        *core.Store
	WorktreeRoot string
}

func (p preparedPublisher) Prepare(ctx context.Context, intent core.PublishIntent) error {
	if intent.Operation != "branch_push" {
		return nil
	}
	if p.Store == nil || !filepath.IsAbs(p.WorktreeRoot) {
		return errors.New("fixed commit-transfer source required")
	}
	st := p.Store.Snapshot()
	decision, ok := st.Decisions[intent.DecisionID]
	author := st.Agents[decision.AuthorAgentID]
	task := st.Tasks[author.TaskID]
	if !ok || decision.Operation != intent.Operation || decision.HeadSHA != intent.HeadSHA || task.HeadSHA != intent.HeadSHA || task.BaseSHA != intent.BaseSHA || task.ID == "" || decision.Ref != "refs/heads/sense-dev/"+task.ID {
		return errors.New("commit-transfer author binding changed")
	}
	bundle, err := committransfer.Create(ctx, filepath.Join(p.WorktreeRoot, task.ID), task.ID, intent.BaseSHA, intent.HeadSHA)
	if err != nil {
		return err
	}
	return p.Client.ImportCommit(ctx, intent, bundle)
}

// importMergedCommits makes the verified graph reachable in the credentialless
// controller source before Platform improvements ask for that exact merge base.
type mergedCommitClient interface {
	ExportMerged(context.Context, core.PublishIntent, string) (committransfer.Bundle, error)
}

func importMergedCommits(ctx context.Context, store *core.Store, client mergedCommitClient, sourceRepo, mission string) error {
	st := store.Snapshot()
	var ids []string
	for id, intent := range st.Intents {
		if intent.Status == "sent" && (intent.Operation == "merge" || intent.Operation == "deploy") {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	var failures []error
	for _, id := range ids {
		intent := st.Intents[id]
		decision := st.Decisions[intent.DecisionID]
		author := st.Agents[decision.AuthorAgentID]
		task := st.Tasks[author.TaskID]
		if task.ID == "" || task.MissionID != mission || intent.ExternalID == "" || intent.HeadSHA != decision.HeadSHA || task.HeadSHA != intent.HeadSHA {
			continue
		}
		if committransfer.Contains(ctx, sourceRepo, intent.ExternalID) {
			continue
		}
		bundle, err := client.ExportMerged(ctx, intent, task.ID)
		if err == nil {
			err = committransfer.Import(ctx, sourceRepo, bundle)
		}
		if err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

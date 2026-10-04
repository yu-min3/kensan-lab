package main

import (
	"context"
	"errors"
	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/core"
)

type hostObservationClient interface {
	core.PublishTransport
	imageObservationClient
	deploymentObservationClient
	mergedCommitClient
}

// This entry point admits remote reads only. It is intentionally called before
// inference-window/billing admission, and cannot prepare or execute new intents.
func reconcileHostObservations(ctx context.Context, s *core.Store, d *core.ReleaseDriver, client hostObservationClient, sourceRepo string) error {
	return errors.Join(
		d.ReconcileUnknown(ctx, client),
		observeImages(ctx, s, client, d.Plan.MissionID),
		importMergedCommits(ctx, s, client, sourceRepo, d.Plan.MissionID),
		observeDeployments(ctx, s, client, d.Plan.MissionID),
	)
}

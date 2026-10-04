package main

import (
	"context"
	"encoding/json"
	"errors"
	"sort"

	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/core"
	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/publisher"
)

type imageObservationClient interface {
	ImageEvidence(context.Context, core.PublishIntent) (publisher.ImageEvidence, error)
}

func observeImages(ctx context.Context, s *core.Store, client imageObservationClient, mission string) error {
	if client == nil {
		return errors.New("authenticated image observer required")
	}
	st := s.Snapshot()
	var ids []string
	for id, i := range st.Intents {
		task := st.Tasks[st.Agents[st.Decisions[i.DecisionID].AuthorAgentID].TaskID]
		if i.Status == "sent" && i.Operation == "image_publish" && task.MissionID == mission && task.Team == core.App && task.Status == "publish_wait" && task.HeadSHA == i.HeadSHA && st.ImageReleases[task.ID].Image.SourceSHA != task.HeadSHA {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	var failures []error
	for _, id := range ids {
		i := st.Intents[id]
		taskID := st.Agents[st.Decisions[i.DecisionID].AuthorAgentID].TaskID
		e, err := client.ImageEvidence(ctx, i)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		if e.Repository != i.Repository || e.Image != core.CanaryImageRepository || e.Visibility != "private" || i.ImageRelease == nil {
			failures = append(failures, errors.New("host image identity mismatch"))
			continue
		}
		record := core.ImageReleaseRecord{SourceTaskID: taskID, DecisionID: i.DecisionID, IntentID: i.ID, Image: core.ImageDeploymentSpec{SourceSHA: e.SourceSHA, SourceAppTreeSHA: e.SourceAppTreeSHA, ImageTag: e.ImageTag, Digest: e.Digest, WorkflowRef: e.WorkflowRef, WorkflowSHA: e.WorkflowSHA, WorkflowSHA256: e.WorkflowSHA256, DispatchID: e.DispatchID, WorkflowRunID: e.WorkflowRunID}}
		body, err := json.Marshal(record)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		a, err := s.PutArtifact("system", "image_release_observation", body)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		record.EvidenceRef = core.ArtifactRef{ID: a.ID, Version: a.Version, SHA256: a.SHA256}
		if err := s.RecordImageRelease(record); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

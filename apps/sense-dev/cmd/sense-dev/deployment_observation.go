package main

import (
	"context"
	"encoding/json"
	"errors"
	"sort"

	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/core"
)

type deploymentObservationClient interface {
	Observe(context.Context, core.PublishIntent, string) (core.DeploymentReceipt, error)
}

// The controller is the sole writer. Only the authenticated host bridge can
// provide observations; model output and web requests cannot enter this path.
func observeDeployments(ctx context.Context, s *core.Store, client deploymentObservationClient, mission string) error {
	if client == nil {
		return errors.New("host observer required")
	}
	st := s.Snapshot()
	ids := make([]string, 0)
	for id, i := range st.Intents {
		d := st.Decisions[i.DecisionID]
		task := st.Tasks[st.Agents[d.AuthorAgentID].TaskID]
		if i.Status == "sent" && (i.Operation == "merge" || i.Operation == "deploy") && d.TargetEnvironment == "private-canary" && task.Kind == "change" && task.Status == "publish_wait" && task.MissionID == mission && task.HeadSHA == i.HeadSHA && st.Deployments[task.ID].HeadSHA != task.HeadSHA {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		intent := st.Intents[id]
		decision := st.Decisions[intent.DecisionID]
		taskID := st.Agents[decision.AuthorAgentID].TaskID
		receipt, err := client.Observe(ctx, intent, taskID)
		if err != nil {
			return err
		}
		if receipt.TaskID != taskID || receipt.IntentID != id || receipt.DecisionID != intent.DecisionID || receipt.HeadSHA != intent.HeadSHA || receipt.Revision != intent.ExternalID {
			return errors.New("host observation identity changed")
		}
		body, err := json.Marshal(receipt)
		if err != nil {
			return err
		}
		artifact, err := s.PutArtifact("system", "deployment_observation", body)
		if err != nil {
			return err
		}
		receipt.EvidenceRef = core.ArtifactRef{ID: artifact.ID, Version: artifact.Version, SHA256: artifact.SHA256}
		if err := s.RecordDeploymentReceipt(receipt); err != nil {
			return err
		}
	}
	return nil
}

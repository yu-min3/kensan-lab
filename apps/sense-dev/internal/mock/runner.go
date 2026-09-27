package mock

import (
	"context"
	"encoding/json"

	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/core"
)

// Runner exercises persistence and dispatch without contacting a model or
// editing a repository. Its output is never release evidence.
type Runner struct{}

func (Runner) Run(_ context.Context, d core.Dispatch) (core.RunResult, error) {
	if err := d.BindSession("mock-" + d.Attempt.ID); err != nil {
		return core.RunResult{}, err
	}
	b, err := json.Marshal(struct {
		SimulationOnly bool   `json:"simulation_only"`
		AttemptID      string `json:"attempt_id"`
		AgentID        string `json:"agent_id"`
		ManifestHash   string `json:"input_manifest_hash"`
		Result         string `json:"result"`
	}{true, d.Attempt.ID, d.Attempt.AgentID, d.Manifest.InputSHA256, "dispatch and persistence exercised; no model or code change"})
	return core.RunResult{Output: b}, err
}

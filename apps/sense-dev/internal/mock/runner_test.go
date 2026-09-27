package mock

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/core"
)

func TestPlannedTaskRunsToPublishWaitWithoutModel(t *testing.T) {
	store, err := core.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.SeedKnowledge(); err != nil {
		t.Fatal(err)
	}
	task, agents, err := store.CreatePlannedTask("mission", core.Platform, "change", "safe canary", "v1")
	if err != nil {
		t.Fatal(err)
	}
	for range agents {
		worked, err := store.Tick(context.Background(), Runner{}, []string{"simulation-only"})
		if err != nil || !worked {
			t.Fatalf("mock tick: worked=%t err=%v", worked, err)
		}
	}
	st := store.Snapshot()
	if st.Tasks[task.ID].Status != "publish_wait" {
		t.Fatalf("task status %s", st.Tasks[task.ID].Status)
	}
	if len(st.Attempts) != 4 {
		t.Fatalf("attempts=%d", len(st.Attempts))
	}
	manifest, err := store.BuildManifest(agents[3].ID, nil)
	if err != nil || len(manifest.StageInputs) != 3 {
		t.Fatalf("review did not receive prior stage artifacts: %v %+v", err, manifest.StageInputs)
	}
	for i, input := range manifest.StageInputs {
		if input.AgentID != agents[i].ID || input.Artifact.SHA256 == "" {
			t.Fatalf("wrong stage handoff %d: %+v", i, input)
		}
	}
	for _, a := range st.Attempts {
		if a.Status != "completed" || a.OutputRef == nil {
			t.Fatalf("bad attempt %+v", a)
		}
		b, err := store.ReadArtifact(a.OutputRef.ID)
		if err != nil {
			t.Fatal(err)
		}
		var result struct {
			SimulationOnly bool `json:"simulation_only"`
		}
		if err := json.Unmarshal(b, &result); err != nil || !result.SimulationOnly {
			t.Fatal("mock result masquerades as real")
		}
	}
}

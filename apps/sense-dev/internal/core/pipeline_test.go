package core

import "testing"

func TestPlannedPipelineHasIndependentStages(t *testing.T) {
	s := testStore(t)
	platform, stages, err := s.CreatePlannedTask("mission", Platform, "change", "golden path canary", "contract-v1")
	if err != nil {
		t.Fatal(err)
	}
	if len(stages) != 4 {
		t.Fatalf("stages=%d", len(stages))
	}
	wantRoles := []string{"requirements", "design_review", "implementation", "implementation_review"}
	wantModels := []string{"fable", "gpt-6-astra", "gpt-6-sol", "opus"}
	for i, a := range stages {
		if a.Role != wantRoles[i] || a.Model != wantModels[i] || a.TaskID != platform.ID || a.SessionID != "" {
			t.Fatalf("wrong stage %d: %+v", i, a)
		}
		if i > 0 && (len(a.DependsOn) != 1 || a.DependsOn[0] != stages[i-1].ID) {
			t.Fatalf("stage %d is not dependency-bound", i)
		}
	}
	app, appStages, err := s.CreatePlannedTask("mission", App, "acceptance", "consumer scenario", "contract-v1")
	if err != nil {
		t.Fatal(err)
	}
	if app.ID == platform.ID || len(appStages) != 1 || appStages[0].Team != App || appStages[0].ID == stages[0].ID {
		t.Fatal("App task is not independent")
	}
	if _, _, err := s.CreatePlannedTask("mission", Platform, "acceptance", "invalid", "contract-v1"); err == nil {
		t.Fatal("Platform acceptance task accepted")
	}
	if len(s.Snapshot().Tasks) != 2 {
		t.Fatal("rejected task partially persisted")
	}
}

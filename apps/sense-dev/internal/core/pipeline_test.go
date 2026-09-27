package core

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestPlannedPipelineHasIndependentStages(t *testing.T) {
	s := testStore(t)
	platform, stages, err := s.CreatePlannedTask("mission", Platform, "change", "golden path canary", "contract-v1")
	if err != nil {
		t.Fatal(err)
	}
	if len(stages) != 5 {
		t.Fatalf("stages=%d", len(stages))
	}
	wantRoles := []string{"requirements", "design_review", "implementation", "verification", "implementation_review"}
	wantModels := []string{"fable", "gpt-6-astra", "gpt-6-sol", "local", "opus"}
	for i, a := range stages {
		if a.Role != wantRoles[i] || a.Model != wantModels[i] || a.TaskID != platform.ID || a.SessionID != "" {
			t.Fatalf("wrong stage %d: %+v", i, a)
		}
		if i > 0 && (len(a.DependsOn) != i || a.DependsOn[i-1] != stages[i-1].ID) {
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

func TestLinkedAcceptanceWaitsForReceivedCurrentPlatformHead(t *testing.T) {
	s := testStore(t)
	platform, _, err := s.CreatePlannedTask("mission", Platform, "change", "golden path canary", "contract-v1")
	if err != nil {
		t.Fatal(err)
	}
	app, appAgent, err := s.CreateLinkedAcceptanceTask(platform.ID, "consumer acceptance")
	if err != nil {
		t.Fatal(err)
	}
	if app.SourceTaskID != platform.ID || app.MissionID != platform.MissionID || app.ContractVersion != platform.ContractVersion || appAgent.Team != App {
		t.Fatalf("incorrect linkage: %+v %+v", app, appAgent)
	}
	if _, _, err := s.CreateLinkedAcceptanceTask(app.ID, "invalid source"); err == nil {
		t.Fatal("App source accepted")
	}
	base, head := strings.Repeat("a", 40), strings.Repeat("b", 40)
	if err := s.SetBaseSHA(platform.ID, base); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if worked, err := s.Tick(context.Background(), releaseReadyRunner{base: base, head: head}, []string{"isolated-model-worker"}); err != nil || !worked {
			t.Fatalf("Platform stage %d: worked=%t err=%v", i, worked, err)
		}
	}
	if got := s.Snapshot().Tasks[app.ID].HeadSHA; got != "" {
		t.Fatalf("App ran before handoff: %s", got)
	}
	if count, err := s.ReconcileReviewedHandoffs(); err != nil || count != 1 {
		t.Fatalf("handoff count=%d err=%v", count, err)
	}
	if count, err := s.ReconcileReviewedHandoffs(); err != nil || count != 0 {
		t.Fatalf("duplicate handoff count=%d err=%v", count, err)
	}
	root := s.root
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if count, err := s.ReconcileReviewedHandoffs(); err != nil || count != 0 {
		t.Fatalf("restart duplicated handoff: %d %v", count, err)
	}
	if got := s.Snapshot().Tasks[app.ID].HeadSHA; got != head {
		t.Fatalf("App head not pinned: %s", got)
	}
	if len(s.Snapshot().Messages) != 1 {
		t.Fatal("handoff was duplicated")
	}
	if err := s.SetHeadSHA(platform.ID, strings.Repeat("c", 40)); err != nil {
		t.Fatal(err)
	}
	if attempt, ok, err := s.ClaimNext(time.Now()); err != nil || ok {
		t.Fatalf("stale handoff claimed %+v: %v", attempt, err)
	}
	if err := s.SetHeadSHA(platform.ID, head); err != nil {
		t.Fatal(err)
	}
	manifest, err := s.BuildManifest(appAgent.ID, nil)
	if err != nil || len(manifest.Inbox) != 3 || len(manifest.MessageIDs) != 1 {
		t.Fatalf("App inbox mismatch: %+v %v", manifest, err)
	}
	attempt, ok, err := s.ClaimNext(time.Now())
	if err != nil || !ok || attempt.AgentID != appAgent.ID || attempt.HeadSHA != head {
		t.Fatalf("App acceptance not claimed: %+v %v", attempt, err)
	}
}

func TestRejectedReviewCannotTriggerAppHandoff(t *testing.T) {
	s := testStore(t)
	platform, stages, err := s.CreatePlannedTask("mission", Platform, "change", "canary", "v1")
	if err != nil {
		t.Fatal(err)
	}
	app, appAgent, err := s.CreateLinkedAcceptanceTask(platform.ID, "acceptance")
	if err != nil {
		t.Fatal(err)
	}
	base, head := strings.Repeat("a", 40), strings.Repeat("b", 40)
	if err := s.SetBaseSHA(platform.ID, base); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if worked, err := s.Tick(context.Background(), releaseReadyRunner{base: base, head: head, reviewVerdict: "fail"}, []string{"isolated-model-worker"}); err != nil || !worked {
			t.Fatalf("stage %d: %t %v", i, worked, err)
		}
	}
	if count, err := s.ReconcileReviewedHandoffs(); err != nil || count != 0 {
		t.Fatalf("rejected review delivered: %d %v", count, err)
	}
	var ref *ArtifactRef
	for _, attempt := range s.Snapshot().Attempts {
		if attempt.AgentID == stages[4].ID && attempt.Status == "completed" {
			ref = attempt.OutputRef
		}
	}
	if ref == nil {
		t.Fatal("missing rejected review artifact")
	}
	message := Message{ID: "forged-pass", CorrelationID: "mission", FromAgent: stages[4].ID, ToAgent: appAgent.ID, SourceTask: platform.ID, TargetTask: app.ID, Kind: "change_ready", ArtifactRefs: []ArtifactRef{*ref}, ContractVersion: platform.ContractVersion, HeadSHA: head}
	if _, err := s.SendMessage(message); err == nil {
		t.Fatal("manual handoff bypassed rejected review")
	}
	if s.Snapshot().Tasks[app.ID].HeadSHA != "" || len(s.Snapshot().Messages) != 0 {
		t.Fatal("rejected change reached App")
	}
}

type pipelineRunner struct {
	head             string
	failVerification bool
}

func (r pipelineRunner) Run(_ context.Context, d Dispatch) (RunResult, error) {
	if d.Attempt.Role == "verification" {
		if r.failVerification {
			return RunResult{Output: []byte(`{"status":"failed","check":"test"}`)}, RunError{Kind: "failed", Err: errors.New("test failed")}
		}
		return RunResult{Output: []byte(`{"status":"passed","check":"test"}`)}, nil
	}
	if err := d.BindSession("session-" + d.Attempt.ID); err != nil {
		return RunResult{}, err
	}
	if d.Attempt.Role == "implementation" {
		return RunResult{Output: []byte("implementation patch"), HeadSHA: r.head}, nil
	}
	return RunResult{Output: []byte("stage output")}, nil
}

func TestPlannedVerificationBlocksReviewAndKeepsFailureEvidence(t *testing.T) {
	s := testStore(t)
	task, stages, err := s.CreatePlannedTask("mission", Platform, "change", "canary", "v1")
	if err != nil {
		t.Fatal(err)
	}
	base, head := strings.Repeat("a", 40), strings.Repeat("b", 40)
	if err := s.SetBaseSHA(task.ID, base); err != nil {
		t.Fatal(err)
	}
	runner := pipelineRunner{head: head, failVerification: true}
	for i := 0; i < 4; i++ {
		worked, err := s.Tick(context.Background(), runner, nil)
		if err != nil || !worked {
			t.Fatalf("stage %d: %t %v", i, worked, err)
		}
	}
	state := s.Snapshot()
	if state.Tasks[task.ID].HeadSHA != head || state.Agents[stages[3].ID].Status != "failed" || state.Agents[stages[4].ID].Status != "ready" {
		t.Fatal("verification failure did not hold implementation review")
	}
	found := false
	for _, attempt := range state.Attempts {
		if attempt.AgentID == stages[3].ID {
			found = attempt.Status == "failed" && attempt.OutputRef != nil
		}
	}
	if !found {
		t.Fatal("failed verifier evidence was not linked to attempt")
	}
	if worked, err := s.Tick(context.Background(), runner, nil); err != nil || worked {
		t.Fatalf("review ran after failed verification: %t %v", worked, err)
	}
}

func TestPlannedReviewGetsRequirementsPatchAndVerification(t *testing.T) {
	s := testStore(t)
	task, stages, err := s.CreatePlannedTask("mission", Platform, "change", "canary", "v1")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetBaseSHA(task.ID, strings.Repeat("a", 40)); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		worked, err := s.Tick(context.Background(), pipelineRunner{head: strings.Repeat("b", 40)}, nil)
		if err != nil || !worked {
			t.Fatalf("stage %d: %t %v", i, worked, err)
		}
	}
	manifest, err := s.BuildManifest(stages[4].ID, nil)
	if err != nil || manifest.HeadSHA != strings.Repeat("b", 40) || len(manifest.StageInputs) != 4 {
		t.Fatalf("review lost prior evidence: %+v %v", manifest, err)
	}
	for i, input := range manifest.StageInputs {
		if input.Role != stages[i].Role {
			t.Fatalf("stage %d mismatched: %+v", i, input)
		}
	}
}

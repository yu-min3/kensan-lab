package core

import (
	"context"
	"encoding/json"
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

type acceptanceRunner struct{ base, head, verdict, observed string }

type invalidAcceptanceRunner struct{ releaseReadyRunner }

func (r invalidAcceptanceRunner) Run(ctx context.Context, d Dispatch) (RunResult, error) {
	if d.Attempt.Role != "app_acceptance" {
		return r.releaseReadyRunner.Run(ctx, d)
	}
	if err := d.BindSession("thread-" + d.Attempt.ID); err != nil {
		return RunResult{}, err
	}
	return RunResult{Output: []byte("accepted without pinned evidence")}, nil
}

func (r acceptanceRunner) Run(ctx context.Context, d Dispatch) (RunResult, error) {
	if d.Attempt.Role != "app_acceptance" {
		return releaseReadyRunner{base: r.base, head: r.head}.Run(ctx, d)
	}
	if err := d.BindSession("thread-" + d.Attempt.ID); err != nil {
		return RunResult{}, err
	}
	body, err := json.Marshal(AcceptanceResult{SchemaVersion: 1, Verdict: r.verdict, ScenarioID: d.Attempt.TaskID, ContractVersion: d.Attempt.ContractVersion, HeadSHA: d.Attempt.HeadSHA, Expected: "consumer scenario", Observed: r.observed})
	return RunResult{Output: body}, err
}

func TestAppFailureIsDeliveredAsPinnedPlatformFeedback(t *testing.T) {
	s := testStore(t)
	platform, stages, err := s.CreatePlannedTask("mission", Platform, "change", "canary", "v1")
	if err != nil {
		t.Fatal(err)
	}
	app, appAgent, err := s.CreateLinkedAcceptanceTask(platform.ID, "consumer scenario")
	if err != nil {
		t.Fatal(err)
	}
	base, head := strings.Repeat("a", 40), strings.Repeat("b", 40)
	if err := s.SetBaseSHA(platform.ID, base); err != nil {
		t.Fatal(err)
	}
	runner := acceptanceRunner{base: base, head: head, verdict: "fail", observed: "SSO callback returned 403"}
	for i := 0; i < 5; i++ {
		if worked, err := s.Tick(context.Background(), runner, []string{"isolated-model-worker"}); err != nil || !worked {
			t.Fatalf("Platform stage %d: %t %v", i, worked, err)
		}
	}
	if count, err := s.ReconcileReviewedHandoffs(); err != nil || count != 1 {
		t.Fatalf("handoff: %d %v", count, err)
	}
	if err := s.SetBaseSHA(app.ID, head); err != nil {
		t.Fatal(err)
	} // Checkout provisioning is exercised separately.
	if worked, err := s.Tick(context.Background(), runner, []string{"isolated-model-worker"}); err != nil || !worked {
		t.Fatalf("App acceptance: %t %v", worked, err)
	}
	if count, err := s.ReconcileAcceptanceOutcomes(); err != nil || count != 1 {
		t.Fatalf("feedback: %d %v", count, err)
	}
	if count, err := s.ReconcileAcceptanceOutcomes(); err != nil || count != 0 {
		t.Fatalf("duplicate feedback: %d %v", count, err)
	}
	state := s.Snapshot()
	if state.Tasks[app.ID].Status != "revision_wait" || state.Agents[appAgent.ID].Status != "completed" {
		t.Fatal("failed App scenario was accepted or lost")
	}
	if state.Tasks[platform.ID].Status != "ready" || state.Tasks[platform.ID].CorrectionCount != 1 || state.Agents[stages[2].ID].Status != "ready" || state.Agents[stages[2].ID].SessionGeneration != 2 || state.Agents[stages[3].ID].Status != "ready" || state.Agents[stages[4].ID].Status != "ready" {
		t.Fatal("Platform correction did not reopen the required stages")
	}
	var feedback Message
	for _, message := range state.Messages {
		if message.Kind == "acceptance_failed" {
			feedback = message
		}
	}
	if feedback.ID == "" || feedback.SourceTask != app.ID || feedback.TargetTask != platform.ID || feedback.ToAgent != stages[2].ID || feedback.HeadSHA != head || feedback.ScenarioID != app.ID || feedback.Expected != app.Title || feedback.Observed != runner.observed {
		t.Fatalf("incorrect feedback: %+v", feedback)
	}
	manifest, err := s.BuildManifest(stages[2].ID, []string{"isolated-model-worker"})
	if err != nil || len(manifest.Inbox) != 1 || manifest.Inbox[0] != feedback.ArtifactRefs[0] {
		t.Fatalf("Platform correction input missing: %+v %v", manifest, err)
	}
	corrected := strings.Repeat("c", 40)
	second := acceptanceRunner{base: base, head: corrected, verdict: "pass", observed: "SSO callback returned 200"}
	for i := 0; i < 3; i++ {
		if worked, err := s.Tick(context.Background(), second, []string{"isolated-model-worker"}); err != nil || !worked {
			t.Fatalf("correction stage %d: %t %v", i, worked, err)
		}
	}
	if count, err := s.ReconcileReviewedHandoffs(); err != nil || count != 1 {
		t.Fatalf("corrected handoff: %d %v", count, err)
	}
	state = s.Snapshot()
	if state.Tasks[app.ID].HeadSHA != corrected || state.Tasks[app.ID].BaseSHA != head || state.Agents[appAgent.ID].SessionGeneration != 2 || state.Agents[appAgent.ID].Status != "ready" {
		t.Fatal("same App scenario was not reopened at corrected head")
	}
	if attempt, ok, err := s.ClaimNext(time.Now()); err != nil || ok {
		t.Fatalf("App ran before checkout advanced: %+v %v", attempt, err)
	}
	if err := s.AdvanceAcceptanceBase(app.ID, head, corrected); err != nil {
		t.Fatal(err)
	} // Filesystem fast-forward is exercised in worktree tests.
	retestManifest, err := s.BuildManifest(appAgent.ID, []string{"isolated-model-worker"})
	if err != nil || retestManifest.HeadSHA != corrected || retestManifest.BaseSHA != corrected || len(retestManifest.MessageIDs) != 2 || retestManifest.PreviousAcceptance == nil {
		t.Fatalf("retest manifest lost revision history: %+v %v", retestManifest, err)
	}
	retestPrompt, err := s.ManifestPrompt(retestManifest)
	if err != nil || !strings.Contains(retestPrompt, `head "`+head+`"`) || !strings.Contains(retestPrompt, `head "`+corrected+`"`) || !strings.Contains(retestPrompt, "SSO callback returned 403") {
		t.Fatalf("retest prompt lost handoff heads: %v", err)
	}
	if worked, err := s.Tick(context.Background(), second, []string{"isolated-model-worker"}); err != nil || !worked {
		t.Fatalf("App retest: %t %v", worked, err)
	}
	if count, err := s.ReconcileAcceptanceOutcomes(); err != nil || count != 1 {
		t.Fatalf("accepted retest: %d %v", count, err)
	}
	state = s.Snapshot()
	if state.Tasks[app.ID].Status != "done" || state.Tasks[app.ID].ID != app.ID || state.Tasks[app.ID].HeadSHA != corrected {
		t.Fatal("same scenario did not pass at corrected head")
	}
	accepted := 0
	for _, message := range state.Messages {
		if message.Kind == "accepted" && message.ScenarioID == app.ID && message.HeadSHA == corrected {
			accepted++
		}
	}
	if accepted != 1 {
		t.Fatalf("accepted retest messages=%d", accepted)
	}
}

func TestLinkedAcceptanceRejectsStaleOrUnstructuredResult(t *testing.T) {
	task := Task{ID: "scenario", Title: "expected", ContractVersion: "v1", SourceTaskID: "platform", HeadSHA: strings.Repeat("a", 40)}
	for _, body := range [][]byte{
		[]byte("accepted"),
		[]byte(`{"schema_version":1,"verdict":"pass","scenario_id":"scenario","contract_version":"v1","head_sha":"` + strings.Repeat("b", 40) + `","expected":"expected","observed":"ok"}`),
		[]byte(`{"schema_version":1,"verdict":"pass","scenario_id":"other","contract_version":"v1","head_sha":"` + strings.Repeat("a", 40) + `","expected":"expected","observed":"ok"}`),
	} {
		if _, err := parseAcceptanceResult(body, task); err == nil {
			t.Fatalf("invalid acceptance passed: %s", body)
		}
	}
}

func TestUnstructuredAppOutputCannotCompleteAcceptance(t *testing.T) {
	s := testStore(t)
	platform, _, err := s.CreatePlannedTask("mission", Platform, "change", "canary", "v1")
	if err != nil {
		t.Fatal(err)
	}
	app, appAgent, err := s.CreateLinkedAcceptanceTask(platform.ID, "consumer scenario")
	if err != nil {
		t.Fatal(err)
	}
	base, head := strings.Repeat("a", 40), strings.Repeat("b", 40)
	if err := s.SetBaseSHA(platform.ID, base); err != nil {
		t.Fatal(err)
	}
	runner := invalidAcceptanceRunner{releaseReadyRunner{base: base, head: head}}
	for i := 0; i < 5; i++ {
		if worked, err := s.Tick(context.Background(), runner, []string{"isolated-model-worker"}); err != nil || !worked {
			t.Fatalf("Platform stage %d: %t %v", i, worked, err)
		}
	}
	if count, err := s.ReconcileReviewedHandoffs(); err != nil || count != 1 {
		t.Fatalf("handoff: %d %v", count, err)
	}
	if err := s.SetBaseSHA(app.ID, head); err != nil {
		t.Fatal(err)
	}
	if worked, err := s.Tick(context.Background(), runner, []string{"isolated-model-worker"}); !worked || err == nil {
		t.Fatalf("unstructured acceptance was not rejected: %t %v", worked, err)
	}
	state := s.Snapshot()
	if state.Tasks[app.ID].Status == "done" || state.Agents[appAgent.ID].Status != "failed" {
		t.Fatal("unstructured result completed App acceptance")
	}
	if count, err := s.ReconcileAcceptanceOutcomes(); err != nil || count != 0 {
		t.Fatalf("invalid result was delivered: %d %v", count, err)
	}
}

func TestThirdAppFailureRequiresHumanDecision(t *testing.T) {
	s := testStore(t)
	platform, _, err := s.CreatePlannedTask("mission", Platform, "change", "canary", "v1")
	if err != nil {
		t.Fatal(err)
	}
	app, appAgent, err := s.CreateLinkedAcceptanceTask(platform.ID, "consumer scenario")
	if err != nil {
		t.Fatal(err)
	}
	head := strings.Repeat("a", 40)
	if err := s.SetHeadSHA(platform.ID, head); err != nil {
		t.Fatal(err)
	}
	if err := s.SetHeadSHA(app.ID, head); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(AcceptanceResult{SchemaVersion: 1, Verdict: "fail", ScenarioID: app.ID, ContractVersion: app.ContractVersion, HeadSHA: head, Expected: app.Title, Observed: "still fails"})
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := s.PutArtifact(appAgent.ID, "result-app_acceptance", body)
	if err != nil {
		t.Fatal(err)
	}
	ref := artifactRef(artifact)
	if err := s.update(func(st *State) error {
		p := st.Tasks[platform.ID]
		p.Status, p.CorrectionCount = "publish_wait", 2
		st.Tasks[p.ID] = p
		a := st.Tasks[app.ID]
		a.Status = "done"
		st.Tasks[a.ID] = a
		agent := st.Agents[appAgent.ID]
		agent.Status = "completed"
		st.Agents[agent.ID] = agent
		st.Attempts["third-failure"] = Attempt{ID: "third-failure", AgentID: agent.ID, TaskID: app.ID, Generation: 1, ContractVersion: app.ContractVersion, HeadSHA: head, Status: "completed", OutputRef: &ref}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if count, err := s.ReconcileAcceptanceOutcomes(); err != nil || count != 1 {
		t.Fatalf("limit feedback: %d %v", count, err)
	}
	state := s.Snapshot()
	if state.Tasks[platform.ID].Status != "decision_wait" || state.Tasks[platform.ID].CorrectionCount != 2 || state.Tasks[app.ID].Status != "revision_wait" {
		t.Fatal("third failure silently restarted correction")
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

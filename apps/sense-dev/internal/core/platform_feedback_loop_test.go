package core

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

type feedbackAcceptanceRunner struct{ acceptanceRunner }

func (r feedbackAcceptanceRunner) Run(ctx context.Context, d Dispatch) (RunResult, error) {
	if d.Attempt.Role != "app_acceptance" {
		return r.acceptanceRunner.Run(ctx, d)
	}
	if err := d.BindSession("thread-" + d.Attempt.ID); err != nil {
		return RunResult{}, err
	}
	body, err := json.Marshal(AcceptanceResult{SchemaVersion: 1, Verdict: "pass", ScenarioID: d.Attempt.TaskID, ContractVersion: d.Attempt.ContractVersion, HeadSHA: d.Attempt.HeadSHA, Expected: "consumer scenario", Observed: "feature works but platform trace is absent", PlatformFeedback: []PlatformFeedback{{SchemaVersion: 1, Category: "observability", Summary: "Show request trace", Expected: "trace visible", Observed: "trace absent", Reproduce: "open consumer scenario"}}})
	return RunResult{Output: body}, err
}

func TestPlatformFeedbackAdoptionWaitsForDeploymentBeforeRetest(t *testing.T) {
	base, head, improved := strings.Repeat("a", 40), strings.Repeat("b", 40), strings.Repeat("c", 40)
	s, source, _ := readyAppChange(t, releaseReadyRunner{base: base, head: head})
	acceptance, _, err := s.CreateLinkedAcceptanceTask(source.ID, "consumer scenario")
	if err != nil {
		t.Fatal(err)
	}
	seedAppDeployment(t, s, source)
	if n, err := s.ReconcileReviewedHandoffs(); err != nil || n != 1 {
		t.Fatalf("handoff=%d %v", n, err)
	}
	if err := s.SetBaseSHA(acceptance.ID, head); err != nil {
		t.Fatal(err)
	}
	runner := feedbackAcceptanceRunner{acceptanceRunner{base: base, head: head}}
	if worked, err := s.Tick(context.Background(), runner, []string{"isolated-model-worker"}); err != nil || !worked {
		t.Fatalf("acceptance=%t %v", worked, err)
	}
	if n, err := s.ReconcileAcceptanceOutcomes(); err != nil || n != 1 {
		t.Fatalf("outcome=%d %v", n, err)
	}
	if n, err := s.ReconcilePlatformFeedback(); err != nil || n != 1 {
		t.Fatalf("feedback=%d %v", n, err)
	}
	st := s.Snapshot()
	if st.Tasks[source.ID].Status != "done" {
		t.Fatal("App change blocked by independent feedback")
	}
	var id string
	var loop FeedbackLoop
	for id, loop = range st.FeedbackLoops {
		break
	}
	if id == "" || loop.Status != "pending_decision" || st.Tasks[loop.AnalysisTaskID].Status != "ready" {
		t.Fatalf("missing independent analysis: %+v", loop)
	}
	var analyst Agent
	for _, a := range st.Agents {
		if a.TaskID == loop.AnalysisTaskID {
			analyst = a
		}
	}
	decision, _ := json.Marshal(PlatformDecision{SchemaVersion: 1, FeedbackMessageID: id, Verdict: "adopt", Reason: "Improve trace visibility"})
	artifact, err := s.PutArtifact(analyst.ID, "model_output", decision)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.update(func(st *State) error {
		task := st.Tasks[loop.AnalysisTaskID]
		task.Status = "done"
		st.Tasks[task.ID] = task
		analyst.Status = "completed"
		st.Agents[analyst.ID] = analyst
		st.Attempts["feedback-decision-attempt"] = Attempt{ID: "feedback-decision-attempt", TaskID: task.ID, AgentID: analyst.ID, Role: "feedback", Status: "completed", OutputRef: ptrArtifactRef(artifactRef(artifact)), StartedAt: time.Now().UTC()}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if n, err := s.ReconcilePlatformFeedback(); err != nil || n != 2 {
		t.Fatalf("adoption=%d %v", n, err)
	}
	st = s.Snapshot()
	loop = st.FeedbackLoops[id]
	change := st.Tasks[loop.ImprovementTaskID]
	if change.CheckoutSourceTaskID != source.ID || change.BaseSHA != strings.Repeat("e", 40) || change.Team != Platform || loop.RetestTaskID != "" {
		t.Fatalf("incorrect improvement source or early retest: %+v %+v", change, loop)
	}
	if len(st.Agents) == 0 {
		t.Fatal("missing improvement stages")
	}
	if err := s.update(func(st *State) error {
		change := st.Tasks[loop.ImprovementTaskID]
		change.HeadSHA, change.Status = improved, "publish_wait"
		st.Tasks[change.ID] = change
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if n, err := s.ReconcilePlatformFeedback(); err != nil || n != 0 {
		t.Fatalf("undeployed retest=%d %v", n, err)
	}
	seedAppDeployment(t, s, s.Snapshot().Tasks[change.ID])
	if n, err := s.ReconcilePlatformFeedback(); err != nil || n != 1 {
		t.Fatalf("deployed retest=%d %v", n, err)
	}
	loop = s.Snapshot().FeedbackLoops[id]
	if loop.RetestTaskID == "" || s.Snapshot().Tasks[loop.RetestTaskID].SourceTaskID != change.ID {
		t.Fatalf("missing same-operation retest: %+v", loop)
	}
	if n, err := s.ReconcilePlatformFeedback(); err != nil || n != 0 {
		t.Fatalf("duplicate retest=%d %v", n, err)
	}
	if err := s.update(func(st *State) error {
		retest := st.Tasks[loop.RetestTaskID]
		retest.HeadSHA, retest.Status = improved, "done"
		st.Tasks[retest.ID] = retest
		st.Messages["accepted-retest"] = Message{ID: "accepted-retest", SourceTask: retest.ID, Kind: "accepted", Status: "received", HeadSHA: improved}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if n, err := s.ReconcilePlatformFeedback(); err != nil || n != 2 {
		t.Fatalf("resolved retest=%d %v", n, err)
	}
	if got := s.Snapshot().FeedbackLoops[id].Status; got != "resolved" {
		t.Fatalf("loop status=%s", got)
	}
	if got := s.Snapshot().Tasks[change.ID].Status; got != "done" {
		t.Fatalf("improved change status=%s", got)
	}
}

func ptrArtifactRef(ref ArtifactRef) *ArtifactRef { return &ref }

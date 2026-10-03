package core

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type releaseReadyRunner struct {
	base, head       string
	failVerification bool
	reviewVerdict    string
	wrongReviewHash  bool
}

type gateDecisionRunner struct{ output GateOutput }

func (r gateDecisionRunner) Run(_ context.Context, d Dispatch) (RunResult, error) {
	session := d.Attempt.SessionID
	if session == "" {
		session = "thread-" + d.Attempt.ID
	}
	if err := d.BindSession(session); err != nil {
		return RunResult{}, err
	}
	body, err := json.Marshal(r.output)
	return RunResult{Output: body}, err
}

func (r releaseReadyRunner) Run(_ context.Context, d Dispatch) (RunResult, error) {
	if d.Attempt.Role != "verification" {
		if err := d.BindSession("thread-" + d.Attempt.ID); err != nil {
			return RunResult{}, err
		}
	}
	encode := func(value any) RunResult {
		body, _ := json.Marshal(value)
		return RunResult{Output: body}
	}
	patch := "diff --git a/canary b/canary\n+safe change\n"
	switch d.Attempt.Role {
	case "implementation":
		result := encode(map[string]any{"schema_version": 1, "change": map[string]any{"base_sha": r.base, "head_sha": r.head, "clean": true, "diff_sha256": digest([]byte(patch)), "patch": patch}})
		result.HeadSHA = r.head
		return result, nil
	case "verification":
		status := "passed"
		if r.failVerification {
			status = "failed"
		}
		result := encode(map[string]any{"schema_version": 1, "base_sha": r.base, "head_sha": r.head, "diff_sha256": digest([]byte(patch)), "plan_sha256": strings.Repeat("c", 64), "status": status, "checks": []map[string]string{{"name": "git-diff-check", "status": "passed"}, {"name": "unit", "status": status}}})
		if r.failVerification {
			return result, RunError{Kind: "failed", Err: errors.New("test failed")}
		}
		return result, nil
	case "implementation_review":
		var implementation, verification ArtifactRef
		for _, input := range d.Manifest.StageInputs {
			switch input.Role {
			case "implementation":
				implementation = input.Artifact
			case "verification":
				verification = input.Artifact
			}
		}
		verdict := r.reviewVerdict
		if verdict == "" {
			verdict = "pass"
		}
		if r.wrongReviewHash {
			verification.SHA256 = strings.Repeat("d", 64)
		}
		return encode(map[string]any{"schema_version": 1, "verdict": verdict, "head_sha": r.head, "implementation_sha256": implementation.SHA256, "verification_sha256": verification.SHA256, "reason": "pinned diff and tests reviewed"}), nil
	default:
		return RunResult{Output: []byte("stage output")}, nil
	}
}

func testStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.SeedKnowledge(); err != nil {
		t.Fatal(err)
	}
	return s
}

func taskAgent(t *testing.T, s *Store, team Team, role string) (Task, Agent) {
	t.Helper()
	task, err := s.CreateTask("golden-path", team, "change", "canary", "contract-v1")
	if err != nil {
		t.Fatal(err)
	}
	provider, model := "codex", "gpt-6-sol"
	if role == "release_gate" {
		model = "gpt-6-astra"
	}
	agent, err := s.AddAgent(task.ID, role, provider, model)
	if err != nil {
		t.Fatal(err)
	}
	return task, agent
}

func TestIndependentContextsAndArtifactExchangeSurviveRestart(t *testing.T) {
	s := testStore(t)
	platformTask, platform := taskAgent(t, s, Platform, "implementation")
	_, app := taskAgent(t, s, App, "acceptance")
	private, err := s.PutArtifact(platform.ID, "memo", []byte("platform-only-sentinel"))
	if err != nil {
		t.Fatal(err)
	}
	before, err := s.BuildManifest(app.ID, []string{"read-canary"})
	if err != nil {
		t.Fatal(err)
	}
	if before.AgentMemo != nil || before.TeamProfile.ID == private.ID || before.TeamKnowledge.ID == private.ID || len(before.Inbox) != 0 {
		t.Fatal("private platform context leaked to app")
	}
	if err := s.SetHeadSHA(platformTask.ID, strings.Repeat("a", 40)); err != nil {
		t.Fatal(err)
	}
	result, err := s.PutArtifact(platform.ID, "change_ready", []byte("contract and test evidence"))
	if err != nil {
		t.Fatal(err)
	}
	msg := Message{ID: "handoff-1", CorrelationID: "mission-1", FromAgent: platform.ID, ToAgent: app.ID, SourceTask: platformTask.ID, TargetTask: app.TaskID, Kind: "change_ready", ArtifactRefs: []ArtifactRef{artifactRef(result)}, ContractVersion: "contract-v1", HeadSHA: strings.Repeat("a", 40)}
	if _, err := s.SendMessage(msg); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SendMessage(msg); err != nil {
		t.Fatalf("duplicate message should be idempotent: %v", err)
	}
	if err := s.ReceiveMessage(app.ID, msg.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.ReceiveMessage(app.ID, msg.ID); err != nil {
		t.Fatal(err)
	}
	root := s.root
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(root)
	if err != nil {
		t.Fatal(err)
	}
	after, err := s.BuildManifest(app.ID, []string{"read-canary"})
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Inbox) != 1 || after.Inbox[0].ID != result.ID || after.AgentMemo != nil || len(after.MessageIDs) != 1 || after.MessageIDs[0] != msg.ID {
		t.Fatalf("bad recovered inbox: %+v", after)
	}
	if after.TeamProfile.ID == before.TeamProfile.ID && after.TeamKnowledge.ID == before.TeamKnowledge.ID && after.InputSHA256 == before.InputSHA256 {
		t.Fatal("inbox change did not change input hash")
	}
	if _, err := s.ReadArtifact(private.ID); err != nil {
		t.Fatal(err)
	}
}

func TestRejectWrongRecipientStaleSHAAndTamperedArtifact(t *testing.T) {
	s := testStore(t)
	pt, platform := taskAgent(t, s, Platform, "implementation")
	_, app := taskAgent(t, s, App, "acceptance")
	if err := s.SetHeadSHA(pt.ID, strings.Repeat("a", 40)); err != nil {
		t.Fatal(err)
	}
	a, err := s.PutArtifact(platform.ID, "change_ready", []byte("evidence"))
	if err != nil {
		t.Fatal(err)
	}
	m := Message{ID: "handoff", CorrelationID: "mission", FromAgent: platform.ID, ToAgent: app.ID, SourceTask: pt.ID, TargetTask: app.TaskID, Kind: "change_ready", ArtifactRefs: []ArtifactRef{artifactRef(a)}, ContractVersion: "contract-v1", HeadSHA: strings.Repeat("a", 40)}
	m.ToAgent = platform.ID
	if _, err := s.SendMessage(m); err == nil {
		t.Fatal("same-team/owner message accepted")
	}
	m.ToAgent = app.ID
	m.HeadSHA = strings.Repeat("b", 40)
	if _, err := s.SendMessage(m); err == nil {
		t.Fatal("stale SHA accepted")
	}
	m.HeadSHA = strings.Repeat("a", 40)
	if _, err := s.SendMessage(m); err != nil {
		t.Fatal(err)
	}
	if err := s.SetHeadSHA(pt.ID, strings.Repeat("c", 40)); err != nil {
		t.Fatal(err)
	}
	if err := s.ReceiveMessage(app.ID, m.ID); err == nil {
		t.Fatal("message with changed source head accepted")
	}
	if err := os.WriteFile(filepath.Join(s.root, a.Path), []byte("tampered"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReadArtifact(a.ID); err == nil {
		t.Fatal("tampered artifact accepted")
	}
}

func TestSessionOwnerAndReleaseGate(t *testing.T) {
	s := testStore(t)
	task, stages, err := s.CreatePlannedTask("golden-path", Platform, "change", "canary", "contract-v1")
	if err != nil {
		t.Fatal(err)
	}
	author := stages[2]
	_, gate := taskAgent(t, s, Platform, "release_gate")
	if err := s.SetBaseSHA(task.ID, strings.Repeat("b", 40)); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < len(stages); i++ {
		worked, err := s.Tick(context.Background(), releaseReadyRunner{base: strings.Repeat("b", 40), head: strings.Repeat("a", 40)}, []string{"isolated-model-worker"})
		if err != nil || !worked {
			t.Fatalf("prepare release stage %d: %t %v", i, worked, err)
		}
	}
	var source Artifact
	for _, attempt := range s.Snapshot().Attempts {
		if attempt.AgentID == author.ID && attempt.OutputRef != nil {
			source = s.Snapshot().Artifacts[attempt.OutputRef.ID]
		}
	}
	if source.ID == "" {
		t.Fatal("implementation artifact missing")
	}
	scanRef, err := s.recordReleaseScan(ReleaseScan{BaseSHA: strings.Repeat("b", 40), HeadSHA: strings.Repeat("a", 40), Repository: "yu-min3/kensan-lab", Ref: "refs/heads/feat/canary", Operation: "pr_create", PolicyVersion: ReleasePolicyVersion, Status: "candidate", CommitCount: 1, DiffSHA256: strings.Repeat("c", 64), ScannedAt: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	candidate := ReleaseCandidate{SchemaVersion: 1, Operation: "pr_create", Repository: "yu-min3/kensan-lab", Ref: "refs/heads/feat/canary", HeadSHA: strings.Repeat("a", 40), TargetEnvironment: "github", Impact: "private canary branch only", Rollback: "close PR and delete task branch"}
	if err := s.BindReleaseGateInputs(gate.ID, author.ID, []ArtifactRef{artifactRef(source)}, scanRef, candidate); err != nil {
		t.Fatal(err)
	}
	gm, err := s.BuildManifest(gate.ID, []string{"isolated-model-worker"})
	if err != nil || len(gm.ReviewInputs) != 5 || gm.ReviewAuthorID != author.ID {
		t.Fatalf("gate manifest missing evidence: %+v %v", gm, err)
	}
	if err := s.SetAgentSession(gate.ID, gate.Provider, gate.Model, "thread-gate", gm.InputSHA256, 1); err != nil {
		t.Fatal(err)
	}
	if err := s.SetAgentSession(gate.ID, gate.Provider, gate.Model, s.Snapshot().Agents[author.ID].SessionID, gm.InputSHA256, 1); err == nil {
		t.Fatal("wrong session reuse accepted")
	}
	if err := s.NewSessionGeneration(gate.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.SetAgentSession(gate.ID, gate.Provider, gate.Model, "thread-gate-2", gm.InputSHA256, 1); err == nil {
		t.Fatal("old generation accepted")
	}
	gm, _ = s.BuildManifest(gate.ID, []string{"isolated-model-worker"})
	if err := s.SetAgentSession(gate.ID, gate.Provider, gate.Model, "thread-gate-2", gm.InputSHA256, 2); err != nil {
		t.Fatal(err)
	}
	gateOutput := GateOutput{SchemaVersion: 1, Verdict: "allow", Reason: "private and reversible", Operation: "pr_create", Repository: "yu-min3/kensan-lab", Ref: "refs/heads/feat/canary", HeadSHA: strings.Repeat("a", 40), TargetEnvironment: "github", PolicyVersion: ReleasePolicyVersion, ImplementationSHA256: gm.ReviewInputs[0].SHA256, VerificationSHA256: gm.ReviewInputs[1].SHA256, QualityReviewSHA256: gm.ReviewInputs[2].SHA256, ScanSHA256: gm.ReviewInputs[3].SHA256, CandidateSHA256: gm.ReviewInputs[4].SHA256, SecretFree: true, PrivateTarget: true, Reversible: true}
	if worked, err := s.Tick(context.Background(), gateDecisionRunner{output: gateOutput}, []string{"isolated-model-worker"}); err != nil || !worked {
		t.Fatalf("release gate turn failed: %t %v", worked, err)
	}
	var evidence ArtifactRef
	for _, attempt := range s.Snapshot().Attempts {
		if attempt.AgentID == gate.ID && attempt.OutputRef != nil {
			evidence = *attempt.OutputRef
		}
	}
	if evidence.ID == "" {
		t.Fatal("gate output artifact missing")
	}
	d := ReleaseDecision{AuthorAgentID: author.ID, GateAgentID: gate.ID, Verdict: "allow", Reason: "private and reversible", Operation: "pr_create", Repository: "yu-min3/kensan-lab", Ref: "refs/heads/feat/canary", HeadSHA: strings.Repeat("a", 40), TargetEnvironment: "github", PolicyVersion: ReleasePolicyVersion, ArtifactRefs: []ArtifactRef{artifactRef(source)}, EvidenceRefs: []ArtifactRef{evidence}, ScanRef: scanRef, SecretFree: true, PrivateTarget: true, Reversible: true, ExpiresAt: time.Now().Add(time.Hour)}
	d.GateAgentID = author.ID
	if _, err := s.RecordReleaseDecision(d); err == nil {
		t.Fatal("author self approval accepted")
	}
	d.GateAgentID = gate.ID
	d.PrivateTarget = false
	if _, err := s.RecordReleaseDecision(d); err == nil {
		t.Fatal("public target accepted")
	}
	d.PrivateTarget = true
	d.Reason = "not the gate's reason"
	if _, err := s.RecordReleaseDecision(d); err == nil {
		t.Fatal("caller changed the gate's reason")
	}
	d.Reason = gateOutput.Reason
	d.TargetEnvironment = "private-sense"
	if _, err := s.RecordReleaseDecision(d); err == nil {
		t.Fatal("caller changed the gate's target environment")
	}
	d.TargetEnvironment = gateOutput.TargetEnvironment
	wrongSource, _ := s.PutArtifact(author.ID, "change_ready", []byte("different diff"))
	d.ArtifactRefs = []ArtifactRef{artifactRef(wrongSource)}
	if _, err := s.RecordReleaseDecision(d); err == nil {
		t.Fatal("gate approved an artifact it did not receive")
	}
	d.ArtifactRefs = []ArtifactRef{artifactRef(source)}
	d.Operation = "pr_update"
	if _, err := s.RecordReleaseDecision(d); err == nil {
		t.Fatal("scan reused for another operation")
	}
	d.Operation = "pr_create"
	d, err = s.RecordReleaseDecision(d)
	if err != nil {
		t.Fatal(err)
	}
	_, rejectingGate := taskAgent(t, s, Platform, "release_gate")
	if err := s.BindReleaseGateInputs(rejectingGate.ID, author.ID, []ArtifactRef{artifactRef(source)}, scanRef, candidate); err != nil {
		t.Fatal(err)
	}
	rejectedOutput := gateOutput
	rejectedOutput.Verdict, rejectedOutput.Reason = "deny", "exposure uncertain"
	if worked, err := s.Tick(context.Background(), gateDecisionRunner{output: rejectedOutput}, []string{"isolated-model-worker"}); err != nil || !worked {
		t.Fatalf("rejecting gate did not run: %t %v", worked, err)
	}
	var rejectedEvidence ArtifactRef
	for _, attempt := range s.Snapshot().Attempts {
		if attempt.AgentID == rejectingGate.ID && attempt.OutputRef != nil {
			rejectedEvidence = *attempt.OutputRef
		}
	}
	forged := d
	forged.ID = ""
	forged.GateAgentID = rejectingGate.ID
	forged.EvidenceRefs = []ArtifactRef{rejectedEvidence}
	if _, err := s.RecordReleaseDecision(forged); err == nil {
		t.Fatal("caller turned Astra deny into allow")
	}
	if _, err := s.PreparePublish(d.ID, "merge", d.Repository, d.Ref, d.HeadSHA); err == nil {
		t.Fatal("decision reused for another operation")
	}
	first, err := s.PreparePublish(d.ID, d.Operation, d.Repository, d.Ref, d.HeadSHA)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.PreparePublish(d.ID, d.Operation, d.Repository, d.Ref, d.HeadSHA)
	if err != nil || first.ID != second.ID {
		t.Fatal("publish intent is not idempotent")
	}
	publisher := &fakePublishTransport{}
	published, err := s.RunPublish(context.Background(), d.ID, publisher)
	if err != nil || published.Status != "sent" || published.ExternalID != "external-pr-1" || publisher.executions != 1 {
		t.Fatalf("approved publisher did not execute once: %+v %v", published, err)
	}
	if _, err := s.RunPublish(context.Background(), d.ID, publisher); err != nil || publisher.executions != 1 {
		t.Fatal("completed publish was executed twice")
	}
	if err := s.update(func(st *State) error {
		current := st.Agents[gate.ID]
		current.Status = "failed"
		st.Agents[gate.ID] = current
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PreparePublish(d.ID, d.Operation, d.Repository, d.Ref, d.HeadSHA); err == nil {
		t.Fatal("already-created intent survived gate result invalidation")
	}
	if err := s.update(func(st *State) error {
		current := st.Agents[gate.ID]
		current.Status = "completed"
		st.Agents[gate.ID] = current
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.update(func(st *State) error {
		review := st.Agents[stages[4].ID]
		review.Status = "failed"
		st.Agents[review.ID] = review
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PreparePublish(d.ID, d.Operation, d.Repository, d.Ref, d.HeadSHA); err == nil {
		t.Fatal("already-created intent survived quality review invalidation")
	}
	if err := s.update(func(st *State) error {
		review := st.Agents[stages[4].ID]
		review.Status = "completed"
		st.Agents[review.ID] = review
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetHeadSHA(task.ID, strings.Repeat("b", 40)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PreparePublish(d.ID, d.Operation, d.Repository, d.Ref, d.HeadSHA); err == nil {
		t.Fatal("old decision authorized changed head")
	}
}

type fakePublishTransport struct{ executions int }

func (*fakePublishTransport) Inspect(context.Context, PublishIntent) (string, bool, error) {
	return "", false, nil
}

func (f *fakePublishTransport) Execute(context.Context, PublishIntent) (string, error) {
	f.executions++
	return "external-pr-1", nil
}

func TestReleaseGateRejectsIncompleteOrNegativeQualityEvidence(t *testing.T) {
	for _, tc := range []struct {
		name  string
		turns int
		run   releaseReadyRunner
	}{
		{"before-verification", 3, releaseReadyRunner{}},
		{"failed-verification", 4, releaseReadyRunner{failVerification: true}},
		{"opus-rejected", 5, releaseReadyRunner{reviewVerdict: "fail"}},
		{"wrong-review-hash", 5, releaseReadyRunner{wrongReviewHash: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := testStore(t)
			base, head := strings.Repeat("b", 40), strings.Repeat("a", 40)
			task, stages, err := s.CreatePlannedTask("golden-path", Platform, "change", "canary", "contract-v1")
			if err != nil {
				t.Fatal(err)
			}
			if err := s.SetBaseSHA(task.ID, base); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < tc.turns; i++ {
				worked, err := s.Tick(context.Background(), releaseReadyRunner{base: base, head: head, failVerification: tc.run.failVerification, reviewVerdict: tc.run.reviewVerdict, wrongReviewHash: tc.run.wrongReviewHash}, []string{"isolated-model-worker"})
				if err != nil || !worked {
					t.Fatalf("stage %d: %t %v", i, worked, err)
				}
			}
			var source ArtifactRef
			for _, attempt := range s.Snapshot().Attempts {
				if attempt.AgentID == stages[2].ID && attempt.OutputRef != nil {
					source = *attempt.OutputRef
				}
			}
			if source.ID == "" {
				t.Fatal("fixture implementation missing")
			}
			_, gate := taskAgent(t, s, Platform, "release_gate")
			scanRef, err := s.recordReleaseScan(ReleaseScan{BaseSHA: base, HeadSHA: head, Repository: "yu-min3/kensan-lab", Ref: "refs/heads/feat/canary", Operation: "pr_create", PolicyVersion: ReleasePolicyVersion, Status: "candidate", CommitCount: 1, DiffSHA256: strings.Repeat("c", 64), ScannedAt: time.Now().UTC()})
			if err != nil {
				t.Fatal(err)
			}
			candidate := ReleaseCandidate{SchemaVersion: 1, Operation: "pr_create", Repository: "yu-min3/kensan-lab", Ref: "refs/heads/feat/canary", HeadSHA: head, TargetEnvironment: "github", Impact: "private canary branch only", Rollback: "close PR and delete task branch"}
			if err := s.BindReleaseGateInputs(gate.ID, stages[2].ID, []ArtifactRef{source}, scanRef, candidate); err == nil {
				t.Fatal("release gate accepted incomplete or negative quality evidence")
			}
		})
	}
}

func TestSingleWriterLock(t *testing.T) {
	s := testStore(t)
	if _, err := Open(s.root); err == nil {
		t.Fatal("second controller acquired same state")
	}
}

func TestMockReleaseCannotAuthorizePublish(t *testing.T) {
	s := testStore(t)
	task, author := taskAgent(t, s, Platform, "implementation")
	_, gate := taskAgent(t, s, Platform, "release_gate")
	if err := s.SetHeadSHA(task.ID, strings.Repeat("a", 40)); err != nil {
		t.Fatal(err)
	}
	for _, a := range []Agent{author, gate} {
		m, err := s.BuildManifest(a.ID, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.SetAgentSession(a.ID, a.Provider, a.Model, "mock-"+a.ID, m.InputSHA256, 1); err != nil {
			t.Fatal(err)
		}
	}
	source, err := s.PutArtifact(author.ID, "change_ready", []byte("mock diff"))
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := s.PutArtifact(gate.ID, "review", []byte("mock review"))
	if err != nil {
		t.Fatal(err)
	}
	d := ReleaseDecision{AuthorAgentID: author.ID, GateAgentID: gate.ID, Verdict: "allow", Reason: "mock", Operation: "pr_create", Repository: "yu-min3/kensan-lab", Ref: "refs/heads/feat/mock", HeadSHA: strings.Repeat("a", 40), TargetEnvironment: "github", PolicyVersion: ReleasePolicyVersion, ArtifactRefs: []ArtifactRef{artifactRef(source)}, EvidenceRefs: []ArtifactRef{artifactRef(evidence)}, SecretFree: true, PrivateTarget: true, Reversible: true, ExpiresAt: time.Now().Add(time.Hour)}
	if _, err := s.RecordReleaseDecision(d); err == nil {
		t.Fatal("simulation authorized release")
	}
}

func TestReviewedHandoffAcceptsPinnedBoundedManifestOnly(t *testing.T) {
	for _, scope := range [][]string{{"isolated-model-worker", "bounded-integration"}, {"isolated-model-worker", "unapproved-scope"}} {
		t.Run(scope[1], func(t *testing.T) {
			s := testStore(t)
			task, stages, err := s.CreatePlannedTask("bounded", Platform, "change", "canary", "v1")
			if err != nil {
				t.Fatal(err)
			}
			if err = s.SetBaseSHA(task.ID, strings.Repeat("b", 40)); err != nil {
				t.Fatal(err)
			}
			if _, _, err = s.CreateLinkedAcceptanceTask(task.ID, "consumer"); err != nil {
				t.Fatal(err)
			}
			for range stages {
				if worked, err := s.Tick(context.Background(), releaseReadyRunner{base: strings.Repeat("b", 40), head: strings.Repeat("a", 40)}, scope); err != nil || !worked {
					t.Fatalf("stage %v %v", worked, err)
				}
			}
			want := 0
			if scope[1] == "bounded-integration" {
				want = 1
			}
			if n, err := s.ReconcileReviewedHandoffs(); err != nil || n != want {
				t.Fatalf("handoff %d wanted%d: %v", n, want, err)
			}
			if n, err := s.ReconcileReviewedHandoffs(); err != nil || n != 0 {
				t.Fatalf("duplicate handoff %d %v", n, err)
			}
		})
	}
}

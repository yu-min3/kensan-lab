package core

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func classifiedReleaseFixture(t *testing.T, operationOverride ...string) (*Store, ReleaseDecision, ReleaseCandidate, ApprovalRequest) {
	t.Helper()
	operation, environment := "pr_create", "github"
	if len(operationOverride) > 0 {
		operation, environment = operationOverride[0], "private-canary"
	}
	s := testStore(t)
	task, stages, err := s.CreatePlannedTask("golden-path", Platform, "change", "canary", "contract-v1")
	if err != nil {
		t.Fatal(err)
	}
	base, head := strings.Repeat("b", 40), strings.Repeat("a", 40)
	if err := s.SetBaseSHA(task.ID, base); err != nil {
		t.Fatal(err)
	}
	for range stages {
		if worked, err := s.Tick(context.Background(), releaseReadyRunner{base: base, head: head}, []string{"isolated-model-worker"}); err != nil || !worked {
			t.Fatalf("stages: %t %v", worked, err)
		}
	}
	author := stages[2]
	var source ArtifactRef
	for _, attempt := range s.Snapshot().Attempts {
		if attempt.AgentID == author.ID && attempt.OutputRef != nil {
			source = *attempt.OutputRef
		}
	}
	scanRef, err := s.recordReleaseScan(ReleaseScan{Team: Platform, BaseSHA: base, HeadSHA: head, Repository: "yu-min3/kensan-lab", Ref: "refs/heads/feat/canary", Operation: operation, PolicyVersion: ReleasePolicyVersion, Status: "needs_human", HumanCategories: []string{"auth"}, HumanReasons: []string{"authentication policy change"}, CommitCount: 1, DiffSHA256: strings.Repeat("c", 64), ScannedAt: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	candidate := ReleaseCandidate{SchemaVersion: 1, Operation: operation, Repository: "yu-min3/kensan-lab", Ref: "refs/heads/feat/canary", HeadSHA: head, TargetEnvironment: environment, Impact: "private configuration correction", Rollback: "revert change"}
	_, gate := taskAgent(t, s, Platform, "release_gate")
	if err := s.BindReleaseGateInputs(gate.ID, author.ID, []ArtifactRef{source}, scanRef, candidate); err != nil {
		t.Fatal(err)
	}
	d := gateFixtureDecision(t, s, gate.ID, author.ID, source, scanRef, "needs_human", "human review required")
	d, err = s.RecordReleaseDecision(d)
	if err != nil || len(d.HumanCategories) != 1 {
		t.Fatalf("classified decision: %+v %v", d, err)
	}
	r, err := s.RequestApproval(d.ID)
	if err != nil || r.ScanSHA256 != scanRef.SHA256 || r.CandidateSHA256 == "" {
		t.Fatalf("approval binding: %+v %v", r, err)
	}
	return s, d, candidate, r
}

func gateFixtureDecision(t *testing.T, s *Store, gateID, authorID string, source, scan ArtifactRef, verdict, reason string) ReleaseDecision {
	t.Helper()
	m, err := s.BuildManifest(gateID, []string{"isolated-model-worker"})
	if err != nil {
		t.Fatal(err)
	}
	body, err := s.ReadArtifact(m.ReviewInputs[len(m.ReviewInputs)-1].ID)
	var candidate ReleaseCandidate
	if err != nil || json.Unmarshal(body, &candidate) != nil {
		t.Fatal("candidate missing")
	}
	output := GateOutput{SchemaVersion: 1, Verdict: verdict, Reason: reason, Operation: candidate.Operation, Repository: candidate.Repository, Ref: candidate.Ref, HeadSHA: candidate.HeadSHA, TargetEnvironment: candidate.TargetEnvironment, PolicyVersion: ReleasePolicyVersion, ImplementationSHA256: m.ReviewInputs[0].SHA256, VerificationSHA256: m.ReviewInputs[1].SHA256, QualityReviewSHA256: m.ReviewInputs[2].SHA256, ScanSHA256: scan.SHA256, CandidateSHA256: m.ReviewInputs[len(m.ReviewInputs)-1].SHA256, SecretFree: true, PrivateTarget: true, Reversible: true, CIComplete: true}
	if worked, err := s.Tick(context.Background(), gateDecisionRunner{output: output}, []string{"isolated-model-worker"}); err != nil || !worked {
		t.Fatalf("gate turn: %t %v", worked, err)
	}
	var evidence ArtifactRef
	for _, a := range s.Snapshot().Attempts {
		if a.AgentID == gateID && a.OutputRef != nil {
			evidence = *a.OutputRef
		}
	}
	return ReleaseDecision{AuthorAgentID: authorID, GateAgentID: gateID, Verdict: verdict, Reason: reason, Operation: output.Operation, Repository: output.Repository, Ref: output.Ref, HeadSHA: output.HeadSHA, TargetEnvironment: output.TargetEnvironment, PolicyVersion: ReleasePolicyVersion, ArtifactRefs: []ArtifactRef{source}, EvidenceRefs: []ArtifactRef{evidence}, ScanRef: scan, SecretFree: true, PrivateTarget: true, Reversible: true, CIComplete: true, ExpiresAt: time.Now().Add(time.Hour)}
}

func TestClassifiedReleaseNeedsApprovalAndFreshIndependentGate(t *testing.T) {
	s, origin, candidate, request := classifiedReleaseFixture(t)
	if _, err := s.PreparePublish(origin.ID, origin.Operation, origin.Repository, origin.Ref, origin.HeadSHA); err == nil {
		t.Fatal("unapproved classification published")
	}
	if _, err := s.DecideApproval(request.ID, "tap", origin.Operation, origin.HeadSHA, "approved"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PreparePublish(origin.ID, origin.Operation, origin.Repository, origin.Ref, origin.HeadSHA); err == nil {
		t.Fatal("human approval replaced independent Gate")
	}
	_, gate := taskAgent(t, s, Platform, "release_gate")
	if err := s.BindReleaseGateInputs(gate.ID, origin.AuthorAgentID, origin.ArtifactRefs, origin.ScanRef, candidate); err != nil {
		t.Fatal(err)
	}
	if len(s.Snapshot().Agents[gate.ID].ReviewInputs) != 6 {
		t.Fatal("fresh Gate missing approval artifact")
	}
	d := gateFixtureDecision(t, s, gate.ID, origin.AuthorAgentID, origin.ArtifactRefs[0], origin.ScanRef, "allow", "approved and independently verified")
	if _, err := s.RecordReleaseDecision(d); err == nil {
		t.Fatal("classified allow omitted approval ID")
	}
	d.ApprovalID = request.ID
	d, err := s.RecordReleaseDecision(d)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.PreparePublish(d.ID, d.Operation, d.Repository, d.Ref, d.HeadSHA); err != nil {
		t.Fatalf("approved fresh Gate publish denied: %v", err)
	}
	if err := s.update(func(st *State) error {
		r := st.Approvals[request.ID]
		r.ExpiresAt = time.Now().Add(-time.Second)
		st.Approvals[r.ID] = r
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PreparePublish(d.ID, d.Operation, d.Repository, d.Ref, d.HeadSHA); err == nil {
		t.Fatal("expired approval reused")
	}
}

func TestHumanApprovalCannotBeBorrowedOrDowngraded(t *testing.T) {
	s, origin, candidate, request := classifiedReleaseFixture(t)
	if _, err := s.DecideApproval(request.ID, "tap", origin.Operation, origin.HeadSHA, "approved"); err != nil {
		t.Fatal(err)
	}
	_, gate := taskAgent(t, s, Platform, "release_gate")
	if err := s.BindReleaseGateInputs(gate.ID, origin.AuthorAgentID, origin.ArtifactRefs, origin.ScanRef, candidate); err != nil {
		t.Fatal(err)
	}
	d := gateFixtureDecision(t, s, gate.ID, origin.AuthorAgentID, origin.ArtifactRefs[0], origin.ScanRef, "allow", "approved and independently verified")
	d.ApprovalID, d.HumanCategories, d.HumanReasons = request.ID, origin.HumanCategories, origin.HumanReasons
	scan, err := s.loadReleaseScan(d.ScanRef, d)
	if err != nil {
		t.Fatal(err)
	}
	if !approvalMatches(s.Snapshot(), d, scan, request.CandidateSHA256) {
		t.Fatal("baseline approval mismatch")
	}
	for _, mutate := range []func(*ReleaseDecision){
		func(d *ReleaseDecision) { d.AuthorAgentID = gate.ID },
		func(d *ReleaseDecision) { d.GateAgentID = origin.GateAgentID },
		func(d *ReleaseDecision) { d.HeadSHA = strings.Repeat("d", 40) },
		func(d *ReleaseDecision) { d.Operation = "merge" },
		func(d *ReleaseDecision) { d.ScanRef.SHA256 = strings.Repeat("e", 64) },
		func(d *ReleaseDecision) { d.HumanCategories = nil },
	} {
		wrong := d
		mutate(&wrong)
		if approvalMatches(s.Snapshot(), wrong, scan, request.CandidateSHA256) {
			t.Fatalf("borrowed approval: %+v", wrong)
		}
	}
	if approvalMatches(s.Snapshot(), d, scan, strings.Repeat("f", 64)) {
		t.Fatal("approval reused for changed candidate")
	}
	denied := scan
	denied.Status = "deny"
	if approvalMatches(s.Snapshot(), d, denied, request.CandidateSHA256) {
		t.Fatal("human approval overrode hard deny")
	}
	// The later model cannot remove controller classifications.
	d.HumanCategories, d.HumanReasons = nil, nil
	stored, err := s.RecordReleaseDecision(d)
	if err != nil || len(stored.HumanCategories) == 0 {
		t.Fatalf("classification downgrade: %+v %v", stored, err)
	}
}

type expireApprovalTransport struct {
	store      *Store
	requestID  string
	executions int
}

func (t *expireApprovalTransport) Inspect(context.Context, PublishIntent) (string, bool, error) {
	err := t.store.update(func(st *State) error {
		r := st.Approvals[t.requestID]
		r.ExpiresAt = time.Now().Add(-time.Second)
		st.Approvals[r.ID] = r
		return nil
	})
	return "", false, err
}

func (t *expireApprovalTransport) Execute(context.Context, PublishIntent) (string, error) {
	t.executions++
	return "unexpected", nil
}

func TestPublisherRejectsApprovalExpiryDuringInspection(t *testing.T) {
	s, origin, candidate, request := classifiedReleaseFixture(t)
	if _, err := s.DecideApproval(request.ID, "tap", origin.Operation, origin.HeadSHA, "approved"); err != nil {
		t.Fatal(err)
	}
	_, gate := taskAgent(t, s, Platform, "release_gate")
	if err := s.BindReleaseGateInputs(gate.ID, origin.AuthorAgentID, origin.ArtifactRefs, origin.ScanRef, candidate); err != nil {
		t.Fatal(err)
	}
	d := gateFixtureDecision(t, s, gate.ID, origin.AuthorAgentID, origin.ArtifactRefs[0], origin.ScanRef, "allow", "approved and independently verified")
	d.ApprovalID = request.ID
	d, err := s.RecordReleaseDecision(d)
	if err != nil {
		t.Fatal(err)
	}
	transport := &expireApprovalTransport{store: s, requestID: request.ID}
	if _, err := s.RunPublish(context.Background(), d.ID, transport); err == nil {
		t.Fatal("expired approval executed")
	}
	if transport.executions != 0 {
		t.Fatalf("transport Execute count=%d", transport.executions)
	}
}

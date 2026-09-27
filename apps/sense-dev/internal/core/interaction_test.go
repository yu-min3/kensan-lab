package core

import (
	"strings"
	"testing"
	"time"
)

func TestQuestionAnswerIdempotentAndSHABound(t *testing.T) {
	s := testStore(t)
	task, agent := taskAgent(t, s, Platform, "requirements")
	q, err := s.AskQuestion(agent.ID, "Which contract applies?")
	if err != nil {
		t.Fatal(err)
	}
	answered, err := s.AnswerQuestion(q.ID, "mobile-action-1", "draft-v1")
	if err != nil || answered.Status != "answered" {
		t.Fatalf("answer: %+v %v", answered, err)
	}
	if _, err := s.AnswerQuestion(q.ID, "mobile-action-1", "draft-v1"); err != nil {
		t.Fatalf("retry should be idempotent: %v", err)
	}
	if _, err := s.AnswerQuestion(q.ID, "mobile-action-2", "different"); err == nil {
		t.Fatal("double answer accepted")
	}
	stale, err := s.AskQuestion(agent.ID, "Old SHA?")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetHeadSHA(task.ID, strings.Repeat("a", 40)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AnswerQuestion(stale.ID, "mobile-action-3", "yes"); err == nil {
		t.Fatal("stale question accepted")
	}
	root := s.root
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if s.Snapshot().Questions[q.ID].Answer != "draft-v1" {
		t.Fatal("answer lost on restart")
	}
}

func TestApprovalRequiresExactDecisionAndNeverPublishes(t *testing.T) {
	s := testStore(t)
	task, author := taskAgent(t, s, Platform, "implementation")
	_, gate := taskAgent(t, s, Platform, "release_gate")
	sha := strings.Repeat("a", 40)
	if err := s.SetHeadSHA(task.ID, sha); err != nil {
		t.Fatal(err)
	}
	for _, a := range []Agent{author, gate} {
		m, err := s.BuildManifest(a.ID, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.SetAgentSession(a.ID, a.Provider, a.Model, "session-"+a.ID, m.InputSHA256, 1); err != nil {
			t.Fatal(err)
		}
	}
	source, err := s.PutArtifact(author.ID, "change", []byte("diff"))
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := s.PutArtifact(gate.ID, "review", []byte("needs human"))
	if err != nil {
		t.Fatal(err)
	}
	d, err := s.RecordReleaseDecision(ReleaseDecision{AuthorAgentID: author.ID, GateAgentID: gate.ID, Verdict: "needs_human", Reason: "existing public app may change", Operation: "merge", Repository: "yu-min3/kensan-lab", Ref: "refs/heads/feat/canary", HeadSHA: sha, TargetEnvironment: "private-canary", PolicyVersion: ReleasePolicyVersion, ArtifactRefs: []ArtifactRef{artifactRef(source)}, EvidenceRefs: []ArtifactRef{artifactRef(evidence)}, ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.RequestApproval(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	retry, err := s.RequestApproval(d.ID)
	if err != nil || retry.ID != r.ID {
		t.Fatal("approval request not idempotent")
	}
	if _, err := s.DecideApproval(r.ID, "tap-1", "deploy", sha, "approved"); err == nil {
		t.Fatal("different operation accepted")
	}
	if _, err := s.DecideApproval(r.ID, "tap-1", "merge", strings.Repeat("b", 40), "approved"); err == nil {
		t.Fatal("different SHA accepted")
	}
	approved, err := s.DecideApproval(r.ID, "tap-1", "merge", sha, "approved")
	if err != nil || approved.Status != "approved" {
		t.Fatalf("approval: %+v %v", approved, err)
	}
	if _, err := s.DecideApproval(r.ID, "tap-1", "merge", sha, "approved"); err != nil {
		t.Fatal(err)
	}
	if len(s.Snapshot().Intents) != 0 {
		t.Fatal("approval invoked publisher")
	}
	if _, err := s.PreparePublish(d.ID, "merge", d.Repository, d.Ref, sha); err == nil {
		t.Fatal("human approval bypassed independent allow gate")
	}
}

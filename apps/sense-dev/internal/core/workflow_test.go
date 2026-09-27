package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(t.TempDir())
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
	task, author := taskAgent(t, s, Platform, "implementation")
	_, gate := taskAgent(t, s, Platform, "release_gate")
	if err := s.SetHeadSHA(task.ID, strings.Repeat("a", 40)); err != nil {
		t.Fatal(err)
	}
	am, err := s.BuildManifest(author.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	gm, err := s.BuildManifest(gate.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetAgentSession(author.ID, author.Provider, author.Model, "thread-author", am.InputSHA256, 1); err != nil {
		t.Fatal(err)
	}
	if err := s.SetAgentSession(gate.ID, gate.Provider, gate.Model, "thread-gate", gm.InputSHA256, 1); err != nil {
		t.Fatal(err)
	}
	if err := s.SetAgentSession(gate.ID, gate.Provider, gate.Model, "thread-author", gm.InputSHA256, 1); err == nil {
		t.Fatal("wrong session reuse accepted")
	}
	if err := s.NewSessionGeneration(gate.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.SetAgentSession(gate.ID, gate.Provider, gate.Model, "thread-gate-2", gm.InputSHA256, 1); err == nil {
		t.Fatal("old generation accepted")
	}
	gm, _ = s.BuildManifest(gate.ID, nil)
	if err := s.SetAgentSession(gate.ID, gate.Provider, gate.Model, "thread-gate-2", gm.InputSHA256, 2); err != nil {
		t.Fatal(err)
	}
	source, _ := s.PutArtifact(author.ID, "change_ready", []byte("diff"))
	evidence, _ := s.PutArtifact(gate.ID, "release_review", []byte("checked scope, secrets, exposure and rollback"))
	d := ReleaseDecision{AuthorAgentID: author.ID, GateAgentID: gate.ID, Verdict: "allow", Reason: "private and reversible", Operation: "pr_create", Repository: "yu-min3/kensan-lab", Ref: "refs/heads/feat/canary", HeadSHA: strings.Repeat("a", 40), TargetEnvironment: "github", PolicyVersion: ReleasePolicyVersion, ArtifactRefs: []ArtifactRef{artifactRef(source)}, EvidenceRefs: []ArtifactRef{artifactRef(evidence)}, SecretFree: true, PrivateTarget: true, Reversible: true, ExpiresAt: time.Now().Add(time.Hour)}
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
	d, err = s.RecordReleaseDecision(d)
	if err != nil {
		t.Fatal(err)
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
	if err := s.SetHeadSHA(task.ID, strings.Repeat("b", 40)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PreparePublish(d.ID, d.Operation, d.Repository, d.Ref, d.HeadSHA); err == nil {
		t.Fatal("old decision authorized changed head")
	}
}

func TestSingleWriterLock(t *testing.T) {
	s := testStore(t)
	if _, err := Open(s.root); err == nil {
		t.Fatal("second controller acquired same state")
	}
}

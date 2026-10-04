package core

import "testing"

func feedbackActors(t *testing.T, s *Store) (Task, Agent, Task, Agent) {
	t.Helper()
	appTask, err := s.CreateTask("mission", App, "change", "app work", "contract-v1")
	if err != nil {
		t.Fatal(err)
	}
	app, err := s.AddAgent(appTask.ID, "implementation", "codex", "gpt-6-sol")
	if err != nil {
		t.Fatal(err)
	}
	platformTask, err := s.CreateTask("mission", Platform, "analysis", "review feedback", "contract-v1")
	if err != nil {
		t.Fatal(err)
	}
	platform, err := s.AddAgent(platformTask.ID, "feedback", "claude", "claude-opus-5-5")
	if err != nil {
		t.Fatal(err)
	}
	return appTask, app, platformTask, platform
}

func validFeedback() PlatformFeedback {
	return PlatformFeedback{SchemaVersion: 1, Category: "deployment", Summary: "image pull failed", Expected: "pod ready", Observed: "image pull error", Reproduce: "deploy and inspect pod condition"}
}

func TestPlatformFeedbackDecisionRoundTrip(t *testing.T) {
	s := testStore(t)
	appTask, app, platformTask, platform := feedbackActors(t, s)
	a, err := s.PutPlatformFeedback(app.ID, validFeedback())
	if err != nil {
		t.Fatal(err)
	}
	m := Message{ID: "feedback-1", CorrelationID: "loop-1", FromAgent: app.ID, ToAgent: platform.ID, SourceTask: appTask.ID, TargetTask: platformTask.ID, Kind: "platform_feedback", ArtifactRefs: []ArtifactRef{artifactRef(a)}, ContractVersion: "contract-v1"}
	if _, err := s.SendMessage(m); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SendMessage(m); err != nil {
		t.Fatalf("same feedback should be idempotent: %v", err)
	}
	if err := s.ReceiveMessage(platform.ID, m.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.ReceiveMessage(platform.ID, m.ID); err != nil {
		t.Fatal(err)
	}
	f, err := s.ReadPlatformFeedback(m.ID)
	if err != nil || f.Summary != "image pull failed" {
		t.Fatalf("feedback lost: %+v %v", f, err)
	}
	d, err := s.PutPlatformDecision(platform.ID, PlatformDecision{SchemaVersion: 1, FeedbackMessageID: m.ID, Verdict: "adopt", Reason: "reproduce with a scoped pull credential"})
	if err != nil {
		t.Fatal(err)
	}
	reply := Message{ID: "decision-1", CorrelationID: m.CorrelationID, ReplyTo: m.ID, FromAgent: platform.ID, ToAgent: app.ID, SourceTask: platformTask.ID, TargetTask: appTask.ID, Kind: "platform_decision", ArtifactRefs: []ArtifactRef{artifactRef(d)}, ContractVersion: "contract-v1"}
	if _, err := s.SendMessage(reply); err != nil {
		t.Fatal(err)
	}
	if err := s.ReceiveMessage(app.ID, reply.ID); err != nil {
		t.Fatal(err)
	}
	got, err := s.ReadPlatformDecision(reply.ID)
	if err != nil || got.Verdict != "adopt" {
		t.Fatalf("decision lost: %+v %v", got, err)
	}
	if s.Snapshot().Tasks[appTask.ID].Status != "ready" {
		t.Fatal("feedback reply blocked App task")
	}
	other, err := s.PutPlatformDecision(platform.ID, PlatformDecision{SchemaVersion: 1, FeedbackMessageID: m.ID, Verdict: "reject", Reason: "second verdict"})
	if err != nil {
		t.Fatal(err)
	}
	reply.ID, reply.ArtifactRefs = "decision-2", []ArtifactRef{artifactRef(other)}
	if _, err := s.SendMessage(reply); err == nil {
		t.Fatal("second decision for one feedback accepted")
	}
}

func TestPlatformEnvelopeRejectsWrongRouteVersionAndReply(t *testing.T) {
	s := testStore(t)
	appTask, app, platformTask, platform := feedbackActors(t, s)
	bad := validFeedback()
	bad.SchemaVersion = 2
	if _, err := s.PutPlatformFeedback(app.ID, bad); err == nil {
		t.Fatal("unknown feedback schema accepted")
	}
	a, err := s.PutPlatformFeedback(app.ID, validFeedback())
	if err != nil {
		t.Fatal(err)
	}
	m := Message{ID: "feedback", CorrelationID: "loop", FromAgent: app.ID, ToAgent: platform.ID, SourceTask: appTask.ID, TargetTask: platformTask.ID, Kind: "platform_feedback", ArtifactRefs: []ArtifactRef{artifactRef(a)}, ContractVersion: "contract-v1"}
	wrong := m
	wrong.FromAgent, wrong.ToAgent = platform.ID, app.ID
	wrong.SourceTask, wrong.TargetTask = platformTask.ID, appTask.ID
	if _, err := s.SendMessage(wrong); err == nil {
		t.Fatal("reversed feedback accepted")
	}
	wrong = m
	wrong.ContractVersion = "old-contract"
	if _, err := s.SendMessage(wrong); err == nil {
		t.Fatal("stale contract accepted")
	}
	if _, err := s.SendMessage(m); err != nil {
		t.Fatal(err)
	}
	d, err := s.PutPlatformDecision(platform.ID, PlatformDecision{SchemaVersion: 1, FeedbackMessageID: m.ID, Verdict: "defer", Reason: "need evidence"})
	if err != nil {
		t.Fatal(err)
	}
	reply := Message{ID: "decision", CorrelationID: m.CorrelationID, ReplyTo: m.ID, FromAgent: platform.ID, ToAgent: app.ID, SourceTask: platformTask.ID, TargetTask: appTask.ID, Kind: "platform_decision", ArtifactRefs: []ArtifactRef{artifactRef(d)}, ContractVersion: "contract-v1"}
	if _, err := s.SendMessage(reply); err == nil {
		t.Fatal("decision sent before feedback receipt")
	}
	if err := s.ReceiveMessage(platform.ID, m.ID); err != nil {
		t.Fatal(err)
	}
	reply.CorrelationID = "another-loop"
	if _, err := s.SendMessage(reply); err == nil {
		t.Fatal("decision with wrong correlation accepted")
	}
	reply.CorrelationID = m.CorrelationID
	reply.ReplyTo = "missing"
	if _, err := s.SendMessage(reply); err == nil {
		t.Fatal("decision with wrong reply accepted")
	}
}

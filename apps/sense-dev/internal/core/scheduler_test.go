package core

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeRunner struct {
	mu       sync.Mutex
	seen     []Dispatch
	failKind string
}

func (f *fakeRunner) Run(_ context.Context, d Dispatch) (RunResult, error) {
	if err := d.BindSession("fake-" + d.Attempt.ID); err != nil {
		return RunResult{}, err
	}
	f.mu.Lock()
	f.seen = append(f.seen, d)
	f.mu.Unlock()
	if f.failKind != "" {
		return RunResult{}, RunError{Kind: f.failKind, Err: errors.New("injected")}
	}
	return RunResult{Output: []byte("result for " + d.Attempt.AgentID)}, nil
}

func TestSchedulerIndependentTeamsAndDependencies(t *testing.T) {
	s := testStore(t)
	platform, err := s.CreateTask("mission", Platform, "change", "platform", "v1")
	if err != nil {
		t.Fatal(err)
	}
	app, err := s.CreateTask("mission", App, "acceptance", "app", "v1")
	if err != nil {
		t.Fatal(err)
	}
	p1, err := s.AddAgent(platform.ID, "requirements", "claude", "fable")
	if err != nil {
		t.Fatal(err)
	}
	p2, err := s.AddAgentWithDeps(platform.ID, "design_review", "codex", "astra", []string{p1.ID})
	if err != nil {
		t.Fatal(err)
	}
	appAgent, err := s.AddAgent(app.ID, "app_feedback", "claude", "fable")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddAgentWithDeps(platform.ID, "implementation", "codex", "sol", []string{p2.ID}); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{}
	for i := 0; i < 4; i++ {
		worked, err := s.Tick(context.Background(), runner, []string{"read-only"})
		if err != nil || !worked {
			t.Fatalf("tick %d: worked=%t err=%v", i, worked, err)
		}
	}
	st := s.Snapshot()
	if st.Tasks[platform.ID].Status != "publish_wait" || st.Tasks[app.ID].Status != "done" {
		t.Fatalf("wrong task status: %+v", st.Tasks)
	}
	if len(st.Attempts) != 4 || len(runner.seen) != 4 {
		t.Fatalf("attempts=%d seen=%d", len(st.Attempts), len(runner.seen))
	}
	for _, d := range runner.seen {
		if d.Attempt.AgentID == appAgent.ID && strings.Contains(d.Prompt, "Platform team:") {
			t.Fatal("platform context leaked to App")
		}
		if d.Attempt.AgentID == p2.ID && st.Agents[p1.ID].Status != "completed" {
			t.Fatal("dependent review started too early")
		}
		if d.Attempt.InputHash != d.Manifest.InputSHA256 || d.Attempt.Generation != 1 {
			t.Fatal("attempt input not pinned")
		}
	}
}

func TestSchedulerProviderSlotPauseAndRecovery(t *testing.T) {
	s := testStore(t)
	_, first := taskAgent(t, s, Platform, "first")
	_, second := taskAgent(t, s, App, "second")
	claim, ok, err := s.ClaimNext(time.Now())
	if err != nil || !ok {
		t.Fatalf("first claim: %v %t", err, ok)
	}
	if _, ok, err := s.ClaimNext(time.Now()); err != nil || ok {
		t.Fatalf("provider double booked: %v %t", err, ok)
	}
	if claim.AgentID != first.ID && claim.AgentID != second.ID {
		t.Fatal("unexpected agent")
	}
	root := s.root
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RecoverInterrupted(); err != nil {
		t.Fatal(err)
	}
	if s.Snapshot().Attempts[claim.ID].Status != "interrupted" {
		t.Fatal("running turn silently replayed")
	}
	worked, err := s.Tick(context.Background(), &fakeRunner{}, nil)
	if err != nil || !worked {
		t.Fatalf("independent task blocked by interrupted task: %v", err)
	}
	if err := s.SetStopped(true); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := s.ClaimNext(time.Now()); ok {
		t.Fatal("stop did not suppress dispatch")
	}
	if err := s.SetStopped(false); err != nil {
		t.Fatal(err)
	}
	if err := s.SetMacPriority(time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := s.ClaimNext(time.Now()); ok {
		t.Fatal("Mac priority did not suppress dispatch")
	}
}

func TestSchedulerQuotaWaitDoesNotBlockAnotherTeam(t *testing.T) {
	s := testStore(t)
	_, _ = taskAgent(t, s, Platform, "requirements")
	_, _ = taskAgent(t, s, App, "acceptance")
	runner := &fakeRunner{failKind: "quota_wait"}
	worked, err := s.Tick(context.Background(), runner, nil)
	if err != nil || !worked {
		t.Fatalf("quota injection: %v", err)
	}
	st := s.Snapshot()
	var waiting Agent
	for _, a := range st.Agents {
		if a.Status == "quota_wait" {
			waiting = a
		}
	}
	if waiting.ID == "" || waiting.RetryAfter == nil {
		t.Fatal("quota wait not persisted")
	}
	if err := s.SetMacPriority(time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := s.ClaimNext(time.Now()); ok {
		t.Fatal("priority pause did not apply")
	}
	if err := s.SetMacPriority(time.Time{}); err != nil {
		t.Fatal(err)
	}
	worked, err = s.Tick(context.Background(), &fakeRunner{}, nil)
	if err != nil || !worked {
		t.Fatalf("other team should continue: %v", err)
	}
}

func TestSessionIDCannotCrossAgents(t *testing.T) {
	s := testStore(t)
	_, platform := taskAgent(t, s, Platform, "requirements")
	first, err := s.BuildManifest(platform.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetAgentSession(platform.ID, platform.Provider, platform.Model, "shared-thread", first.InputSHA256, 1); err != nil {
		t.Fatal(err)
	}
	claim, ok, err := s.ClaimNext(time.Now())
	if err != nil || !ok || claim.AgentID != platform.ID {
		t.Fatalf("claim: %v %+v", err, claim)
	}
	if err := s.SetAttemptInput(claim.ID, first.InputSHA256); err != nil {
		t.Fatal(err)
	}
	if err := s.SetAttemptSession(claim.ID, "shared-thread"); err != nil {
		t.Fatal(err)
	}
	_, app := taskAgent(t, s, App, "acceptance")
	second, err := s.BuildManifest(app.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetAgentSession(app.ID, app.Provider, app.Model, "shared-thread", second.InputSHA256, 1); err == nil {
		t.Fatal("provider session reused across team boundary")
	}
	if err := s.FailAttempt(claim.ID, "interrupted", "test", time.Time{}); err != nil {
		t.Fatal(err)
	}
	if err := s.NewSessionGeneration(platform.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.SetAgentSession(app.ID, app.Provider, app.Model, "shared-thread", second.InputSHA256, 1); err == nil {
		t.Fatal("historical session reused across team boundary")
	}
}

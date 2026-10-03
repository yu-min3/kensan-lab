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
	p1, err := s.AddAgent(platform.ID, "requirements", "claude", "claude-opus-5-5")
	if err != nil {
		t.Fatal(err)
	}
	p2, err := s.AddAgentWithDeps(platform.ID, "design_review", "codex", "astra", []string{p1.ID})
	if err != nil {
		t.Fatal(err)
	}
	appAgent, err := s.AddAgent(app.ID, "app_feedback", "claude", "claude-opus-5-5")
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

func TestReleaseGateDoesNotRunWithoutPinnedInputs(t *testing.T) {
	s := testStore(t)
	_, gate := taskAgent(t, s, Platform, "release_gate")
	if attempt, worked, err := s.ClaimNext(time.Now()); err != nil || worked || attempt.ID != "" {
		t.Fatalf("unbound release gate was dispatched: %+v %t %v", attempt, worked, err)
	}
	if s.Snapshot().Agents[gate.ID].Status != "ready" {
		t.Fatal("unbound release gate state changed")
	}
}

func TestIsolatedDispatchWaitsForPinnedTaskBase(t *testing.T) {
	s := testStore(t)
	task, _, err := s.CreatePlannedTask("mission", Platform, "change", "pinned worktree", "v1")
	if err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{}
	if worked, err := s.Tick(context.Background(), runner, []string{"isolated-model-worker"}); err != nil || worked {
		t.Fatalf("unprovisioned task was dispatched: %t %v", worked, err)
	}
	if err := s.SetBaseSHA(task.ID, strings.Repeat("a", 40)); err != nil {
		t.Fatal(err)
	}
	if worked, err := s.Tick(context.Background(), runner, []string{"isolated-model-worker"}); err != nil || !worked {
		t.Fatalf("provisioned task not dispatched: %t %v", worked, err)
	}
}

func TestCommittedImplementationHeadBindsReviewInput(t *testing.T) {
	s := testStore(t)
	task, err := s.CreateTask("mission", Platform, "change", "canary", "v1")
	if err != nil {
		t.Fatal(err)
	}
	base, head := strings.Repeat("a", 40), strings.Repeat("b", 40)
	if err := s.SetBaseSHA(task.ID, base); err != nil {
		t.Fatal(err)
	}
	author, err := s.AddAgent(task.ID, "implementation", "codex", "gpt-6-sol")
	if err != nil {
		t.Fatal(err)
	}
	reviewer, err := s.AddAgentWithDeps(task.ID, "implementation_review", "claude", "opus", []string{author.ID})
	if err != nil {
		t.Fatal(err)
	}
	attempt, ok, err := s.ClaimNext(time.Now())
	if err != nil || !ok || attempt.AgentID != author.ID {
		t.Fatalf("claim: %+v %t %v", attempt, ok, err)
	}
	if err := s.SetAttemptInput(attempt.ID, strings.Repeat("c", 64)); err != nil {
		t.Fatal(err)
	}
	artifact, err := s.PutArtifact(author.ID, "result-implementation", []byte("fixed diff"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CompleteAttemptAtHead(attempt.ID, artifactRef(artifact), base); err == nil {
		t.Fatal("unchanged base promoted")
	}
	if err := s.CompleteAttemptAtHead(attempt.ID, artifactRef(artifact), head); err != nil {
		t.Fatal(err)
	}
	state := s.Snapshot()
	if state.Tasks[task.ID].HeadSHA != head || state.Attempts[attempt.ID].HeadSHA != head {
		t.Fatal("implementation head not bound atomically")
	}
	manifest, err := s.BuildManifest(reviewer.ID, []string{"read-only"})
	if err != nil || manifest.HeadSHA != head || len(manifest.StageInputs) != 1 || manifest.StageInputs[0].Artifact.ID != artifact.ID {
		t.Fatalf("review input not bound to implementation: %+v %v", manifest, err)
	}
	if err := s.SetHeadSHA(task.ID, strings.Repeat("d", 40)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.BuildManifest(reviewer.ID, nil); err == nil {
		t.Fatal("stale implementation artifact accepted for new head")
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

func TestBoundedRunStopsBeforeClaimingBeyondBudget(t *testing.T) {
	s := testStore(t)
	for _, title := range []string{"first", "second"} {
		if _, _, err := s.CreatePlannedTask("bounded", App, "analysis", title, "v1"); err != nil {
			t.Fatal(err)
		}
	}
	runner := &fakeRunner{}
	if worked, err := s.TickBounded(context.Background(), runner, nil, 1); err != nil || !worked {
		t.Fatalf("first tick: %v %v", worked, err)
	}
	for i := 0; i < 3; i++ {
		if worked, err := s.TickBounded(context.Background(), runner, nil, 1); err != nil || worked {
			t.Fatalf("budget exceeded: %v %v", worked, err)
		}
	}
	if len(s.Snapshot().Attempts) != 1 || len(runner.seen) != 1 {
		t.Fatal("budget allowed an extra claim or inference")
	}
}

func TestBoundedRunStopsAfterFailureBeforeOtherTeamDispatch(t *testing.T) {
	for _, kind := range []string{"failed", "quota_wait", "retry_wait", "auth_required", "interrupted"} {
		t.Run(kind, func(t *testing.T) {
			s := testStore(t)
			for _, title := range []string{"first", "second"} {
				if _, _, err := s.CreatePlannedTask("bounded", App, "analysis", title, "v1"); err != nil {
					t.Fatal(err)
				}
			}
			runner := &fakeRunner{failKind: kind}
			if worked, err := s.TickBounded(context.Background(), runner, nil, 6); err != nil || !worked {
				t.Fatalf("first tick: %v %v", worked, err)
			}
			if worked, err := s.TickBounded(context.Background(), runner, nil, 6); err != nil || worked {
				t.Fatalf("dispatched after failure: %v %v", worked, err)
			}
			if len(s.Snapshot().Attempts) != 1 {
				t.Fatal("extra attempt after failure")
			}
		})
	}
}

type boundedVerdictRunner struct{ fakeRunner }

func (r *boundedVerdictRunner) Run(ctx context.Context, d Dispatch) (RunResult, error) {
	result, err := r.fakeRunner.Run(ctx, d)
	result.Output = []byte(`{"verdict":"needs_human"}`)
	return result, err
}
func TestBoundedRunStopsOnHumanVerdict(t *testing.T) {
	s := testStore(t)
	for _, title := range []string{"first", "second"} {
		if _, _, err := s.CreatePlannedTask("bounded", App, "analysis", title, "v1"); err != nil {
			t.Fatal(err)
		}
	}
	runner := &boundedVerdictRunner{}
	if worked, err := s.TickBounded(context.Background(), runner, nil, 6); err != nil || !worked {
		t.Fatalf("first %v %v", worked, err)
	}
	if worked, err := s.TickBounded(context.Background(), runner, nil, 6); err != nil || worked {
		t.Fatalf("ignored verdict %v %v", worked, err)
	}
	if len(s.Snapshot().Attempts) != 1 {
		t.Fatal("extra claim after human verdict")
	}
}

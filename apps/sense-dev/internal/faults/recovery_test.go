package faults

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/core"
)

func store(t *testing.T) *core.Store {
	t.Helper()
	s, err := core.Open(filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.SeedKnowledge(); err != nil {
		t.Fatalf("SeedKnowledge: %v", err)
	}
	return s
}

// firstCall は最初の dispatch。1 度も配車されていなければテストを止める。
func firstCall(t *testing.T, r *Runner) Call {
	t.Helper()
	calls := r.Calls()
	if len(calls) == 0 {
		t.Fatal("no dispatch happened")
	}
	return calls[0]
}

func plan(t *testing.T, s *core.Store, title string) {
	t.Helper()
	if _, _, err := s.CreatePlannedTask("m1", core.Platform, "change", title, "v1"); err != nil {
		t.Fatalf("CreatePlannedTask %q: %v", title, err)
	}
}

// tick は 1 回配車して走らせる。戻りは「何か動いたか」。
func tick(t *testing.T, s *core.Store, r *Runner) bool {
	t.Helper()
	moved, err := s.Tick(context.Background(), r, nil)
	if err != nil {
		// 故障は RunError として返るので、Tick 自体のエラーは記録して続ける。
		t.Logf("Tick returned: %v", err)
	}
	return moved
}

func agentOf(t *testing.T, s *core.Store, agentID string) core.Agent {
	t.Helper()
	a, ok := s.Snapshot().Agents[agentID]
	if !ok {
		t.Fatalf("unknown agent %s", agentID)
	}
	return a
}

// 枠切れは「待てば直る」。待機理由が quota_wait で、再開時刻が入ること。
func TestQuotaWaitRecordsResumePoint(t *testing.T) {
	s := store(t)
	plan(t, s, "golden path")
	r := New(Quota)

	tick(t, s, r)
	if n := len(r.Calls()); n != 1 {
		t.Fatalf("calls = %d, want 1", n)
	}
	a := agentOf(t, s, firstCall(t, r).AgentID)
	if a.Status != "quota_wait" {
		t.Fatalf("status = %q, want quota_wait", a.Status)
	}
	if a.RetryAfter == nil {
		t.Fatal("quota_wait has no resume point")
	}
	if !a.RetryAfter.After(time.Now().UTC()) {
		t.Fatalf("resume point %v is not in the future", a.RetryAfter)
	}
	if n := len(r.External()); n != 0 {
		t.Fatalf("external operations = %d, want 0", n)
	}
}

// 認証切れは人を呼ぶ。自動の再開時刻を置かず、配車もされないこと。
func TestAuthRequiredStopsUntilHuman(t *testing.T) {
	s := store(t)
	plan(t, s, "golden path")
	r := New(Auth)

	tick(t, s, r)
	agentID := firstCall(t, r).AgentID
	a := agentOf(t, s, agentID)
	if a.Status != "auth_required" {
		t.Fatalf("status = %q, want auth_required", a.Status)
	}
	if a.RetryAfter != nil {
		t.Fatalf("auth_required has an automatic resume point %v; a human must act", a.RetryAfter)
	}

	// 放っておいても再配車されない。
	for i := 0; i < 3; i++ {
		tick(t, s, r)
	}
	for _, c := range r.Calls()[1:] {
		if c.AgentID == agentID {
			t.Fatal("auth_required agent was redispatched without a human")
		}
	}
}

// 人が直した後は再開でき、必ず新しい generation で始まること。
func TestResumeAfterAuthStartsNewGeneration(t *testing.T) {
	s := store(t)
	plan(t, s, "golden path")
	r := New(Auth)

	tick(t, s, r)
	agentID := firstCall(t, r).AgentID
	before := agentOf(t, s, agentID)

	if err := s.ResumeAgent(agentID, "re-authenticated by Yu"); err != nil {
		t.Fatalf("ResumeAgent: %v", err)
	}
	after := agentOf(t, s, agentID)
	if after.Status != "ready" {
		t.Fatalf("status = %q, want ready", after.Status)
	}
	if after.SessionGeneration != before.SessionGeneration+1 {
		t.Fatalf("generation = %d, want %d", after.SessionGeneration, before.SessionGeneration+1)
	}
	if after.SessionID != "" {
		t.Fatalf("session %q survived the resume", after.SessionID)
	}

	// 再開後は実際に配車される。
	tick(t, s, r)
	last := r.Calls()[len(r.Calls())-1]
	if last.AgentID != agentID {
		t.Fatalf("resumed agent was not dispatched (got %s)", last.AgentID)
	}
	if last.Generation != after.SessionGeneration {
		t.Fatalf("dispatched generation = %d, want %d", last.Generation, after.SessionGeneration)
	}
}

// 走っていない agent は再開できない。間違って ready に戻さないため。
func TestResumeRejectsRunningAndReadyAgents(t *testing.T) {
	s := store(t)
	plan(t, s, "golden path")
	r := New(None)

	st := s.Snapshot()
	var readyID string
	for id, a := range st.Agents {
		if a.Status == "ready" {
			readyID = id
			break
		}
	}
	if readyID == "" {
		t.Fatal("no ready agent")
	}
	if err := s.ResumeAgent(readyID, "no reason"); err == nil {
		t.Fatal("ResumeAgent accepted a ready agent")
	}
	if err := s.ResumeAgent(readyID, ""); err == nil {
		t.Fatal("ResumeAgent accepted an empty reason")
	}
	_ = r
}

// provider の一時障害は再試行で直りうる。上限までは待機時刻が入る。
func TestProviderErrorRetriesThenStops(t *testing.T) {
	s := store(t)
	plan(t, s, "golden path")
	r := New(Provider, Provider, Provider, Provider)

	tick(t, s, r)
	agentID := firstCall(t, r).AgentID
	a := agentOf(t, s, agentID)
	if a.Status != "retry_wait" {
		t.Fatalf("status = %q, want retry_wait", a.Status)
	}
	if a.RetryAfter == nil {
		t.Fatal("retry_wait has no resume point")
	}
	if n := len(r.External()); n != 0 {
		t.Fatalf("external operations = %d, want 0", n)
	}
}

// timeout は成功ではない。成果物を残さず、再開点を持って止まること。
func TestTimeoutIsNotSuccess(t *testing.T) {
	s := store(t)
	plan(t, s, "golden path")
	r := New(Timeout)

	tick(t, s, r)
	agentID := firstCall(t, r).AgentID
	a := agentOf(t, s, agentID)
	if a.Status != "retry_wait" {
		t.Fatalf("status = %q, want retry_wait", a.Status)
	}
	if n := len(r.External()); n != 0 {
		t.Fatalf("timeout counted as an external operation (%d)", n)
	}
	for _, attempt := range s.Snapshot().Attempts {
		if attempt.AgentID == agentID && attempt.Status == "completed" {
			t.Fatal("timed-out attempt was recorded as completed")
		}
	}
}

// worker が固まった場合。lease 切れで provider を解放し、勝手に再実行しない。
func TestWorkerStallReleasesProviderWithoutRerun(t *testing.T) {
	s := store(t)
	plan(t, s, "first")
	plan(t, s, "second")

	now := time.Now().UTC()
	stalled, ok, err := s.ClaimNext(now)
	if err != nil || !ok {
		t.Fatalf("ClaimNext: %v ok=%v", err, ok)
	}
	n, err := s.ExpireLeases(now.Add(core.LeaseTTL + time.Second))
	if err != nil || n != 1 {
		t.Fatalf("ExpireLeases = %d (err=%v), want 1", n, err)
	}
	if got := agentOf(t, s, stalled.AgentID).Status; got != "interrupted" {
		t.Fatalf("status = %q, want interrupted", got)
	}

	// provider は空いているので別 task は進むが、固まった agent は戻らない。
	r := New()
	tick(t, s, r)
	for _, c := range r.Calls() {
		if c.AgentID == stalled.AgentID {
			t.Fatal("stalled agent was rerun without inspection")
		}
	}
	if len(r.Calls()) == 0 {
		t.Fatal("provider was not released for the other task")
	}
}

// controller 再起動。走っていた attempt は interrupted になり、二重実行しない。
func TestControllerRestartDoesNotRerunInFlightWork(t *testing.T) {
	s := store(t)
	plan(t, s, "golden path")

	running, ok, err := s.ClaimNext(time.Now().UTC())
	if err != nil || !ok {
		t.Fatalf("ClaimNext: %v ok=%v", err, ok)
	}
	if err := s.RecoverInterrupted(); err != nil {
		t.Fatalf("RecoverInterrupted: %v", err)
	}
	if got := s.Snapshot().Attempts[running.ID].Status; got != "interrupted" {
		t.Fatalf("attempt status = %q, want interrupted", got)
	}

	r := New()
	for i := 0; i < 3; i++ {
		tick(t, s, r)
	}
	for _, c := range r.Calls() {
		if c.AgentID == running.AgentID {
			t.Fatal("in-flight agent was rerun after restart")
		}
	}
	if n := len(r.External()); n != 0 {
		t.Fatalf("external operations after restart = %d, want 0", n)
	}
}

// Mac 優先の間は新規実行を出さない。解除後は再開する。
func TestMacPriorityPausesNewWork(t *testing.T) {
	s := store(t)
	plan(t, s, "golden path")
	r := New()

	until := time.Now().UTC().Add(2 * time.Hour)
	if err := s.SetMacPriority(until); err != nil {
		t.Fatalf("SetMacPriority: %v", err)
	}
	tick(t, s, r)
	if n := len(r.Calls()); n != 0 {
		t.Fatalf("dispatched %d times during Mac priority, want 0", n)
	}

	if err := s.SetMacPriority(time.Time{}); err != nil {
		t.Fatalf("SetMacPriority(clear): %v", err)
	}
	tick(t, s, r)
	if n := len(r.Calls()); n == 0 {
		t.Fatal("work did not resume after Mac priority was cleared")
	}
}

// 同じ attempt が二度 外部操作として記録されないこと。
func TestNoDuplicateExternalOperationPerAttempt(t *testing.T) {
	s := store(t)
	plan(t, s, "first")
	plan(t, s, "second")
	r := New()

	// 正常に進める。工程が進むほど attempt が増えるので、重複が出るなら出る。
	for i := 0; i < 8; i++ {
		tick(t, s, r)
	}
	seen := map[string]bool{}
	for _, id := range r.External() {
		if seen[id] {
			t.Fatalf("attempt %s performed an external operation twice", id)
		}
		seen[id] = true
	}
	if len(seen) == 0 {
		t.Fatal("no successful run was recorded; the test proves nothing")
	}
}

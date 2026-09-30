package core

import (
	"path/filepath"
	"testing"
	"time"
)

func leaseStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// 配車した時点で lease が入ること。入っていないと期限切れを判定できない。
func TestClaimSetsLease(t *testing.T) {
	s := leaseStore(t)
	if _, _, err := s.CreatePlannedTask("m1", Platform, "change", "golden path", "v1"); err != nil {
		t.Fatalf("CreatePlannedTask: %v", err)
	}
	now := time.Now().UTC()
	attempt, ok, err := s.ClaimNext(now)
	if err != nil || !ok {
		t.Fatalf("ClaimNext: %v ok=%v", err, ok)
	}
	if attempt.LeaseExpiresAt == nil {
		t.Fatal("claimed attempt has no lease")
	}
	if got := attempt.LeaseExpiresAt.Sub(now); got != LeaseTTL {
		t.Fatalf("lease = %v, want %v", got, LeaseTTL)
	}
}

// 固まった worker が provider を握り続けないこと。これが lease の目的。
//
// 同じ provider を使う独立した task を 2 本置く。1 本目が lease を握っている間は
// 2 本目が配車されず、期限切れで解放されたら配車される、という形で確かめる。
func TestExpiredLeaseReleasesProvider(t *testing.T) {
	s := leaseStore(t)
	for _, title := range []string{"golden path", "second change"} {
		if _, _, err := s.CreatePlannedTask("m1", Platform, "change", title, "v1"); err != nil {
			t.Fatalf("CreatePlannedTask %q: %v", title, err)
		}
	}
	now := time.Now().UTC()
	first, ok, err := s.ClaimNext(now)
	if err != nil || !ok {
		t.Fatalf("ClaimNext: %v ok=%v", err, ok)
	}

	// lease 内は同じ provider を掴めない（provider ごと最大 1 実行）。
	if _, ok, err := s.ClaimNext(now.Add(time.Minute)); err != nil || ok {
		t.Fatalf("claimed while provider busy: ok=%v err=%v", ok, err)
	}

	after := now.Add(LeaseTTL + time.Second)
	n, err := s.ExpireLeases(after)
	if err != nil {
		t.Fatalf("ExpireLeases: %v", err)
	}
	if n != 1 {
		t.Fatalf("expired %d attempts, want 1", n)
	}

	st := s.Snapshot()
	if got := st.Attempts[first.ID].Status; got != "interrupted" {
		t.Fatalf("attempt status = %q, want interrupted", got)
	}
	if got := st.Agents[first.AgentID].Status; got != "interrupted" {
		t.Fatalf("agent status = %q, want interrupted", got)
	}

	// 解放された provider で、もう 1 本の task が配車されること。
	next, ok, err := s.ClaimNext(after)
	if err != nil {
		t.Fatalf("ClaimNext after expiry: %v", err)
	}
	if !ok {
		t.Fatal("no attempt claimed after lease expiry")
	}
	if next.Provider != first.Provider {
		t.Fatalf("claimed provider = %q, want %q", next.Provider, first.Provider)
	}
	if next.TaskID == first.TaskID {
		t.Fatal("claimed the interrupted task again")
	}
}

// 期限切れは自動で再開しない。同じ外部操作を二重に走らせないため。
// 配車できる相手が他にいる状況で、interrupted の agent が選ばれないことを見る。
func TestExpiredLeaseDoesNotRequeueSameAgent(t *testing.T) {
	s := leaseStore(t)
	for _, title := range []string{"golden path", "second change"} {
		if _, _, err := s.CreatePlannedTask("m1", Platform, "change", title, "v1"); err != nil {
			t.Fatalf("CreatePlannedTask %q: %v", title, err)
		}
	}
	now := time.Now().UTC()
	first, _, err := s.ClaimNext(now)
	if err != nil {
		t.Fatalf("ClaimNext: %v", err)
	}
	if _, err := s.ExpireLeases(now.Add(LeaseTTL + time.Second)); err != nil {
		t.Fatalf("ExpireLeases: %v", err)
	}
	for i := 0; i < 3; i++ {
		next, ok, err := s.ClaimNext(now.Add(LeaseTTL + time.Duration(2+i)*time.Second))
		if err != nil {
			t.Fatalf("ClaimNext: %v", err)
		}
		if ok && next.AgentID == first.AgentID {
			t.Fatal("interrupted agent was redispatched without inspection")
		}
		if !ok {
			break
		}
	}
}

// 生きている worker は lease を伸ばせる。伸ばした後は期限切れにならない。
func TestRenewLeaseKeepsAttemptRunning(t *testing.T) {
	s := leaseStore(t)
	if _, _, err := s.CreatePlannedTask("m1", Platform, "change", "golden path", "v1"); err != nil {
		t.Fatalf("CreatePlannedTask: %v", err)
	}
	now := time.Now().UTC()
	attempt, _, err := s.ClaimNext(now)
	if err != nil {
		t.Fatalf("ClaimNext: %v", err)
	}

	renewAt := now.Add(LeaseTTL - time.Minute)
	if err := s.RenewLease(attempt.ID, renewAt); err != nil {
		t.Fatalf("RenewLease: %v", err)
	}
	if n, err := s.ExpireLeases(now.Add(LeaseTTL + time.Second)); err != nil || n != 0 {
		t.Fatalf("expired %d attempts after renew (err=%v), want 0", n, err)
	}
	if got := s.Snapshot().Attempts[attempt.ID].Status; got != "running" {
		t.Fatalf("attempt status = %q, want running", got)
	}
}

// running でない attempt の lease は伸ばせない。
func TestRenewLeaseRejectsFinishedAttempt(t *testing.T) {
	s := leaseStore(t)
	if _, _, err := s.CreatePlannedTask("m1", Platform, "change", "golden path", "v1"); err != nil {
		t.Fatalf("CreatePlannedTask: %v", err)
	}
	now := time.Now().UTC()
	attempt, _, err := s.ClaimNext(now)
	if err != nil {
		t.Fatalf("ClaimNext: %v", err)
	}
	if err := s.FailAttempt(attempt.ID, "auth_required", "needs login", time.Time{}); err != nil {
		t.Fatalf("FailAttempt: %v", err)
	}
	if err := s.RenewLease(attempt.ID, now.Add(time.Minute)); err == nil {
		t.Fatal("RenewLease succeeded on a finished attempt")
	}
}

// attempt 上限を超えた agent は配車されない。無限 retry を作らないため。
func TestAttemptLimitStopsDispatch(t *testing.T) {
	s := leaseStore(t)
	if _, _, err := s.CreatePlannedTask("m1", Platform, "change", "golden path", "v1"); err != nil {
		t.Fatalf("CreatePlannedTask: %v", err)
	}
	now := time.Now().UTC()
	var agentID string
	for i := 0; i < MaxAttempts; i++ {
		attempt, ok, err := s.ClaimNext(now)
		if err != nil || !ok {
			t.Fatalf("ClaimNext %d: %v ok=%v", i, err, ok)
		}
		agentID = attempt.AgentID
		if err := s.FailAttempt(attempt.ID, "retry_wait", "boom", now); err != nil {
			t.Fatalf("FailAttempt %d: %v", i, err)
		}
	}
	if got := s.Snapshot().Agents[agentID].AttemptCount; got != MaxAttempts {
		t.Fatalf("AttemptCount = %d, want %d", got, MaxAttempts)
	}
	next, ok, err := s.ClaimNext(now.Add(time.Hour))
	if err != nil {
		t.Fatalf("ClaimNext: %v", err)
	}
	if ok && next.AgentID == agentID {
		t.Fatalf("agent dispatched past the attempt limit (%d)", MaxAttempts)
	}
}

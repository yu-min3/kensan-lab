package jobs

import (
	"errors"
	"testing"
)

func TestDefinitiveRejectionPreservesScopeAndReplay(t *testing.T) {
	s, dir := openTest(t)
	a := admission("blocked")
	r, _, e := s.Admit(a, 1)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.RejectDispatch(a.Owner, r.ID, "failed", "runtime_request_rejected"); !errors.Is(e, ErrInvalid) {
		t.Fatal("undispatched job rejected")
	}
	s.BeginDispatch(a.Owner, r.ID)
	other := a.Owner
	other.App = "other"
	if _, e = s.RejectDispatch(other, r.ID, "failed", "runtime_request_rejected"); !errors.Is(e, ErrNotFound) {
		t.Fatal("foreign owner altered state")
	}
	waiting, e := s.RejectDispatch(a.Owner, r.ID, "waiting", "runtime_auth_required")
	if e != nil || waiting.Phase != "prepared" {
		t.Fatal(waiting, e)
	}
	again, reused, e := s.Admit(a, 0)
	if e != nil || !reused || again.ID != r.ID {
		t.Fatal("replay reserved another slot", e)
	}
	s.BeginDispatch(a.Owner, r.ID)
	if _, e = s.RejectDispatch(a.Owner, r.ID, "failed", "runtime_request_rejected"); e != nil {
		t.Fatal(e)
	}
	s.Close()
	restored, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer restored.Close()
	failed, e := restored.Get(a.Owner, r.ID)
	if e != nil || failed.State != "failed" || failed.Reason != "runtime_request_rejected" {
		t.Fatal("rejection lost across restart", failed, e)
	}
}

func TestAuthWaitSurvivesRestartAndCanCancelBeforeAcceptance(t *testing.T) {
	s, dir := openTest(t)
	a := admission("auth-wait")
	r, _, _ := s.Admit(a, 1)
	s.BeginDispatch(a.Owner, r.ID)
	s.RejectDispatch(a.Owner, r.ID, "waiting", "runtime_auth_required")
	s.Close()
	reopened, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	r, e = reopened.Get(a.Owner, r.ID)
	if e != nil || r.State != "waiting" || r.Phase != "prepared" {
		t.Fatal("auth wait recovery", r, e)
	}
	r, e = reopened.CancelPrepared(a.Owner, r.ID)
	if e != nil || r.State != "cancelled" {
		t.Fatal("non-admitted auth wait cancellation", r, e)
	}
}

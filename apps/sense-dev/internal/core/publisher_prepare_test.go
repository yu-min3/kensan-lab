package core

import (
	"context"
	"errors"
	"testing"
	"time"
)

type preparationTransport struct {
	prepareCalls, executeCalls int
	prepare                    func() error
	executeError               bool
}

func (p *preparationTransport) Inspect(context.Context, PublishIntent) (string, bool, error) {
	return "", false, nil
}
func (p *preparationTransport) Prepare(context.Context, PublishIntent) error {
	p.prepareCalls++
	if p.prepare != nil {
		return p.prepare()
	}
	return nil
}
func (p *preparationTransport) Execute(context.Context, PublishIntent) (string, error) {
	p.executeCalls++
	if p.executeError {
		return "", errors.New("result lost")
	}
	return "remote-result", nil
}
func preparePublishFixture(t *testing.T) (*Store, ReleaseFlow) {
	t.Helper()
	d, _ := driverFixture(t, "apps/canary/main.go")
	reconcileDriver(t, d)
	completeDriverGate(t, d, soleFlow(t, d), "allow")
	reconcileDriver(t, d)
	return d.Store, soleFlow(t, d)
}
func TestPublishPreparationFailureStaysPending(t *testing.T) {
	s, f := preparePublishFixture(t)
	p := &preparationTransport{prepare: func() error { return errors.New("bundle unavailable") }}
	if _, err := s.RunPublish(context.Background(), f.DecisionID, p); err == nil {
		t.Fatal("preparation failure ignored")
	}
	if s.Snapshot().Intents[f.IntentID].Status != "pending_reconcile" || p.executeCalls != 0 {
		t.Fatal("failed local transfer became ambiguous or published")
	}
	p.prepare = nil
	if _, err := s.RunPublish(context.Background(), f.DecisionID, p); err != nil {
		t.Fatal(err)
	}
	if p.prepareCalls != 2 || p.executeCalls != 1 {
		t.Fatal("pending preparation not retryable")
	}
}
func TestPublishRechecksExpiryAfterPreparation(t *testing.T) {
	s, f := preparePublishFixture(t)
	p := &preparationTransport{prepare: func() error {
		return s.update(func(st *State) error {
			d := st.Decisions[f.DecisionID]
			d.ExpiresAt = time.Now().Add(-time.Minute)
			st.Decisions[d.ID] = d
			return nil
		})
	}}
	if _, err := s.RunPublish(context.Background(), f.DecisionID, p); err == nil {
		t.Fatal("expired authorization executed")
	}
	if p.prepareCalls != 1 || p.executeCalls != 0 || s.Snapshot().Intents[f.IntentID].Status != "pending_reconcile" {
		t.Fatal("preparation bypassed final authorization")
	}
}
func TestUnknownPublishDoesNotPrepareOrExecuteAgain(t *testing.T) {
	s, f := preparePublishFixture(t)
	p := &preparationTransport{executeError: true}
	if _, err := s.RunPublish(context.Background(), f.DecisionID, p); err == nil {
		t.Fatal("ambiguous result ignored")
	}
	if _, err := s.RunPublish(context.Background(), f.DecisionID, p); err == nil {
		t.Fatal("unknown attempt retried")
	}
	if p.prepareCalls != 1 || p.executeCalls != 1 {
		t.Fatal("unknown result repeated side effects")
	}
}

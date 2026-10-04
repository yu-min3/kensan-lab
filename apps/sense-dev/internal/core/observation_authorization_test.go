package core

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestDelayedDeploymentObservationKeepsTimelyAuthorization(t *testing.T) {
	s := testStore(t)
	task, _, err := s.CreatePlannedTask("mission", App, "change", "feature", "v1")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.update(func(st *State) error {
		v := st.Tasks[task.ID]
		v.Status = "publish_wait"
		v.HeadSHA = strings.Repeat("a", 40)
		st.Tasks[v.ID] = v
		task = v
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	seedAppDeployment(t, s, task)
	r := s.Snapshot().Deployments[task.ID]
	if err := s.update(func(st *State) error {
		delete(st.Deployments, task.ID)
		d := st.Decisions[r.DecisionID]
		d.ExpiresAt = time.Now().Add(-time.Minute)
		st.Decisions[d.ID] = d
		i := st.Intents[r.IntentID]
		i.ExpiresAt = d.ExpiresAt
		i.AuthorizedAt = d.ExpiresAt.Add(-time.Minute)
		st.Intents[i.ID] = i
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordDeploymentReceipt(r); err != nil {
		t.Fatalf("delayed authorized observation rejected: %v", err)
	}
	if !appDeploymentReady(s.Snapshot(), task) {
		t.Fatal("delayed receipt did not admit acceptance")
	}
	baseline := s.Snapshot()
	for name, mutate := range map[string]func(*State){
		"zero": func(st *State) { i := st.Intents[r.IntentID]; i.AuthorizedAt = time.Time{}; st.Intents[i.ID] = i },
		"future": func(st *State) {
			i := st.Intents[r.IntentID]
			i.AuthorizedAt = time.Now().Add(time.Hour)
			i.ExpiresAt = i.AuthorizedAt.Add(time.Hour)
			st.Intents[i.ID] = i
			d := st.Decisions[r.DecisionID]
			d.ExpiresAt = i.ExpiresAt
			st.Decisions[d.ID] = d
		},
		"expiry boundary":   func(st *State) { i := st.Intents[r.IntentID]; i.AuthorizedAt = i.ExpiresAt; st.Intents[i.ID] = i },
		"changed ref":       func(st *State) { i := st.Intents[r.IntentID]; i.Ref = "refs/heads/other"; st.Intents[i.ID] = i },
		"changed operation": func(st *State) { i := st.Intents[r.IntentID]; i.Operation = "branch_push"; st.Intents[i.ID] = i },
		"changed target":    func(st *State) { i := st.Intents[r.IntentID]; i.TargetEnvironment = "public"; st.Intents[i.ID] = i },
		"changed task":      func(st *State) { v := st.Tasks[task.ID]; v.HeadSHA = strings.Repeat("b", 40); st.Tasks[v.ID] = v },
	} {
		t.Run(name, func(t *testing.T) {
			if err := s.update(func(st *State) error {
				*st = baseline
				st.Intents = map[string]PublishIntent{r.IntentID: baseline.Intents[r.IntentID]}
				st.Decisions = map[string]ReleaseDecision{r.DecisionID: baseline.Decisions[r.DecisionID]}
				st.Tasks = map[string]Task{task.ID: task}
				delete(st.Deployments, task.ID)
				mutate(st)
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if s.RecordDeploymentReceipt(r) == nil || appDeploymentReady(s.Snapshot(), s.Snapshot().Tasks[task.ID]) {
				t.Fatal("invalid historical authorization admitted")
			}
		})
	}
}

func TestUnknownReconciliationAfterExpiryNeverReauthorizesOrExecutes(t *testing.T) {
	s, origin, candidate, request := classifiedReleaseFixture(t, "image_publish")
	if _, err := s.DecideApproval(request.ID, "tap", origin.Operation, origin.HeadSHA, "approved"); err != nil {
		t.Fatal(err)
	}
	_, gate := taskAgent(t, s, Platform, "release_gate")
	if err := s.BindReleaseGateInputs(gate.ID, origin.AuthorAgentID, origin.ArtifactRefs, origin.ScanRef, candidate); err != nil {
		t.Fatal(err)
	}
	d := gateFixtureDecision(t, s, gate.ID, origin.AuthorAgentID, origin.ArtifactRefs[0], origin.ScanRef, "allow", "independent verified image inputs")
	d.ApprovalID = request.ID
	d, err := s.RecordReleaseDecision(d)
	if err != nil {
		t.Fatal(err)
	}
	transport := &unknownImageTransport{}
	if _, err := s.RunPublish(context.Background(), d.ID, transport); err == nil {
		t.Fatal("ambiguous dispatch accepted")
	}
	var initial PublishIntent
	for _, i := range s.Snapshot().Intents {
		if i.DecisionID == d.ID {
			initial = i
		}
	}
	if initial.AuthorizedAt.IsZero() {
		t.Fatal("mutation authorization not recorded")
	}
	if err := s.update(func(st *State) error {
		i := st.Intents[initial.ID]
		i.ExpiresAt = i.AuthorizedAt.Add(time.Nanosecond)
		st.Intents[i.ID] = i
		v := st.Decisions[d.ID]
		v.ExpiresAt = i.ExpiresAt
		st.Decisions[v.ID] = v
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	driver := &ReleaseDriver{Store: s, Plan: ReleasePlan{MissionID: s.Snapshot().Tasks[s.Snapshot().Agents[d.AuthorAgentID].TaskID].MissionID}}
	if driver.ReconcileUnknown(context.Background(), transport) == nil || transport.executions != 1 {
		t.Fatal("unknown absence retried")
	}
	transport.accepted = true
	if err := driver.ReconcileUnknown(context.Background(), transport); err != nil {
		t.Fatal(err)
	}
	result := s.Snapshot().Intents[initial.ID]
	if result.Status != "sent" || !result.AuthorizedAt.Equal(initial.AuthorizedAt) || transport.executions != 1 {
		t.Fatal("expired reconciliation changed authorization or executed")
	}
	if err := s.update(func(st *State) error {
		i := st.Intents[initial.ID]
		i.Status = "unknown"
		i.AuthorizedAt = time.Time{}
		st.Intents[i.ID] = i
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if driver.ReconcileUnknown(context.Background(), transport) == nil || !s.Snapshot().Intents[initial.ID].AuthorizedAt.IsZero() || transport.executions != 1 {
		t.Fatal("unknown result gained new authorization")
	}

}

type expiringSuccessTransport struct{ executions int }

func (*expiringSuccessTransport) Inspect(context.Context, PublishIntent) (string, bool, error) {
	return "", false, nil
}
func (f *expiringSuccessTransport) Execute(_ context.Context, i PublishIntent) (string, error) {
	f.executions++
	time.Sleep(time.Until(i.ExpiresAt) + 10*time.Millisecond)
	return "exact-remote-result", nil
}
func TestMutationCompletingAfterExpiryKeepsOriginalAuthorization(t *testing.T) {
	s, flow := preparePublishFixture(t)
	if err := s.update(func(st *State) error {
		d := st.Decisions[flow.DecisionID]
		d.ExpiresAt = time.Now().Add(300 * time.Millisecond)
		st.Decisions[d.ID] = d
		i := st.Intents[flow.IntentID]
		i.ExpiresAt = d.ExpiresAt
		st.Intents[i.ID] = i
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	transport := &expiringSuccessTransport{}
	result, err := s.RunPublish(context.Background(), flow.DecisionID, transport)
	if err != nil || result.Status != "sent" || !result.AuthorizedAt.Before(result.ExpiresAt) || time.Now().Before(result.ExpiresAt) || transport.executions != 1 {
		t.Fatalf("delayed completion %+v %v", result, err)
	}
}

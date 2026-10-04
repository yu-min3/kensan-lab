package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func driverFixture(t *testing.T, path string) (*ReleaseDriver, Task) {
	t.Helper()
	root := t.TempDir()
	repo := filepath.Join(root, "source")
	if err := os.Mkdir(repo, 0700); err != nil {
		t.Fatal(err)
	}
	gitTest(t, repo, "init", "-q")
	gitTest(t, repo, "config", "user.name", "Test")
	gitTest(t, repo, "config", "user.email", "test@example.invalid")
	base := commitTestFile(t, repo, "README.md", "base\n", "base")
	head := commitTestFile(t, repo, path, "safe change\n", "change")
	s, task, _ := readyAppChange(t, releaseReadyRunner{base: base, head: head})
	gitTest(t, root, "clone", "-q", repo, task.ID)
	return &ReleaseDriver{Store: s, WorktreeRoot: root, Plan: ReleasePlan{SchemaVersion: 1, MissionID: task.MissionID, Repository: "yu-min3/kensan-lab", TargetEnvironment: "private-canary", Impact: "private canary application", Rollback: "revert the reviewed commit", PullRequestSummary: "canary change"}}, task
}

func reconcileDriver(t *testing.T, d *ReleaseDriver) {
	t.Helper()
	if _, err := d.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
}
func soleFlow(t *testing.T, d *ReleaseDriver) ReleaseFlow {
	t.Helper()
	if len(d.Store.Snapshot().ReleaseFlows) != 1 {
		t.Fatal("expected one flow")
	}
	for _, f := range d.Store.Snapshot().ReleaseFlows {
		return f
	}
	panic("unreachable")
}
func completeDriverGate(t *testing.T, d *ReleaseDriver, f ReleaseFlow, verdict string) {
	t.Helper()
	gate := d.Store.Snapshot().Agents[f.GateAgentID]
	gateFixtureDecision(t, d.Store, gate.ID, f.AuthorAgentID, gate.ReviewInputs[0], f.ScanRef, verdict, "fixed inputs reviewed")
	reconcileDriver(t, d)
}

type driverTransport struct{ operations []string }

func (x *driverTransport) Inspect(context.Context, PublishIntent) (string, bool, error) {
	return "", false, nil
}
func (x *driverTransport) Execute(_ context.Context, i PublishIntent) (string, error) {
	x.operations = append(x.operations, i.Operation)
	return "external-" + i.Operation, nil
}

func TestReleaseDriverOrderedIndependentGatesAndRestart(t *testing.T) {
	d, task := driverFixture(t, "apps/canary/main.go")
	reconcileDriver(t, d)
	first := soleFlow(t, d)
	if first.Status != "gate_wait" || first.Operation != "branch_push" || first.AuthorTaskID != task.ID {
		t.Fatalf("flow %+v", first)
	}
	gate := d.Store.Snapshot().Agents[first.GateAgentID]
	if gate.Team != Platform || gate.ID == first.AuthorAgentID || d.Store.Snapshot().Tasks[gate.TaskID].SourceTaskID != task.ID {
		t.Fatal("Gate not independent and linked")
	}
	restarted := &ReleaseDriver{Store: d.Store, Plan: d.Plan, WorktreeRoot: d.WorktreeRoot}
	var wg sync.WaitGroup
	for _, driver := range []*ReleaseDriver{d, restarted} {
		wg.Add(1)
		go func(driver *ReleaseDriver) {
			defer wg.Done()
			if _, err := driver.Reconcile(context.Background()); err != nil {
				t.Error(err)
			}
		}(driver)
	}
	wg.Wait()
	if soleFlow(t, d).GateAgentID != first.GateAgentID {
		t.Fatal("restart duplicated Gate")
	}
	transport := &driverTransport{}
	for _, operation := range []string{"branch_push", "pr_create", "merge"} {
		var flow ReleaseFlow
		for _, f := range d.Store.Snapshot().ReleaseFlows {
			if f.Operation == operation {
				flow = f
			}
		}
		if flow.ID == "" {
			t.Fatal("missing operation", operation)
		}
		completeDriverGate(t, d, flow, "allow")
		reconcileDriver(t, d)
		if err := d.PublishReady(context.Background(), transport); err != nil {
			t.Fatal(err)
		}
		reconcileDriver(t, d)
	}
	if len(transport.operations) != 3 {
		t.Fatalf("operations %v", transport.operations)
	}
	for i, op := range []string{"branch_push", "pr_create", "merge"} {
		if transport.operations[i] != op {
			t.Fatal(transport.operations)
		}
	}
}

func TestReleaseDriverExpiredGateAndPlanChange(t *testing.T) {
	d, _ := driverFixture(t, "apps/canary/main.go")
	reconcileDriver(t, d)
	old := soleFlow(t, d)
	if err := d.Store.update(func(st *State) error {
		f := st.ReleaseFlows[old.ID]
		f.GateExpiresAt = time.Now().Add(-time.Minute)
		st.ReleaseFlows[f.ID] = f
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	reconcileDriver(t, d)
	fresh := soleFlow(t, d)
	if fresh.GateGeneration != 2 || fresh.GateAgentID == old.GateAgentID {
		t.Fatal("expired Gate was reused")
	}
	d.Plan.Rollback = "different operator rollback"
	reconcileDriver(t, d)
	if d.Store.Snapshot().ReleaseFlows[old.ID].Status != "superseded" || d.Store.Snapshot().Agents[fresh.GateAgentID].Status != "superseded" {
		t.Fatal("old plan remains dispatchable")
	}
}

func TestReleaseDriverOwnershipDenyNeverDispatchesGate(t *testing.T) {
	d, _ := driverFixture(t, "apps/sense-dev/cmd/main.go")
	reconcileDriver(t, d)
	f := soleFlow(t, d)
	if f.Status != "denied" || f.GateAgentID != "" || len(d.Store.Snapshot().Intents) != 0 {
		t.Fatalf("unsafe flow %+v", f)
	}
}

func TestLoadReleasePlanRejectsMutableAndWorkerFields(t *testing.T) {
	d, _ := driverFixture(t, "apps/canary/main.go")
	path := filepath.Join(t.TempDir(), "release.json")
	body, _ := json.Marshal(d.Plan)
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadReleasePlan(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0666); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadReleasePlan(path); err == nil {
		t.Fatal("mutable plan allowed")
	}
	os.Chmod(path, 0600)
	body = append(body[:len(body)-1], []byte(`,"verdict":"allow"}`)...)
	os.WriteFile(path, body, 0600)
	if _, err := LoadReleasePlan(path); err == nil {
		t.Fatal("worker authorization fields allowed")
	}
}

func TestReleaseDriverHumanApprovalRequiresFreshGateAndPinnedEvidence(t *testing.T) {
	d, _ := driverFixture(t, "apps/canary/main.go")
	d.Plan.Impact = "authentication policy modification"
	reconcileDriver(t, d)
	old := soleFlow(t, d)
	completeDriverGate(t, d, old, "needs_human")
	awaiting := soleFlow(t, d)
	if awaiting.Status != "needs_human" || awaiting.IntentID != "" {
		t.Fatalf("flow %+v", awaiting)
	}
	reconcileDriver(t, d)
	if soleFlow(t, d).GateAgentID != old.GateAgentID {
		t.Fatal("unapproved Gate advanced")
	}
	if _, err := d.Store.DecideApproval(awaiting.ApprovalID, "exact-tap", awaiting.Operation, awaiting.HeadSHA, "approved"); err != nil {
		t.Fatal(err)
	}
	reconcileDriver(t, d)
	fresh := soleFlow(t, d)
	if fresh.GateAgentID == old.GateAgentID || fresh.ScanRef != old.ScanRef || fresh.IntentID != "" {
		t.Fatal("approval bypassed fresh Gate or lost exact scan")
	}
	reconcileDriver(t, d)
	if len(d.Store.Snapshot().Agents[fresh.GateAgentID].ReviewInputs) != 6 {
		t.Fatal("fresh Gate did not receive approval")
	}
	completeDriverGate(t, d, fresh, "allow")
	reconcileDriver(t, d)
	if soleFlow(t, d).IntentID == "" {
		t.Fatal("fresh approved Gate did not prepare intent")
	}
}

func TestReleaseDriverExpiredScanCreatesNewEvidence(t *testing.T) {
	d, _ := driverFixture(t, "apps/canary/main.go")
	reconcileDriver(t, d)
	old := soleFlow(t, d)
	body, err := d.Store.ReadArtifact(old.ScanRef.ID)
	if err != nil {
		t.Fatal(err)
	}
	var scan ReleaseScan
	if err := json.Unmarshal(body, &scan); err != nil {
		t.Fatal(err)
	}
	scan.ScannedAt = time.Now().Add(-2 * time.Hour)
	ref, err := d.Store.recordReleaseScan(scan)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Store.update(func(st *State) error {
		f := st.ReleaseFlows[old.ID]
		f.ScanRef = ref
		f.GateExpiresAt = time.Now().Add(-time.Minute)
		st.ReleaseFlows[f.ID] = f
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	reconcileDriver(t, d)
	fresh := soleFlow(t, d)
	if fresh.ScanRef == ref || fresh.GateAgentID == old.GateAgentID || fresh.ApprovalID != "" {
		t.Fatal("expired scan/approval reused")
	}
}

func TestReleaseDriverPlanChangeDoesNotRetryAmbiguousIntent(t *testing.T) {
	d, _ := driverFixture(t, "apps/canary/main.go")
	reconcileDriver(t, d)
	completeDriverGate(t, d, soleFlow(t, d), "allow")
	reconcileDriver(t, d)
	old := soleFlow(t, d)
	if err := d.Store.update(func(st *State) error {
		i := st.Intents[old.IntentID]
		i.Status = "unknown"
		st.Intents[i.ID] = i
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	d.Plan.Rollback = "new rollback"
	reconcileDriver(t, d)
	if len(d.Store.Snapshot().ReleaseFlows) != 1 {
		t.Fatal("plan change bypassed ambiguous previous operation")
	}
	x := &driverTransport{}
	if err := d.PublishReady(context.Background(), x); err == nil {
		t.Fatal("ambiguous missing remote result accepted")
	}
	if len(x.operations) != 0 {
		t.Fatal("ambiguous external operation retried")
	}
}

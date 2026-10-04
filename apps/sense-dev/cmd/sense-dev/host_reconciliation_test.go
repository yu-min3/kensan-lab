package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/committransfer"
	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/core"
	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/publisher"
)

type readonlyHostFixture struct{ inspected, observed, images, exported, executed int }

func (f *readonlyHostFixture) Inspect(_ context.Context, i core.PublishIntent) (string, bool, error) {
	f.inspected++
	return i.HeadSHA, true, nil
}
func (f *readonlyHostFixture) Execute(context.Context, core.PublishIntent) (string, error) {
	f.executed++
	return "", errors.New("unexpected mutation")
}
func (f *readonlyHostFixture) ImageEvidence(context.Context, core.PublishIntent) (publisher.ImageEvidence, error) {
	f.images++
	return publisher.ImageEvidence{}, errors.New("proof not yet available")
}
func (f *readonlyHostFixture) ExportMerged(context.Context, core.PublishIntent, string) (committransfer.Bundle, error) {
	f.exported++
	return committransfer.Bundle{}, errors.New("graph not yet available")
}
func (f *readonlyHostFixture) Observe(_ context.Context, i core.PublishIntent, task string) (core.DeploymentReceipt, error) {
	f.observed++
	return core.DeploymentReceipt{TaskID: task, DecisionID: i.DecisionID, IntentID: i.ID, HeadSHA: i.HeadSHA, Revision: i.ExternalID, ImageSourceSHA: i.ExternalID, ImageDigest: "sha256:" + strings.Repeat("c", 64), ObservedRelease: "v2", Environment: "private-canary", Status: "healthy", UserPath: "/api/release"}, nil
}

func TestHostReconciliationNeedsNoInferenceAdmissionAndNeverExecutes(t *testing.T) {
	state := core.NewState()
	deadline := time.Now().Add(-time.Minute)
	for _, kind := range []string{"sent", "unknown", "pending_reconcile", "image"} {
		operation, status := "branch_push", kind
		if kind == "sent" {
			operation = "merge"
		}
		if kind == "image" {
			operation, status = "image_publish", "sent"
		}
		head := strings.Repeat("a", 40)
		state.Tasks[kind] = core.Task{ID: kind, MissionID: "mission", Team: core.App, Kind: "change", Status: "publish_wait", HeadSHA: head}
		state.Agents[kind] = core.Agent{ID: kind, TaskID: kind, Team: core.App}
		d := core.ReleaseDecision{ID: kind, AuthorAgentID: kind, Verdict: "allow", Operation: operation, Repository: "yu-min3/kensan-lab", Ref: "refs/heads/sense-dev/" + kind, HeadSHA: head, TargetEnvironment: "private-canary", PolicyVersion: core.ReleasePolicyVersion, ExpiresAt: deadline}
		state.Decisions[kind] = d
		state.Intents[kind] = core.PublishIntent{ID: kind, DecisionID: kind, Operation: operation, Repository: d.Repository, Ref: d.Ref, HeadSHA: head, TargetEnvironment: d.TargetEnvironment, PolicyVersion: d.PolicyVersion, ExpiresAt: deadline, AuthorizedAt: deadline.Add(-time.Minute), Status: status, ExternalID: strings.Repeat("b", 40)}
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(state)
	if err := os.WriteFile(filepath.Join(dir, "state.json"), body, 0600); err != nil {
		t.Fatal(err)
	}
	store, err := core.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	driver := &core.ReleaseDriver{Store: store, Plan: core.ReleasePlan{MissionID: "mission"}}
	client := &readonlyHostFixture{}
	// The read-only entry point has no billing/window requirement or model runner.
	// One unavailable graph/image proof must not prevent the independent receipt.
	if err := reconcileHostObservations(context.Background(), store, driver, client, t.TempDir()); err == nil {
		t.Fatal("unavailable evidence ignored")
	}
	if client.executed != 0 || client.inspected != 1 || client.observed != 1 || client.images != 1 || client.exported != 1 {
		t.Fatalf("host operations %+v", client)
	}
	if store.Snapshot().Intents["pending_reconcile"].Status != "pending_reconcile" || store.Snapshot().Intents["unknown"].Status != "sent" || len(store.Snapshot().Deployments) != 1 || len(store.Snapshot().ImageReleases) != 0 {
		t.Fatal("read-only reconciliation changed unapproved work or lost delayed receipt")
	}
}

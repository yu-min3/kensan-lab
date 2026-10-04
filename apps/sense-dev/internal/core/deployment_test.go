package core

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestDeploymentReceiptRequiresActualApprovedDelivery(t *testing.T) {
	s := testStore(t)
	task, agents, err := s.CreatePlannedTask("app-development", App, "change", "feature", "app-v1")
	if err != nil {
		t.Fatal(err)
	}
	head := strings.Repeat("a", 40)
	if err := s.update(func(st *State) error {
		v := st.Tasks[task.ID]
		v.HeadSHA = head
		v.Status = "publish_wait"
		st.Tasks[task.ID] = v
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	r := DeploymentReceipt{ObservedRelease: "v2", TaskID: task.ID, DecisionID: "decision", IntentID: "intent", HeadSHA: head, Revision: strings.Repeat("e", 40), ImageSourceSHA: strings.Repeat("e", 40), ImageDigest: "sha256:" + strings.Repeat("b", 64), Environment: "private-canary", Status: "healthy", UserPath: "private-canary/health"}
	observation := func() {
		body, _ := json.Marshal(r)
		a, err := s.PutArtifact("system", "deployment_observation", body)
		if err != nil {
			t.Fatal(err)
		}
		r.EvidenceRef = artifactRef(a)
	}
	observation()
	if err := s.RecordDeploymentReceipt(r); err == nil {
		t.Fatal("receipt accepted without gate and delivery")
	}
	if err := s.update(func(st *State) error {
		st.Decisions[r.DecisionID] = ReleaseDecision{ID: r.DecisionID, AuthorAgentID: agents[2].ID, Repository: "yu-min3/kensan-lab", Ref: "refs/heads/sense-dev/fixture", Verdict: "allow", HeadSHA: head, Operation: "pr_create", TargetEnvironment: r.Environment, PolicyVersion: ReleasePolicyVersion, ExpiresAt: time.Now().Add(time.Hour)}
		st.Intents[r.IntentID] = PublishIntent{ID: r.IntentID, DecisionID: r.DecisionID, Status: "sent", HeadSHA: head, Operation: "pr_create"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordDeploymentReceipt(r); err == nil {
		t.Fatal("sent PR treated as deployment")
	}
	for _, status := range []string{"pending", "unknown", "failed"} {
		if err := s.update(func(st *State) error {
			d := st.Decisions[r.DecisionID]
			d.Operation = "deploy"
			st.Decisions[d.ID] = d
			i := st.Intents[r.IntentID]
			i.Operation = "deploy"
			i.Status = status
			st.Intents[i.ID] = i
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if err := s.RecordDeploymentReceipt(r); err == nil {
			t.Fatalf("%s delivery accepted as complete", status)
		}
	}
	if err := s.update(func(st *State) error {
		i := st.Intents[r.IntentID]
		i.Status = "sent"
		i.AuthorizedAt = time.Now().Add(-time.Second)
		i.ExpiresAt = time.Now().Add(time.Minute)
		i.Repository = "yu-min3/kensan-lab"
		i.Ref = "refs/heads/sense-dev/fixture"
		i.TargetEnvironment = r.Environment
		i.PolicyVersion = ReleasePolicyVersion
		i.ExternalID = r.Revision
		st.Intents[i.ID] = i
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordDeploymentReceipt(r); err != nil {
		t.Fatal(err)
	}
	if !appDeploymentReady(s.Snapshot(), s.Snapshot().Tasks[task.ID]) {
		t.Fatal("approved deployed revision not admitted")
	}
	if err := s.RecordDeploymentReceipt(r); err != nil {
		t.Fatal("duplicate receipt not idempotent", err)
	}
	if err := s.SetHeadSHA(task.ID, strings.Repeat("c", 40)); err != nil {
		t.Fatal(err)
	}
	if appDeploymentReady(s.Snapshot(), s.Snapshot().Tasks[task.ID]) {
		t.Fatal("old deployment admitted new revision")
	}
}

func TestDeploymentRejectsMismatchAndWorkerOwnedObservation(t *testing.T) {
	s := testStore(t)
	_, agents, err := s.CreatePlannedTask("app", App, "change", "feature", "v1")
	if err != nil {
		t.Fatal(err)
	}
	r := DeploymentReceipt{ObservedRelease: "v2", TaskID: agents[0].TaskID, DecisionID: "d", IntentID: "i", HeadSHA: strings.Repeat("a", 40), Revision: strings.Repeat("b", 40), ImageDigest: "sha256:" + strings.Repeat("b", 64), Environment: "private-canary", Status: "healthy", UserPath: "private/health"}
	body, _ := json.Marshal(r)
	a, err := s.PutArtifact(agents[0].ID, "deployment_observation", body)
	if err != nil {
		t.Fatal(err)
	}
	r.EvidenceRef = artifactRef(a)
	if err := s.RecordDeploymentReceipt(r); err == nil {
		t.Fatal("wrong revision accepted")
	}
	r.Revision = r.HeadSHA
	body, _ = json.Marshal(r)
	a, err = s.PutArtifact(agents[0].ID, "deployment_observation", body)
	if err != nil {
		t.Fatal(err)
	}
	r.EvidenceRef = artifactRef(a)
	if err := s.RecordDeploymentReceipt(r); err == nil {
		t.Fatal("worker output created trusted deployment receipt")
	}
}

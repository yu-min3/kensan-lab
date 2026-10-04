package core

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// Keep the real Git patch and verification hash aligned; the existing runner
// supplies sessions and the remaining independent stage output contracts.
type imagePipelineRunner struct {
	base, head string
	patch      []byte
}

func (r imagePipelineRunner) Run(ctx context.Context, d Dispatch) (RunResult, error) {
	result, err := (releaseReadyRunner{base: r.base, head: r.head}).Run(ctx, d)
	if err != nil {
		return result, err
	}
	if d.Attempt.Role != "implementation" && d.Attempt.Role != "verification" {
		return result, nil
	}
	var output map[string]any
	if err := json.Unmarshal(result.Output, &output); err != nil {
		return result, err
	}
	switch d.Attempt.Role {
	case "implementation":
		change := output["change"].(map[string]any)
		change["patch"] = string(r.patch)
		change["diff_sha256"] = digest(r.patch)
	case "verification":
		output["diff_sha256"] = digest(r.patch)
	}
	result.Output, err = json.Marshal(output)
	return result, err
}

type imagePipelineAcceptanceRunner struct{ title string }

func (r imagePipelineAcceptanceRunner) Run(_ context.Context, d Dispatch) (RunResult, error) {
	if err := d.BindSession("thread-" + d.Attempt.ID); err != nil {
		return RunResult{}, err
	}
	body, err := json.Marshal(AcceptanceResult{SchemaVersion: 1, Verdict: "pass", ScenarioID: d.Attempt.TaskID, ContractVersion: d.Attempt.ContractVersion, HeadSHA: d.Attempt.HeadSHA, Expected: r.title, Observed: "deployed /api/release returns v2"})
	return RunResult{Output: body}, err
}

type imagePipelineTransport struct {
	operations []string
	mergeSHA   string
}

func (x *imagePipelineTransport) Inspect(context.Context, PublishIntent) (string, bool, error) {
	return "", false, nil
}
func (x *imagePipelineTransport) Execute(_ context.Context, i PublishIntent) (string, error) {
	x.operations = append(x.operations, i.Operation)
	switch i.Operation {
	case "image_publish":
		return i.ImageRelease.DispatchID, nil
	case "merge":
		return x.mergeSHA, nil
	default:
		return i.HeadSHA, nil
	}
}
func pipelineFlow(t *testing.T, d *ReleaseDriver, task, op string) ReleaseFlow {
	t.Helper()
	for _, f := range d.Store.Snapshot().ReleaseFlows {
		if f.AuthorTaskID == task && f.Operation == op && f.Status != "superseded" {
			return f
		}
	}
	t.Fatalf("missing %s flow for %s", op, task)
	return ReleaseFlow{}
}
func pipelinePublish(t *testing.T, d *ReleaseDriver, task, op string, x *imagePipelineTransport) ReleaseFlow {
	t.Helper()
	reconcileDriver(t, d)
	flow := pipelineFlow(t, d, task, op)
	candidate := d.candidate(d.Store.Snapshot().Tasks[task], op)
	categories, _ := ClassifyPlatformProposal(nil, candidate.Impact+"\n"+candidate.PullRequestSummary)
	if len(categories) > 0 && flow.ApprovalID == "" {
		completeDriverGate(t, d, flow, "needs_human")
		flow = pipelineFlow(t, d, task, op)
		if _, err := d.Store.DecideApproval(flow.ApprovalID, "exact-"+flow.GateAgentID, op, flow.HeadSHA, "approved"); err != nil {
			t.Fatal(err)
		}
		previousGate := flow.GateAgentID
		reconcileDriver(t, d)
		reconcileDriver(t, d)
		flow = pipelineFlow(t, d, task, op)
		if flow.GateAgentID == previousGate {
			t.Fatal("classified operation reused approval Gate")
		}
	}
	completeDriverGate(t, d, flow, "allow")
	reconcileDriver(t, d)
	if err := d.PublishReady(context.Background(), x); err != nil {
		t.Fatal(err)
	}
	reconcileDriver(t, d)
	flow = pipelineFlow(t, d, task, op)
	if flow.Status != "sent" {
		t.Fatalf("%s not sent: %+v", op, flow)
	}
	return flow
}
func imageObservation(t *testing.T, s *Store, r ImageReleaseRecord, owner string) ImageReleaseRecord {
	t.Helper()
	body, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	a, err := s.PutArtifact(owner, "image_release_observation", body)
	if err != nil {
		t.Fatal(err)
	}
	r.EvidenceRef = artifactRef(a)
	return r
}
func deploymentObservation(t *testing.T, s *Store, r DeploymentReceipt, owner string) DeploymentReceipt {
	t.Helper()
	body, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	a, err := s.PutArtifact(owner, "deployment_observation", body)
	if err != nil {
		t.Fatal(err)
	}
	r.EvidenceRef = artifactRef(a)
	return r
}

func TestImagePipelineControllerFromSourceThroughDigestChildAndAcceptance(t *testing.T) {
	ctx := context.Background()
	sourceRepo, base := ownershipRepo(t)
	aSHA := commitTestFile(t, sourceRepo, "apps/canary/main.py", "print('user path v2')\n", "source A")
	patchA := []byte(gitTest(t, sourceRepo, "diff", base, aSHA, "--") + "\n")
	s := testStore(t)
	source, stages, err := s.CreatePlannedTask("image-scenario", App, "change", "private canary user path", "v1")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetBaseSHA(source.ID, base); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.CreateLinkedAcceptanceTask(source.ID, "deployed user path returns v2"); err != nil {
		t.Fatal(err)
	}
	for stageIndex := range stages {
		if worked, err := s.Tick(ctx, imagePipelineRunner{base, aSHA, patchA}, []string{"isolated-model-worker"}); err != nil || !worked {
			t.Fatalf("A stage %d %t %v", stageIndex, worked, err)
		}
	}
	root := t.TempDir()
	gitTest(t, root, "clone", "-q", sourceRepo, source.ID)
	d := &ReleaseDriver{Store: s, WorktreeRoot: root, Plan: ReleasePlan{SchemaVersion: 1, MissionID: source.MissionID, Repository: "yu-min3/kensan-lab", TargetEnvironment: "private-canary", Impact: "private image publication and canary application", Rollback: "revert reviewed values", PullRequestSummary: "private canary v2", ImageWorkflow: &ImageWorkflowPlan{Ref: "refs/tags/trusted-image-v1", SHA: strings.Repeat("c", 40), SHA256: strings.Repeat("d", 64)}}}
	external := &imagePipelineTransport{}
	pipelinePublish(t, d, source.ID, "branch_push", external)
	imageFlow := pipelineFlow(t, d, source.ID, "image_publish")
	if imageFlow.GateAgentID == pipelineFlow(t, d, source.ID, "branch_push").GateAgentID {
		t.Fatal("image operation reused push Gate")
	}
	completeDriverGate(t, d, imageFlow, "needs_human")
	awaiting := pipelineFlow(t, d, source.ID, "image_publish")
	if awaiting.Status != "needs_human" || awaiting.IntentID != "" {
		t.Fatal("image approval skipped")
	}
	if _, err := s.DecideApproval(awaiting.ApprovalID, "exact-image-tap", "image_publish", aSHA, "approved"); err != nil {
		t.Fatal(err)
	}
	reconcileDriver(t, d)
	fresh := pipelineFlow(t, d, source.ID, "image_publish")
	if fresh.GateAgentID == imageFlow.GateAgentID {
		t.Fatal("approval replaced fresh independent Gate")
	}
	reconcileDriver(t, d)
	imageFlow = pipelinePublish(t, d, source.ID, "image_publish", external)
	for _, f := range s.Snapshot().ReleaseFlows {
		if f.AuthorTaskID == source.ID && (f.Operation == "pr_create" || f.Operation == "merge") {
			t.Fatal("A created a GitOps PR/merge")
		}
	}
	imageIntent := s.Snapshot().Intents[imageFlow.IntentID]
	spec := imageIntent.ImageRelease
	image := ImageDeploymentSpec{WorkflowRef: spec.WorkflowRef, SourceSHA: spec.SourceSHA, SourceAppTreeSHA: spec.SourceAppTreeSHA, ImageTag: spec.ImageTag, Digest: "sha256:" + strings.Repeat("e", 64), WorkflowSHA: spec.WorkflowSHA, WorkflowSHA256: spec.WorkflowSHA256, DispatchID: spec.DispatchID, WorkflowRunID: 77}
	record := ImageReleaseRecord{SourceTaskID: source.ID, DecisionID: imageFlow.DecisionID, IntentID: imageFlow.IntentID, Image: image}
	for name, mutate := range map[string]func(*ImageReleaseRecord){"source": func(r *ImageReleaseRecord) {
		r.Image.SourceSHA = base
		r.Image.ImageTag = "sense-" + base + "-" + r.Image.DispatchID
	}, "tree": func(r *ImageReleaseRecord) { r.Image.SourceAppTreeSHA = strings.Repeat("f", 40) }} {
		wrong := record
		mutate(&wrong)
		wrong = imageObservation(t, s, wrong, "system")
		if err := s.RecordImageRelease(wrong); err == nil {
			t.Fatalf("false CI %s admitted", name)
		}
	}
	if err := s.RecordImageRelease(imageObservation(t, s, record, stages[2].ID)); err == nil {
		t.Fatal("worker artifact admitted as host CI evidence")
	}
	if err := s.RecordImageRelease(imageObservation(t, s, record, "system")); err != nil {
		t.Fatal(err)
	}
	if n, err := s.ReconcileImageDeployments(source.MissionID); err != nil || n != 1 {
		t.Fatalf("child reservation %d %v", n, err)
	}
	childID := s.Snapshot().ImageReleases[source.ID].DeploymentTaskID
	child := s.Snapshot().Tasks[childID]
	if child.BaseSHA != base || child.HeadSHA != aSHA || child.ImageSourceTaskID != source.ID {
		t.Fatal("B lost original base or source seed A")
	}
	// Reopen the persisted controller ledger and continue with the same driver plan.
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(s.root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	d = &ReleaseDriver{Store: s, Plan: d.Plan, WorktreeRoot: root}
	if n, err := s.ReconcileImageDeployments(source.MissionID); err != nil || n != 0 || s.Snapshot().ImageReleases[source.ID].DeploymentTaskID != childID {
		t.Fatal("restart duplicated deployment child")
	}
	gitTest(t, root, "clone", "-q", filepath.Join(root, source.ID), childID)
	childRepo := filepath.Join(root, childID)
	gitTest(t, childRepo, "config", "user.name", "Test")
	gitTest(t, childRepo, "config", "user.email", "test@example.invalid")
	values := strings.Replace(canaryValues, "tag: first", "digest: "+image.Digest, 1)
	bSHA := commitTestFile(t, childRepo, "kubernetes/apps/app-canary/values.yaml", values, "digest child B")
	patchB := []byte(gitTest(t, childRepo, "diff", base, bSHA, "--") + "\n")
	childAgents := 0
	for _, agent := range s.Snapshot().Agents {
		if agent.TaskID == childID {
			childAgents++
		}
	}
	if childAgents != 5 {
		t.Fatalf("B independent stages %d", childAgents)
	}
	for range 5 {
		if worked, err := s.Tick(ctx, imagePipelineRunner{base, bSHA, patchB}, []string{"isolated-model-worker"}); err != nil || !worked {
			t.Fatalf("B stage %t %v", worked, err)
		}
	}
	// Real merge C combines B with the trusted original main base.
	gitTest(t, sourceRepo, "fetch", "-q", childRepo, bSHA)
	gitTest(t, sourceRepo, "checkout", "-q", "--detach", base)
	gitTest(t, sourceRepo, "merge", "--no-ff", "-qm", "merge C", bSHA)
	cSHA := gitTest(t, sourceRepo, "rev-parse", "HEAD")
	external.mergeSHA = cSHA
	if cSHA == bSHA || cSHA == aSHA || aSHA == bSHA {
		t.Fatal("A/B/C identities collapsed")
	}
	pipelinePublish(t, d, childID, "branch_push", external)
	pipelinePublish(t, d, childID, "pr_create", external)
	mergeFlow := pipelinePublish(t, d, childID, "merge", external)
	mergeIntent := s.Snapshot().Intents[mergeFlow.IntentID]
	if mergeIntent.HeadSHA != bSHA || mergeIntent.ExternalID != cSHA || mergeIntent.ImageDeployment == nil || *mergeIntent.ImageDeployment != image {
		t.Fatal("merge lost B/C or image D built from A")
	}
	if n, err := s.ReconcileReviewedHandoffs(); err != nil || n != 0 {
		t.Fatal("dispatch alone authorized deployed acceptance")
	}
	receipt := DeploymentReceipt{TaskID: childID, DecisionID: mergeFlow.DecisionID, IntentID: mergeFlow.IntentID, HeadSHA: bSHA, Revision: cSHA, ImageSourceSHA: aSHA, ImageDigest: image.Digest, Environment: "private-canary", Status: "healthy", UserPath: "/api/release", ObservedRelease: "v2"}
	for name, mutate := range map[string]func(*DeploymentReceipt){"digest": func(r *DeploymentReceipt) { r.ImageDigest = "sha256:" + strings.Repeat("f", 64) }, "source": func(r *DeploymentReceipt) { r.ImageSourceSHA = cSHA }, "head": func(r *DeploymentReceipt) { r.HeadSHA = aSHA }} {
		wrong := receipt
		mutate(&wrong)
		wrong = deploymentObservation(t, s, wrong, "system")
		if err := s.RecordDeploymentReceipt(wrong); err == nil {
			t.Fatalf("false deployed %s admitted", name)
		}
	}
	if err := s.RecordDeploymentReceipt(deploymentObservation(t, s, receipt, "system")); err != nil {
		t.Fatal(err)
	}
	if n, err := s.ReconcileReviewedHandoffs(); err != nil || n != 1 {
		t.Fatalf("B acceptance handoff %d %v", n, err)
	}
	var acceptance Task
	for _, task := range s.Snapshot().Tasks {
		if task.Kind == "acceptance" && task.SourceTaskID == childID {
			acceptance = task
		}
	}
	if acceptance.ID == "" {
		t.Fatal("B acceptance missing")
	}
	if err := s.SetBaseSHA(acceptance.ID, bSHA); err != nil {
		t.Fatal(err)
	}
	if worked, err := s.Tick(ctx, imagePipelineAcceptanceRunner{title: acceptance.Title}, []string{"isolated-model-worker"}); err != nil || !worked {
		t.Fatalf("deployed acceptance %t %v", worked, err)
	}
	if _, err := s.ReconcileAcceptanceOutcomes(); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{acceptance.ID, childID, source.ID} {
		if s.Snapshot().Tasks[id].Status != "done" {
			t.Fatalf("%s did not complete", id)
		}
	}
	for _, flow := range s.Snapshot().ReleaseFlows {
		if flow.AuthorTaskID == source.ID && (flow.Operation == "pr_create" || flow.Operation == "merge") {
			t.Fatal("driver later promoted A instead of its digest child")
		}
	}
	want := []string{"branch_push", "image_publish", "branch_push", "pr_create", "merge"}
	if strings.Join(external.operations, ",") != strings.Join(want, ",") {
		t.Fatalf("external order %v", external.operations)
	}
}

package core

import (
	"strings"
	"testing"
)

func testImageReleaseSpec(source, tree string) *ImageReleaseSpec {
	id := strings.Repeat("f", 32)
	return &ImageReleaseSpec{SourceSHA: source, SourceAppTreeSHA: tree, ImageTag: "sense-" + source + "-" + id, WorkflowRef: "refs/tags/sense-image-workflow-v1", WorkflowSHA: strings.Repeat("c", 40), WorkflowPath: CanaryImageWorkflowPath, WorkflowSHA256: strings.Repeat("d", 64), DispatchID: id}
}

func TestImageReleaseCandidatePinsAllInputs(t *testing.T) {
	head, tree := strings.Repeat("a", 40), strings.Repeat("e", 40)
	scan := ReleaseScan{Team: App, SourceAppTreeSHA: tree, HeadSHA: head, Operation: "image_publish", Repository: "yu-min3/kensan-lab", Ref: "refs/heads/sense-dev/task"}
	candidate := ReleaseCandidate{SchemaVersion: 1, Operation: scan.Operation, Repository: scan.Repository, Ref: scan.Ref, HeadSHA: head, TargetEnvironment: "private-canary", Impact: "private image build", Rollback: "retain prior image", ImageRelease: testImageReleaseSpec(head, tree)}
	if err := validateReleaseCandidate(candidate, scan); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*ReleaseCandidate){
		func(c *ReleaseCandidate) { c.ImageRelease = nil },
		func(c *ReleaseCandidate) { c.TargetEnvironment = "github" },
		func(c *ReleaseCandidate) { c.ImageRelease.SourceSHA = strings.Repeat("b", 40) },
		func(c *ReleaseCandidate) { c.ImageRelease.SourceAppTreeSHA = strings.Repeat("b", 40) },
		func(c *ReleaseCandidate) { c.ImageRelease.WorkflowPath = ".github/workflows/unsafe.yml" },
		func(c *ReleaseCandidate) { c.ImageRelease.WorkflowRef = "refs/heads/untrusted" },
		func(c *ReleaseCandidate) { c.ImageRelease.WorkflowSHA = "short" },
		func(c *ReleaseCandidate) { c.ImageRelease.WorkflowSHA256 = "short" },
		func(c *ReleaseCandidate) { c.ImageRelease.DispatchID = "reused" },
		func(c *ReleaseCandidate) { c.ImageRelease.ImageTag = "latest" },
		func(c *ReleaseCandidate) { c.Operation = "merge" },
	} {
		wrong := candidate
		wrong.ImageRelease = cloneImageRelease(candidate.ImageRelease)
		mutate(&wrong)
		if validateReleaseCandidate(wrong, scan) == nil {
			t.Fatalf("unsafe image candidate accepted: %+v", wrong)
		}
	}
	scan.Team = Platform
	if validateReleaseCandidate(candidate, scan) == nil {
		t.Fatal("Platform borrowed App image scope")
	}
}

func TestImageReleaseScanObtainsActualAppTree(t *testing.T) {
	dir, base := ownershipRepo(t)
	head := commitTestFile(t, dir, "apps/canary/main.py", "print('hello')\n", "app source")
	scan, err := ScanGitRangeForTeam(dir, base, head, "image_publish", "refs/heads/sense-dev/task", App)
	if err != nil || scan.Status != "candidate" || scan.SourceAppTreeSHA != gitTest(t, dir, "rev-parse", head+":apps/canary") {
		t.Fatalf("App tree: %+v %v", scan, err)
	}
	if _, err := ScanGitRangeForTeam(dir, base, head, "image_publish", "refs/heads/sense-dev/task", Platform); err == nil {
		t.Fatal("Platform image scan accepted")
	}
}

func TestImagePublishGateAndIntentKeepExactSpec(t *testing.T) {
	s, origin, candidate, request := classifiedReleaseFixture(t, "image_publish")
	if _, err := s.PreparePublish(origin.ID, origin.Operation, origin.Repository, origin.Ref, origin.HeadSHA); err == nil {
		t.Fatal("unapproved image published")
	}
	if _, err := s.DecideApproval(request.ID, "tap", origin.Operation, origin.HeadSHA, "approved"); err != nil {
		t.Fatal(err)
	}
	_, gate := taskAgent(t, s, Platform, "release_gate")
	if err := s.BindReleaseGateInputs(gate.ID, origin.AuthorAgentID, origin.ArtifactRefs, origin.ScanRef, candidate); err != nil {
		t.Fatal(err)
	}
	d := gateFixtureDecision(t, s, gate.ID, origin.AuthorAgentID, origin.ArtifactRefs[0], origin.ScanRef, "allow", "approved image inputs verified")
	d.ApprovalID = request.ID
	d.ImageRelease = cloneImageRelease(candidate.ImageRelease)
	d.ImageRelease.ImageTag = "changed"
	if _, err := s.RecordReleaseDecision(d); err == nil {
		t.Fatal("decision changed image inputs")
	}
	d.ImageRelease = nil
	d, err := s.RecordReleaseDecision(d)
	if err != nil {
		t.Fatal(err)
	}
	if !imageReleasesEqual(d.ImageRelease, candidate.ImageRelease) {
		t.Fatal("fixed candidate spec omitted from decision")
	}
	intent, err := s.PreparePublish(d.ID, d.Operation, d.Repository, d.Ref, d.HeadSHA)
	if err != nil {
		t.Fatal(err)
	}
	if !imageReleasesEqual(intent.ImageRelease, candidate.ImageRelease) {
		t.Fatal("image intent differs from candidate")
	}
	intent.ImageRelease.ImageTag = "caller mutation"
	d.ImageRelease.WorkflowSHA = "caller mutation"
	if !imageReleasesEqual(s.Snapshot().Intents[intent.ID].ImageRelease, candidate.ImageRelease) || !imageReleasesEqual(s.Snapshot().Decisions[d.ID].ImageRelease, candidate.ImageRelease) {
		t.Fatal("caller mutated controller state through image pointer")
	}
	if err := s.update(func(st *State) error {
		i := st.Intents[intent.ID]
		i.ImageRelease.ImageTag = "tampered"
		st.Intents[i.ID] = i
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PreparePublish(d.ID, d.Operation, d.Repository, d.Ref, d.HeadSHA); err == nil {
		t.Fatal("changed image intent reused")
	}
}

func TestAppImageDigestContract(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	for _, image := range []map[string]any{
		{"repository": CanaryImageRepository, "digest": digest},
		{"repository": CanaryImageRepository, "digest": digest, "tag": ""},
		{"repository": CanaryImageRepository, "tag": "v1", "digest": ""},
	} {
		if err := validAppImage(image); err != nil {
			t.Fatalf("valid digest/tag rejected: %v", err)
		}
	}
	for _, image := range []map[string]any{
		{"repository": CanaryImageRepository, "digest": digest, "tag": "v1"},
		{"repository": CanaryImageRepository, "digest": "sha256:bad"},
		{"repository": "ghcr.io/other/app", "digest": digest},
		{"repository": CanaryImageRepository},
	} {
		if validAppImage(image) == nil {
			t.Fatalf("unsafe image accepted: %+v", image)
		}
	}
}

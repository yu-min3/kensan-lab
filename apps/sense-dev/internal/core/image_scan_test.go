package core

import (
	"strings"
	"testing"
)

func TestImageDeploymentScanRejectsSourceAndNonImageChanges(t *testing.T) {
	for _, failure := range []string{"none", "source", "replicas", "intermediate"} {
		t.Run(failure, func(t *testing.T) {
			repo, base := ownershipRepo(t)
			source := commitTestFile(t, repo, "apps/canary/main.py", "print('v2')\n", "app")
			s := testStore(t)
			original, _, err := s.CreatePlannedTask("mission", App, "change", "feature", "v1")
			if err != nil {
				t.Fatal(err)
			}
			child, _, err := s.CreatePlannedTask("mission", App, "change", "digest", "v1")
			if err != nil {
				t.Fatal(err)
			}
			spec := testImageReleaseSpec(source, gitTest(t, repo, "rev-parse", source+":apps/canary"))
			image := ImageDeploymentSpec{SourceSHA: source, SourceAppTreeSHA: spec.SourceAppTreeSHA, ImageTag: spec.ImageTag, WorkflowRef: spec.WorkflowRef, WorkflowSHA: spec.WorkflowSHA, WorkflowSHA256: spec.WorkflowSHA256, DispatchID: spec.DispatchID, WorkflowRunID: 12, Digest: "sha256:" + strings.Repeat("d", 64)}
			values := strings.Replace(canaryValues, "tag: first", "tag: ''\n  digest: "+image.Digest, 1)
			if failure == "intermediate" {
				commitTestFile(t, repo, "apps/canary/main.py", "print('wrong')\n", "bad intermediate")
				commitTestFile(t, repo, "apps/canary/main.py", "print('v2')\n", "restore")
			}
			head := commitTestFile(t, repo, "kubernetes/apps/app-canary/values.yaml", values, "digest")
			if failure == "source" {
				head = commitTestFile(t, repo, "apps/canary/main.py", "print('wrong')\n", "source change")
			}
			if failure == "replicas" {
				head = commitTestFile(t, repo, "kubernetes/apps/app-canary/values.yaml", strings.Replace(values, "replicas: 1", "replicas: 2", 1), "replicas")
			}
			root := t.TempDir()
			gitTest(t, root, "clone", "-q", repo, child.ID)
			err = s.update(func(st *State) error {
				original.BaseSHA, original.HeadSHA, original.Status = base, source, "publish_wait"
				child.BaseSHA, child.HeadSHA, child.ImageSourceTaskID = base, head, original.ID
				st.Tasks[original.ID] = original
				st.Tasks[child.ID] = child
				st.ImageReleases[original.ID] = ImageReleaseRecord{SourceTaskID: original.ID, DeploymentTaskID: child.ID, Image: image}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			driver := ReleaseDriver{Store: s, WorktreeRoot: root}
			scan := ReleaseScan{HeadSHA: head}
			err = driver.enrichImageScan(child, &scan)
			if (err == nil) != (failure == "none") {
				t.Fatalf("scan failure=%s err=%v", failure, err)
			}
			if failure == "none" && (!scan.ImageValuesOnly || scan.SourceAppTreeSHA != image.SourceAppTreeSHA || scan.ImageDigest != image.Digest) {
				t.Fatal("native image provenance missing")
			}
		})
	}
}

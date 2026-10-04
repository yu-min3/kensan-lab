package deploymentobserver

import (
	"strings"
	"testing"
)

func TestImageValuesMustPinProvenDigest(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	valid := "image:\n  repository: " + image + "\n  tag: ''\n  digest: " + digest + "\n"
	if err := validateImageValues([]byte(valid), digest); err != nil {
		t.Fatal(err)
	}
	for name, yaml := range map[string]string{
		"mutable tag":      "image:\n  repository: " + image + "\n  tag: v1\n  digest: " + digest + "\n",
		"other repository": "image:\n  repository: ghcr.io/other/repo\n  tag: ''\n  digest: " + digest + "\n",
		"different digest": "image:\n  repository: " + image + "\n  tag: ''\n  digest: sha256:" + strings.Repeat("b", 64) + "\n",
		"missing image":    "replicas: 1\n",
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateImageValues([]byte(yaml), digest); err == nil {
				t.Fatal("unproven image values accepted")
			}
		})
	}
}

func TestWorkflowRunPathRequiresFixedFileAndRef(t *testing.T) {
	ref := "refs/tags/sense-image-workflow-v1"
	for _, path := range []string{".github/workflows/canary-image.yml", ".github/workflows/canary-image.yml@" + ref, repository + "/.github/workflows/canary-image.yml@" + ref} {
		if !validWorkflowRunPath(path, ref) {
			t.Fatalf("fixed workflow path rejected: %s", path)
		}
	}
	for _, path := range []string{".github/workflows/canary-image.yml@refs/heads/main", ".github/workflows/canary-image.yml@refs/tags/other", ".github/workflows/canary-image.yml.backup@" + ref, "other/.github/workflows/canary-image.yml@" + ref} {
		if validWorkflowRunPath(path, ref) {
			t.Fatalf("other workflow path accepted: %s", path)
		}
	}
}

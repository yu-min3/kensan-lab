package core

import (
	"strings"
	"testing"
)

func TestFixedPlatformClassifications(t *testing.T) {
	cases := []struct{ path, body, category, status string }{
		{"kubernetes/auth/keycloak/config.yaml", "setting: changed\n", "auth", "needs_human"},
		{"kubernetes/secrets/vault/values.yaml", "setting: changed\n", "secret", "needs_human"},
		{"kubernetes/network/cilium/values.yaml", "setting: changed\n", "publication", "needs_human"},
		{"apps/sense-dev/internal/core/ownership_policy.go", "policy changed\n", "control", "needs_human"},
		{"kubernetes/apps/app-canary/resources/pvc.yaml", "kind: PersistentVolumeClaim\n", "data_destruction", "needs_human"},
		{".github/workflows/ci.yml", "permissions:\n  contents: write\n", "secret", "needs_human"},
		{"apps/canary/config.txt", "OPENAI_API_KEY=sk-example-secret-value\n", "", "deny"},
		{"apps/konro/main.py", "safe app code\n", "", "deny"},
		{".github/workflows/ci.yml", "name: unit-test\n", "", "candidate"},
	}
	for _, tt := range cases {
		t.Run(tt.path+tt.status, func(t *testing.T) {
			dir, base := ownershipRepo(t)
			head := commitTestFile(t, dir, tt.path, tt.body, "change")
			scan, err := ScanGitRange(dir, base, head, "pr_create", "refs/heads/feat/canary")
			if err != nil || scan.Status != tt.status || tt.category != "" && !containsFinding(scan.HumanCategories, tt.category) {
				t.Fatalf("classification: %+v %v", scan, err)
			}
		})
	}
	t.Run("nested public activation", func(t *testing.T) {
		dir, base := ownershipRepo(t)
		head := commitTestFile(t, dir, "kubernetes/apps/app-canary/values.yaml", strings.Replace(canaryValues, "enabled: false", "enabled: true", 1), "activate")
		scan, err := ScanGitRange(dir, base, head, "pr_create", "refs/heads/feat/canary")
		if err != nil || scan.Status != "deny" || !containsFinding(scan.HumanCategories, "publication") {
			t.Fatalf("public activation: %+v %v", scan, err)
		}
	})
	t.Run("public key deletion", func(t *testing.T) {
		dir, base := ownershipRepo(t)
		head := commitTestFile(t, dir, "kubernetes/apps/app-canary/values.yaml", strings.Replace(canaryValues, "httproute:\n  enabled: false\n", "", 1), "delete route key")
		scan, err := ScanGitRange(dir, base, head, "pr_create", "refs/heads/feat/canary")
		if err != nil || scan.Status != "needs_human" || !containsFinding(scan.HumanCategories, "publication") {
			t.Fatalf("public key deletion: %+v %v", scan, err)
		}
	})
	t.Run("secret env actual value", func(t *testing.T) {
		dir, base := ownershipRepo(t)
		head := commitTestFile(t, dir, "kubernetes/apps/app-canary/values.yaml", strings.Replace(canaryValues, "APP_MESSAGE", "DATABASE_PASSWORD", 1), "secret env")
		scan, err := ScanGitRange(dir, base, head, "pr_create", "refs/heads/feat/canary")
		if err != nil || scan.Status != "deny" || !containsFinding(scan.HumanCategories, "secret") {
			t.Fatalf("raw secret env: %+v %v", scan, err)
		}
	})
}

func TestProposalClassificationCannotDowngradeActualRisk(t *testing.T) {
	categories, _ := ClassifyPlatformProposal([]string{"kubernetes/auth/keycloak/values.yaml"}, "just docs")
	if !containsFinding(categories, "auth") {
		t.Fatal("proposal path risk missed")
	}
	categories, _ = ClassifyPlatformProposal(nil, "Add public hostname and change publisher permissions")
	if !containsFinding(categories, "publication") || !containsFinding(categories, "control") {
		t.Fatal("proposal text risk missed")
	}
	classified := candidateClassification(ReleaseScan{Status: "candidate"}, ReleaseCandidate{Impact: "change authentication", PullRequestSummary: "update public route"})
	if classified.Status != "needs_human" || !containsFinding(classified.HumanCategories, "auth") || !containsFinding(classified.HumanCategories, "publication") {
		t.Fatal("candidate proposal bypassed classification")
	}
}

package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const canaryValues = `image:
  repository: ghcr.io/yu-min3/canary
  tag: first
replicas: 1
env:
  - name: APP_MESSAGE
    value: hello
httproute:
  enabled: false
pvc:
  enabled: true
`

func TestAppValuesStructuralOwnership(t *testing.T) {
	cases := []struct {
		name, body string
		allow      bool
	}{
		{"image", strings.Replace(canaryValues, "tag: first", "tag: second", 1), true},
		{"replicas", strings.Replace(canaryValues, "replicas: 1", "replicas: 2", 1), true},
		{"nonsecret env", strings.Replace(canaryValues, "value: hello", "value: next", 1), true},
		{"remove nonsecret env", strings.Replace(canaryValues, "env:\n  - name: APP_MESSAGE\n    value: hello\n", "", 1), true},
		{"public key", strings.Replace(canaryValues, "enabled: false", "enabled: true", 1), false},
		{"remove public key", strings.Replace(canaryValues, "httproute:\n  enabled: false\n", "", 1), false},
		{"unknown", canaryValues + "futurePublicKey: true\n", false},
		{"unknown image nested", strings.Replace(canaryValues, "tag: first", "tag: first\n  registryAuth: none", 1), false},
		{"remove image nested", strings.Replace(canaryValues, "  tag: first\n", "", 1), false},
		{"image scalar", strings.Replace(canaryValues, "image:\n  repository: ghcr.io/yu-min3/canary\n  tag: first", "image: null", 1), false},
		{"env secret", strings.Replace(canaryValues, "APP_MESSAGE", "DATABASE_PASSWORD", 1), false},
		{"env unknown", strings.Replace(canaryValues, "APP_MESSAGE", "APP_TOKEN", 1), false},
		{"env load", strings.Replace(canaryValues, "APP_MESSAGE", "DEMO_LOAD_ENABLED", 1), false},
		{"env valueFrom", strings.Replace(canaryValues, "value: hello", "valueFrom: {secretKeyRef: {name: vault, key: token}}", 1), false},
		{"env extra", strings.Replace(canaryValues, "value: hello", "value: hello\n    valueFrom: {}", 1), false},
		{"credential in allowed env", strings.Replace(canaryValues, "value: hello", "value: ghp_abcdefghijklmnopqrstuvwxyz", 1), false},
		{"duplicate", canaryValues + "replicas: 2\n", false},
		{"nested duplicate", strings.Replace(canaryValues, "tag: first", "tag: first\n  tag: second", 1), false},
		{"alias", strings.Replace(canaryValues, "replicas: 1", "replicas: &count 1\ncopy: *count", 1), false},
		{"multidoc", canaryValues + "---\nreplicas: 2\n", false},
		{"null", "null\n", false},
		{"bad YAML", "image: [\n", false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			err := appValuesChange([]byte(canaryValues), []byte(tt.body))
			if (err == nil) != tt.allow {
				t.Fatalf("allow=%t error=%v", tt.allow, err)
			}
		})
	}
	secretBefore := strings.Replace(canaryValues, "APP_MESSAGE", "DATABASE_PASSWORD", 1)
	if appValuesChange([]byte(secretBefore), []byte(canaryValues)) == nil {
		t.Fatal("secret env removal/rename accepted")
	}
	unknownBefore := canaryValues + "futurePublicKey: true\n"
	if appValuesChange([]byte(unknownBefore), []byte(canaryValues)) == nil {
		t.Fatal("unknown key deletion accepted")
	}
}

func ownershipRepo(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	gitTest(t, dir, "init", "-q")
	gitTest(t, dir, "config", "user.name", "Test")
	gitTest(t, dir, "config", "user.email", "test@example.invalid")
	base := commitTestFile(t, dir, "kubernetes/apps/app-canary/values.yaml", canaryValues, "base")
	return dir, base
}

func TestAppScanFixedOwnershipAcrossCommits(t *testing.T) {
	for _, path := range []string{"apps/konro/main.go", "charts/app-base/values.yaml", "backstage/templates/a.yaml", ".github/workflows/ci.yml", "kubernetes/apps/app-canary/resources/pvc.yaml", "infra/a.yaml", "apps/canary/.env"} {
		t.Run(path, func(t *testing.T) {
			dir, base := ownershipRepo(t)
			head := commitTestFile(t, dir, path, "safe-looking content\n", "outside scope")
			report, err := ScanGitRangeForTeam(dir, base, head, "pr_create", "refs/heads/feat/canary", App)
			if err != nil || report.Status != "deny" || !containsFinding(report.Findings, "App ownership denies path: "+path) {
				t.Fatalf("ownership missed: %+v %v", report, err)
			}
		})
	}
	t.Run("allowed code and values", func(t *testing.T) {
		dir, base := ownershipRepo(t)
		commitTestFile(t, dir, "apps/canary/app/main.py", "print('hello')\n", "code")
		head := commitTestFile(t, dir, "kubernetes/apps/app-canary/values.yaml", strings.Replace(canaryValues, "tag: first", "tag: second", 1), "image")
		report, err := ScanGitRangeForTeam(dir, base, head, "pr_create", "refs/heads/feat/canary", App)
		if err != nil || report.Status != "candidate" || report.Team != App {
			t.Fatalf("safe App denied: %+v %v", report, err)
		}
	})
	t.Run("intermediate public change restored", func(t *testing.T) {
		dir, base := ownershipRepo(t)
		commitTestFile(t, dir, "kubernetes/apps/app-canary/values.yaml", strings.Replace(canaryValues, "enabled: false", "enabled: true", 1), "public")
		head := commitTestFile(t, dir, "kubernetes/apps/app-canary/values.yaml", canaryValues, "restore")
		report, err := ScanGitRangeForTeam(dir, base, head, "pr_create", "refs/heads/feat/canary", App)
		if err != nil || report.Status != "deny" {
			t.Fatalf("intermediate public change missed: %+v %v", report, err)
		}
	})
	t.Run("rename into allowed path", func(t *testing.T) {
		dir, _ := ownershipRepo(t)
		base := commitTestFile(t, dir, "apps/konro/main.py", "print('hello')\n", "other app")
		if err := os.MkdirAll(filepath.Join(dir, "apps/canary"), 0700); err != nil {
			t.Fatal(err)
		}
		gitTest(t, dir, "mv", "apps/konro/main.py", "apps/canary/main.py")
		gitTest(t, dir, "commit", "-qm", "rename")
		head := gitTest(t, dir, "rev-parse", "HEAD")
		report, err := ScanGitRangeForTeam(dir, base, head, "pr_create", "refs/heads/feat/canary", App)
		if err != nil || report.Status != "deny" || !containsFinding(report.Findings, "App ownership denies path: apps/konro/main.py") {
			t.Fatalf("rename source missed: %+v %v", report, err)
		}
	})
	t.Run("values removal", func(t *testing.T) {
		dir, base := ownershipRepo(t)
		gitTest(t, dir, "rm", "kubernetes/apps/app-canary/values.yaml")
		gitTest(t, dir, "commit", "-qm", "remove")
		head := gitTest(t, dir, "rev-parse", "HEAD")
		report, err := ScanGitRangeForTeam(dir, base, head, "pr_create", "refs/heads/feat/canary", App)
		if err != nil || report.Status != "deny" {
			t.Fatalf("values removal missed: %+v %v", report, err)
		}
	})
	t.Run("owned symlink", func(t *testing.T) {
		dir, base := ownershipRepo(t)
		if err := os.MkdirAll(filepath.Join(dir, "apps/canary"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("../../private", filepath.Join(dir, "apps/canary/link")); err != nil {
			t.Fatal(err)
		}
		gitTest(t, dir, "add", ".")
		gitTest(t, dir, "commit", "-qm", "link")
		head := gitTest(t, dir, "rev-parse", "HEAD")
		report, err := ScanGitRangeForTeam(dir, base, head, "pr_create", "refs/heads/feat/canary", App)
		if err != nil || report.Status != "deny" {
			t.Fatalf("owned symlink accepted: %+v %v", report, err)
		}
	})
}

func TestScanCannotBorrowOtherTeamOwnership(t *testing.T) {
	for _, team := range []Team{"", Platform} {
		if scanMatchesAuthorTeam(ReleaseScan{Team: team}, Agent{Team: App}, Task{Team: App}) {
			t.Fatal("App reused Platform scan")
		}
	}
	if !scanMatchesAuthorTeam(ReleaseScan{Team: App}, Agent{Team: App}, Task{Team: App}) {
		t.Fatal("own App scan rejected")
	}
	if scanMatchesAuthorTeam(ReleaseScan{Team: App}, Agent{Team: App}, Task{Team: Platform}) {
		t.Fatal("task team mutation accepted")
	}
}

func TestControllerScanEnforcesAuthorOwnership(t *testing.T) {
	dir, base := ownershipRepo(t)
	head := commitTestFile(t, dir, "apps/canary/main.py", "print('hello')\n", "code")
	s := testStore(t)
	_, author := taskAgent(t, s, App, "implementation")
	decision := ReleaseDecision{AuthorAgentID: author.ID, Repository: "yu-min3/kensan-lab", Ref: "refs/heads/feat/canary", Operation: "pr_create", HeadSHA: head, PolicyVersion: ReleasePolicyVersion}
	_, platformRef, err := s.ScanAndRecordRelease(dir, base, head, decision.Operation, decision.Ref)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.checkReleaseScan(platformRef, decision); err == nil {
		t.Fatal("App author reused Platform controller scan")
	}
	_, appRef, err := s.ScanAndRecordReleaseForTeam(dir, base, head, decision.Operation, decision.Ref, App)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.checkReleaseScan(appRef, decision); err != nil {
		t.Fatalf("own App controller scan rejected: %v", err)
	}
	head = commitTestFile(t, dir, "charts/app-base/values.yaml", "replicas: 2\n", "outside")
	decision.HeadSHA = head
	_, deniedRef, err := s.ScanAndRecordReleaseForTeam(dir, base, head, decision.Operation, decision.Ref, App)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.checkReleaseScan(deniedRef, decision); err == nil {
		t.Fatal("denied ownership scan accepted for release")
	}
}

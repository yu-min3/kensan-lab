package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitTest(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func commitTestFile(t *testing.T, dir, path, body, message string) string {
	t.Helper()
	full := filepath.Join(dir, path)
	if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	gitTest(t, dir, "add", "--", path)
	gitTest(t, dir, "commit", "-qm", message)
	return gitTest(t, dir, "rev-parse", "HEAD")
}

func TestReleaseScanInspectsEveryProposedCommit(t *testing.T) {
	dir := t.TempDir()
	gitTest(t, dir, "init", "-q")
	gitTest(t, dir, "config", "user.name", "Test")
	gitTest(t, dir, "config", "user.email", "test@example.invalid")
	base := commitTestFile(t, dir, "README.md", "base\n", "base")
	clean := commitTestFile(t, dir, "docs/guide.md", "safe change\n", "safe")
	report, err := ScanGitRange(dir, base, clean, "pr_create", "refs/heads/feat/canary")
	if err != nil || report.Status != "candidate" || report.CommitCount != 1 || report.DiffSHA256 == "" {
		t.Fatalf("clean scan: %+v %v", report, err)
	}
	s := testStore(t)
	recorded, scanRef, err := s.ScanAndRecordRelease(dir, base, clean, "pr_create", "refs/heads/feat/canary")
	if err != nil || recorded.DiffSHA256 != report.DiffSHA256 || scanRef.ID == "" {
		t.Fatalf("scan was not persisted: %+v %+v %v", recorded, scanRef, err)
	}
	if _, err := s.checkReleaseScan(scanRef, ReleaseDecision{Repository: "yu-min3/kensan-lab", Ref: "refs/heads/feat/canary", Operation: "pr_update", HeadSHA: clean, PolicyVersion: ReleasePolicyVersion}); err == nil {
		t.Fatal("scan reused for a different operation")
	}
	secret := commitTestFile(t, dir, "config/.env", "OPENAI_API_KEY=sk-example-secret-value\n", "secret in intermediate commit")
	if secret == clean {
		t.Fatal("commit did not advance")
	}
	final := commitTestFile(t, dir, "config/.env", "removed\n", "remove secret later")
	report, err = ScanGitRange(dir, base, final, "pr_create", "refs/heads/feat/canary")
	if err != nil || report.Status != "needs_human" || report.CommitCount != 3 {
		t.Fatalf("intermediate secret missed: %+v %v", report, err)
	}
	if len(report.Findings) == 0 {
		t.Fatal("no finding for secret range")
	}
	if _, err := ScanGitRange(dir, base, clean, "pr_create", "refs/heads/feat/canary"); err == nil {
		t.Fatal("scan accepted head different from worktree")
	}
	_ = commitTestFile(t, dir, "manifests/route.yaml", "kind: Ingress\n", "add route")
	current := gitTest(t, dir, "rev-parse", "HEAD")
	report, err = ScanGitRange(dir, final, current, "pr_create", "refs/heads/feat/canary")
	if err != nil || report.Status != "needs_human" {
		t.Fatalf("public route missed: %+v %v", report, err)
	}
}

func TestReleaseScanFlagsIntermediateSymlink(t *testing.T) {
	dir := t.TempDir()
	gitTest(t, dir, "init", "-q")
	gitTest(t, dir, "config", "user.name", "Test")
	gitTest(t, dir, "config", "user.email", "test@example.invalid")
	base := commitTestFile(t, dir, "README.md", "base\n", "base")
	link := filepath.Join(dir, "docs", "shared")
	if err := os.MkdirAll(filepath.Dir(link), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../../private/secret", link); err != nil {
		t.Fatal(err)
	}
	gitTest(t, dir, "add", "--", "docs/shared")
	gitTest(t, dir, "commit", "-qm", "symlink in intermediate commit")
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	gitTest(t, dir, "add", "--", "docs/shared")
	gitTest(t, dir, "commit", "-qm", "remove symlink")
	head := gitTest(t, dir, "rev-parse", "HEAD")
	report, err := ScanGitRange(dir, base, head, "pr_create", "refs/heads/feat/canary")
	if err != nil || report.Status != "needs_human" || report.CommitCount != 2 {
		t.Fatalf("intermediate symlink was not flagged: %+v %v", report, err)
	}
	if !containsFinding(report.Findings, "symlink or submodule change needs manual review") {
		t.Fatalf("symlink finding missing: %v", report.Findings)
	}
}

func TestReleaseScanFlagsSecurityBoundaryChanges(t *testing.T) {
	dir := t.TempDir()
	gitTest(t, dir, "init", "-q")
	gitTest(t, dir, "config", "user.name", "Test")
	gitTest(t, dir, "config", "user.email", "test@example.invalid")
	base := commitTestFile(t, dir, "README.md", "base\n", "base")
	paths := []string{
		"apps/sense-dev/internal/isolation/bwrap.go",
		"apps/sense-dev/internal/workerclient/client.go",
		"apps/sense-dev/internal/workerwire/wire.go",
		"apps/sense-dev/internal/verifier/verifier.go",
		"apps/sense-dev/internal/web/server.go",
		"apps/sense-dev/cmd/sense-dev/main.go",
	}
	for _, path := range paths {
		_ = commitTestFile(t, dir, path, "security boundary\n", "change boundary")
	}
	head := gitTest(t, dir, "rev-parse", "HEAD")
	report, err := ScanGitRange(dir, base, head, "pr_create", "refs/heads/feat/canary")
	if err != nil || report.Status != "needs_human" || report.CommitCount != len(paths) {
		t.Fatalf("security boundary change was not flagged: %+v %v", report, err)
	}
	for _, path := range paths {
		if !containsFinding(report.Findings, "publication or policy path needs review: "+path) {
			t.Errorf("missing risk finding for %s", path)
		}
	}
}

func TestParseChangedPathsFlagsSubmoduleMode(t *testing.T) {
	raw := []byte(":000000 160000 before after A\x00modules/example\x00")
	paths, risky, err := parseChangedPaths(raw)
	if err != nil || !risky || len(paths) != 1 || paths[0] != "modules/example" {
		t.Fatalf("submodule mode was not flagged: %v %t %v", paths, risky, err)
	}
}

func TestParseChangedPathsRejectsMalformedRawRecords(t *testing.T) {
	for _, raw := range [][]byte{
		[]byte(":100644 120000 before after A\x00"),
		[]byte(":100644 120000 before after A\x00path"),
		[]byte("not a header\x00path\x00"),
		[]byte(":badmod 100644 before after M\x00path\x00"),
	} {
		if _, _, err := parseChangedPaths(raw); err == nil {
			t.Fatalf("accepted malformed raw change record %q", raw)
		}
	}
}

func containsFinding(findings []string, want string) bool {
	for _, finding := range findings {
		if finding == want {
			return true
		}
	}
	return false
}

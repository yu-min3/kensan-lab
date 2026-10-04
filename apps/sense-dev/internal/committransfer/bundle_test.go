package committransfer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func testGit(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}
func repoFixture(t *testing.T) (source, trusted, base, head string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	source = filepath.Join(root, "source")
	trusted = filepath.Join(root, "trusted")
	os.Mkdir(source, 0700)
	testGit(t, source, "init", "-q")
	testGit(t, source, "config", "user.name", "Test")
	testGit(t, source, "config", "user.email", "test@example.invalid")
	os.WriteFile(filepath.Join(source, "README"), []byte("base"), 0600)
	testGit(t, source, "add", "README")
	testGit(t, source, "commit", "-qm", "base")
	base = testGit(t, source, "rev-parse", "HEAD")
	testGit(t, root, "clone", "-q", source, trusted)
	os.WriteFile(filepath.Join(source, "feature"), []byte("author feature"), 0600)
	testGit(t, source, "add", "feature")
	testGit(t, source, "commit", "-qm", "feature")
	head = testGit(t, source, "rev-parse", "HEAD")
	return
}
func TestFixedIncrementalBundleImportsMissingAuthorCommit(t *testing.T) {
	source, trusted, base, head := repoFixture(t)
	task := strings.Repeat("a", 32)
	b, err := Create(context.Background(), source, task, base, head)
	if err != nil {
		t.Fatal(err)
	}
	if err := Import(context.Background(), trusted, b); err != nil {
		t.Fatal(err)
	}
	if testGit(t, trusted, "rev-parse", "HEAD") != base || testGit(t, trusted, "rev-parse", bundleRef(task, head)) != head {
		t.Fatal("import changed checkout or missed exact commit")
	}
	if err := Import(context.Background(), trusted, b); err != nil {
		t.Fatal("idempotent import", err)
	}
	clone := filepath.Join(filepath.Dir(source), "consumer")
	testGit(t, filepath.Dir(source), "clone", "-q", trusted, clone)
	if testGit(t, clone, "rev-parse", "--verify", head+"^{commit}") != head {
		t.Fatal("import not reachable to controller source clone")
	}
}
func TestWorkerConfigHooksAndCredentialsNeverExecute(t *testing.T) {
	source, trusted, base, head := repoFixture(t)
	marker := filepath.Join(filepath.Dir(source), "executed")
	hook := filepath.Join(source, ".git", "hooks", "pre-upload-pack")
	os.WriteFile(hook, []byte("#!/bin/sh\ntouch "+marker+"\n"), 0700)
	testGit(t, source, "config", "core.fsmonitor", "touch "+marker)
	testGit(t, source, "config", "credential.helper", "!touch "+marker)
	testGit(t, source, "config", "include.path", filepath.Join(filepath.Dir(source), "missing"))
	// Host Git environment must not alter the isolated graph capture either.
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "core.fsmonitor")
	t.Setenv("GIT_CONFIG_VALUE_0", "touch "+marker)
	b, err := Create(context.Background(), source, strings.Repeat("a", 32), base, head)
	if err != nil {
		t.Fatal(err)
	}
	if err := Import(context.Background(), trusted, b); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("worker-controlled configuration executed")
	}
}
func TestBundleTamperingWrongBindingsAndIndirectObjectsRejected(t *testing.T) {
	source, trusted, base, head := repoFixture(t)
	b, err := Create(context.Background(), source, strings.Repeat("a", 32), base, head)
	if err != nil {
		t.Fatal(err)
	}
	for _, modify := range []func(*Bundle){func(b *Bundle) { b.HeadSHA = base }, func(b *Bundle) { b.TaskID = "../../repo" }, func(b *Bundle) { b.BaseSHA = head }, func(b *Bundle) { b.Data = append([]byte(nil), b.Data...); b.Data[len(b.Data)-1] ^= 1 }} {
		invalid := b
		modify(&invalid)
		if err := Import(context.Background(), trusted, invalid); err == nil {
			t.Fatal("invalid bundle accepted")
		}
	}
	missing := b
	missing.BaseSHA = strings.Repeat("b", 40)
	if err := Import(context.Background(), trusted, missing); err == nil {
		t.Fatal("unknown trusted base accepted")
	}
	malformed := b
	malformed.Data = []byte("not a git bundle")
	h := sha256.Sum256(malformed.Data)
	malformed.SHA256 = hex.EncodeToString(h[:])
	if err := Import(context.Background(), trusted, malformed); err == nil {
		t.Fatal("digest replaced Git verification")
	}
	if err := os.WriteFile(filepath.Join(source, ".git", "objects", "info", "alternates"), []byte(filepath.Join(trusted, ".git", "objects")), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Create(context.Background(), source, b.TaskID, base, head); err == nil {
		t.Fatal("external alternates accepted")
	}
	os.Remove(filepath.Join(source, ".git", "objects", "info", "alternates"))
	if err := os.Symlink(filepath.Join(trusted, ".git", "objects"), filepath.Join(source, ".git", "objects", "ff")); err != nil {
		t.Fatal(err)
	}
	if _, err := Create(context.Background(), source, b.TaskID, base, head); err == nil {
		t.Fatal("object symlink accepted")
	}
}

func TestObjectStorageSizeLimitBeforeCapture(t *testing.T) {
	source, _, base, head := repoFixture(t)
	pack := filepath.Join(source, ".git", "objects", "pack", "pack-"+strings.Repeat("f", 40)+".pack")
	file, err := os.OpenFile(pack, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(MaxObjectBytes + 1); err != nil {
		t.Fatal(err)
	}
	file.Close()
	if _, err := Create(context.Background(), source, strings.Repeat("a", 32), base, head); err == nil {
		t.Fatal("oversized storage accepted")
	}
}

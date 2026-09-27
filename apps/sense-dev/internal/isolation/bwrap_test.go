package isolation

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func testConfig(t *testing.T) Config {
	t.Helper()
	base := t.TempDir()
	for _, name := range []string{"runtime/usr/bin", "runtime/workspace", "runtime/agent-auth", "runtime/proc", "runtime/dev", "runtime/tmp", "worktree", "auth", "state"} {
		if err := os.MkdirAll(filepath.Join(base, name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(base, "runtime/usr/bin/worker"), []byte("test"), 0700); err != nil {
		t.Fatal(err)
	}
	return Config{Bubblewrap: "/bin/echo", RuntimeRoot: filepath.Join(base, "runtime"), Worktree: filepath.Join(base, "worktree"), AuthHome: filepath.Join(base, "auth"), ControllerState: filepath.Join(base, "state")}
}

func TestCommandMountsOnlyAllowlistedPaths(t *testing.T) {
	c := testConfig(t)
	cmd, err := c.Command(context.Background(), "/usr/bin/worker", "--task", "one")
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := filepath.EvalSymlinks(c.RuntimeRoot)
	if err != nil {
		t.Fatal(err)
	}
	worktree, err := filepath.EvalSymlinks(c.Worktree)
	if err != nil {
		t.Fatal(err)
	}
	auth, err := filepath.EvalSymlinks(c.AuthHome)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"--unshare-all", "--share-net", "--die-with-parent", "--new-session", "--clearenv", "--setenv", "HOME", "/agent-auth", "--setenv", "PATH", "/usr/local/bin:/usr/bin:/bin", "--ro-bind", runtime, "/", "--proc", "/proc", "--dev", "/dev", "--tmpfs", "/tmp", "--bind", worktree, "/workspace", "--bind", auth, "/agent-auth", "--chdir", "/workspace", "--", "/usr/bin/worker", "--task", "one"}
	if !reflect.DeepEqual(cmd.Args[1:], want) {
		t.Fatalf("unexpected arguments: %q", cmd.Args[1:])
	}
	if strings.Contains(strings.Join(cmd.Args, " "), c.ControllerState) {
		t.Fatal("controller state path leaked into worker arguments")
	}
	if !reflect.DeepEqual(cmd.Env, []string{"PATH=/usr/bin:/bin"}) {
		t.Fatalf("ambient environment leaked: %q", cmd.Env)
	}
}

func TestRejectsOverlapsAndHostRoot(t *testing.T) {
	c := testConfig(t)
	c.AuthHome = c.ControllerState
	if _, err := c.Command(context.Background(), "/usr/bin/worker"); err == nil {
		t.Fatal("state mounted as auth home")
	}
	c = testConfig(t)
	c.RuntimeRoot = "/"
	if _, err := c.Command(context.Background(), "/usr/bin/worker"); err == nil {
		t.Fatal("host root mounted as runtime")
	}
	c = testConfig(t)
	c.ControllerState = filepath.Join(c.Worktree, "state")
	if err := os.Mkdir(c.ControllerState, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Command(context.Background(), "/usr/bin/worker"); err == nil {
		t.Fatal("worktree contains controller state")
	}
}

func TestRejectsSymlinkToState(t *testing.T) {
	c := testConfig(t)
	alias := filepath.Join(filepath.Dir(c.AuthHome), "state-alias")
	if err := os.Symlink(c.ControllerState, alias); err != nil {
		t.Fatal(err)
	}
	c.AuthHome = alias
	if _, err := c.Command(context.Background(), "/usr/bin/worker"); err == nil {
		t.Fatal("symlink alias bypassed overlap check")
	}
}

func TestRejectsSharedAuthHome(t *testing.T) {
	c := testConfig(t)
	if err := os.Chmod(c.AuthHome, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Command(context.Background(), "/usr/bin/worker"); err == nil {
		t.Fatal("world-readable auth home accepted")
	}
}

func TestRejectsProgramOutsideRuntime(t *testing.T) {
	c := testConfig(t)
	for _, program := range []string{"../worker", "/", "/usr/bin/missing", "/usr/bin/../bin/worker"} {
		if _, err := c.Command(context.Background(), program); err == nil {
			t.Errorf("accepted program %q", program)
		}
	}
}

func TestRejectsMissingOrSymlinkedMountPoint(t *testing.T) {
	c := testConfig(t)
	if err := os.Remove(filepath.Join(c.RuntimeRoot, "workspace")); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Command(context.Background(), "/usr/bin/worker"); err == nil {
		t.Fatal("missing rootfs mount point accepted")
	}
	if err := os.Symlink(c.ControllerState, filepath.Join(c.RuntimeRoot, "workspace")); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Command(context.Background(), "/usr/bin/worker"); err == nil {
		t.Fatal("symlinked rootfs mount point accepted")
	}
}

func TestRejectsBubblewrapFromWorkerWritablePath(t *testing.T) {
	c := testConfig(t)
	c.Bubblewrap = filepath.Join(c.Worktree, "bwrap")
	if err := os.WriteFile(c.Bubblewrap, []byte("test"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Command(context.Background(), "/usr/bin/worker"); err == nil {
		t.Fatal("worker-writable bubblewrap binary accepted")
	}
}

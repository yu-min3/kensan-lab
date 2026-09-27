// Package isolation builds a fail-closed Linux worker boundary. The controller
// never invokes a provider CLI directly: a future worker client must launch it
// inside this boundary and verify the boundary on the target host first.
package isolation

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Config names only the host paths a model worker may see. RuntimeRoot is a
// purpose-built, non-secret rootfs, never the host root. ControllerState is
// required so accidental overlap with the state/token directory is rejected.
type Config struct {
	Bubblewrap      string
	RuntimeRoot     string
	Worktree        string
	AuthHome        string
	ControllerState string
}

// Command returns a command whose filesystem is limited to a read-only
// runtime, a writable worktree, and the provider's dedicated auth home.
// Network is shared for subscription login; this does not provide egress
// isolation. No ambient environment or extra file descriptors are supplied.
func (c Config) Command(program string, args ...string) (*exec.Cmd, error) {
	if !strings.HasPrefix(program, "/") || filepath.Clean(program) != program || program == "/" {
		return nil, errors.New("worker program must be a clean absolute path inside runtime")
	}
	paths := []string{c.RuntimeRoot, c.Worktree, c.AuthHome, c.ControllerState}
	resolved := make([]string, len(paths))
	for i, path := range paths {
		if !filepath.IsAbs(path) || path == "/" {
			return nil, errors.New("all isolation paths must be absolute non-root directories")
		}
		canonical, err := filepath.EvalSymlinks(path)
		if err != nil {
			return nil, fmt.Errorf("resolve isolation path: %w", err)
		}
		info, err := os.Stat(canonical)
		if err != nil || !info.IsDir() {
			return nil, errors.New("isolation path must be a directory")
		}
		resolved[i] = canonical
	}
	if info, err := os.Stat(resolved[2]); err != nil || info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("provider auth home must be private")
	}
	for i := range resolved {
		for j := i + 1; j < len(resolved); j++ {
			if overlaps(resolved[i], resolved[j]) {
				return nil, errors.New("isolation paths overlap, possibly exposing controller state")
			}
		}
	}
	if _, err := os.Stat(filepath.Join(resolved[0], program)); err != nil {
		return nil, fmt.Errorf("worker program absent from runtime: %w", err)
	}
	if c.Bubblewrap == "" || !filepath.IsAbs(c.Bubblewrap) {
		return nil, errors.New("absolute bubblewrap binary required")
	}
	if info, err := os.Stat(c.Bubblewrap); err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
		return nil, errors.New("bubblewrap binary must exist and be executable")
	}
	argv := []string{
		"--unshare-all", "--share-net", "--die-with-parent", "--new-session",
		"--clearenv", "--setenv", "HOME", "/agent-auth",
		"--setenv", "PATH", "/usr/local/bin:/usr/bin:/bin",
		"--ro-bind", resolved[0], "/",
		"--proc", "/proc", "--dev", "/dev", "--tmpfs", "/tmp",
		"--bind", resolved[1], "/workspace",
		"--bind", resolved[2], "/agent-auth",
		"--chdir", "/workspace", "--", program,
	}
	argv = append(argv, args...)
	cmd := exec.Command(c.Bubblewrap, argv...)
	cmd.Env = []string{"PATH=/usr/bin:/bin"}
	return cmd, nil
}

func overlaps(a, b string) bool {
	if a == b {
		return true
	}
	rel, err := filepath.Rel(a, b)
	if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return true
	}
	rel, err = filepath.Rel(b, a)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

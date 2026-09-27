// Package isolation builds a fail-closed Linux worker boundary. The controller
// never invokes a provider CLI directly: a future worker client must launch it
// inside this boundary and verify the boundary on the target host first.
package isolation

import (
	"context"
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
	Bubblewrap       string
	RuntimeRoot      string
	Worktree         string
	ReadOnlyWorktree bool
	AuthHome         string
	ControllerState  string
}

// Command returns a command whose filesystem is limited to a read-only
// runtime, a writable worktree, and the provider's dedicated auth home.
// Network is shared for subscription login; this does not provide egress
// isolation. No ambient environment or extra file descriptors are supplied.
func (c Config) Command(ctx context.Context, program string, args ...string) (*exec.Cmd, error) {
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
	// Nested mounts cannot reliably create destinations below a read-only
	// root bind. The prepared rootfs must contain empty mount points already.
	for _, name := range []string{"workspace", "agent-auth", "proc", "dev", "tmp"} {
		info, err := os.Lstat(filepath.Join(resolved[0], name))
		if err != nil || !info.IsDir() {
			return nil, fmt.Errorf("worker rootfs is missing mount point %s", name)
		}
	}
	if _, err := os.Stat(filepath.Join(resolved[0], program)); err != nil {
		return nil, fmt.Errorf("worker program absent from runtime: %w", err)
	}
	if c.Bubblewrap == "" || !filepath.IsAbs(c.Bubblewrap) {
		return nil, errors.New("absolute bubblewrap binary required")
	}
	resolvedBwrap, err := filepath.EvalSymlinks(c.Bubblewrap)
	if err != nil || within(resolved[1], resolvedBwrap) || within(resolved[2], resolvedBwrap) || within(resolved[3], resolvedBwrap) {
		return nil, errors.New("bubblewrap binary must be outside writable worker mounts and controller state")
	}
	if info, err := os.Stat(resolvedBwrap); err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
		return nil, errors.New("bubblewrap binary must exist and be executable")
	}
	worktreeMount := "--bind"
	if c.ReadOnlyWorktree {
		worktreeMount = "--ro-bind"
	}
	argv := []string{
		"--unshare-all", "--share-net", "--die-with-parent", "--new-session",
		"--clearenv", "--setenv", "HOME", "/agent-auth",
		"--setenv", "PATH", "/usr/local/bin:/usr/bin:/bin",
		"--ro-bind", resolved[0], "/",
		"--proc", "/proc", "--dev", "/dev", "--tmpfs", "/tmp",
		worktreeMount, resolved[1], "/workspace",
		"--bind", resolved[2], "/agent-auth",
		"--chdir", "/workspace", "--", program,
	}
	argv = append(argv, args...)
	cmd := exec.CommandContext(ctx, resolvedBwrap, argv...)
	cmd.Env = []string{"PATH=/usr/bin:/bin"}
	return cmd, nil
}

// VerifierCommand runs repository tests without provider credentials or
// network. The worktree is read-only; tools may write only to private /tmp.
// The same path checks as a model worker still apply, but AuthHome is never
// mounted. The caller must supply a trusted, operator-owned test command.
func (c Config) VerifierCommand(ctx context.Context, cwd, program string, args ...string) (*exec.Cmd, error) {
	if cwd == "" || filepath.IsAbs(cwd) || cwd == ".." || strings.HasPrefix(filepath.Clean(cwd), ".."+string(filepath.Separator)) {
		return nil, errors.New("verifier working directory must stay in task checkout")
	}
	if _, err := c.Command(ctx, program, args...); err != nil {
		return nil, err
	}
	runtimeRoot, _ := filepath.EvalSymlinks(c.RuntimeRoot)
	worktree, _ := filepath.EvalSymlinks(c.Worktree)
	bwrap, _ := filepath.EvalSymlinks(c.Bubblewrap)
	if entries, err := os.ReadDir(filepath.Join(runtimeRoot, "agent-auth")); err != nil || len(entries) != 0 {
		return nil, errors.New("verifier rootfs auth mount point must be empty")
	}
	argv := []string{
		"--unshare-all", "--die-with-parent", "--new-session", "--clearenv",
		"--setenv", "HOME", "/tmp",
		"--setenv", "PATH", "/usr/local/bin:/usr/bin:/bin",
		"--setenv", "GOCACHE", "/tmp/go-cache",
		"--setenv", "GOMODCACHE", "/tmp/go-mod",
		"--setenv", "GOPROXY", "off",
		"--setenv", "npm_config_offline", "true",
		"--setenv", "GIT_CONFIG_GLOBAL", "/dev/null",
		"--setenv", "GIT_CONFIG_NOSYSTEM", "1",
		"--setenv", "GIT_TERMINAL_PROMPT", "0",
		"--ro-bind", runtimeRoot, "/",
		"--proc", "/proc", "--dev", "/dev", "--tmpfs", "/tmp",
		"--ro-bind", worktree, "/workspace",
		"--chdir", filepath.Join("/workspace", cwd), "--", program,
	}
	argv = append(argv, args...)
	cmd := exec.CommandContext(ctx, bwrap, argv...)
	cmd.Env = []string{"PATH=/usr/bin:/bin"}
	return cmd, nil
}

func within(root, path string) bool {
	if root == path {
		return true
	}
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
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

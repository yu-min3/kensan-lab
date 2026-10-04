// Package committransfer transports fixed Git objects without executing Git in
// worker-controlled repositories or copying their configuration and hooks.
package committransfer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const MaxBundleBytes = 64 << 20
const MaxObjectBytes = 256 << 20

var taskPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)
var shaPattern = regexp.MustCompile(`^[a-f0-9]{40}$`)
var objectPattern = regexp.MustCompile(`^(?:[a-f0-9]{2}/[a-f0-9]{38}|pack/pack-[a-f0-9]{40}\.(?:pack|idx|rev))$`)

type Bundle struct {
	TaskID  string `json:"task_id"`
	BaseSHA string `json:"base_sha"`
	HeadSHA string `json:"head_sha"`
	SHA256  string `json:"sha256"`
	Data    []byte `json:"data"`
}

func bundleRef(task, head string) string { return "refs/heads/sense-transfer/" + task + "/" + head }
func (b Bundle) Validate() error {
	h := sha256.Sum256(b.Data)
	if !taskPattern.MatchString(b.TaskID) || !shaPattern.MatchString(b.BaseSHA) || b.BaseSHA == b.HeadSHA || !shaPattern.MatchString(b.HeadSHA) || len(b.Data) == 0 || len(b.Data) > MaxBundleBytes || b.SHA256 != hex.EncodeToString(h[:]) {
		return errors.New("invalid fixed commit bundle")
	}
	return nil
}
func git(ctx context.Context, repo string, args ...string) ([]byte, error) {
	prefix := []string{"-c", "core.hooksPath=/dev/null", "-c", "credential.helper=", "-c", "protocol.allow=never", "-c", "protocol.file.allow=always", "-c", "core.fsmonitor=false", "-c", "core.useReplaceRefs=false", "-c", "transfer.fsckObjects=true", "-C", repo}
	cmd := exec.CommandContext(ctx, "git", append(prefix, args...)...)
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=/nonexistent", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0", "GIT_NO_REPLACE_OBJECTS=1"}
	out, err := cmd.Output()
	if err != nil {
		return nil, errors.New("fixed commit Git validation failed")
	}
	return out, nil
}
func canonicalDir(path string) error {
	info, err := os.Lstat(path)
	actual, e := filepath.EvalSymlinks(path)
	if err != nil || e != nil || !filepath.IsAbs(path) || actual != path || !info.IsDir() {
		return errors.New("canonical repository directory required")
	}
	return nil
}
func quarantine() (string, func(), error) {
	p, err := os.MkdirTemp("", "sense-commit-transfer-")
	if err != nil {
		return "", nil, err
	}
	return p, func() { os.RemoveAll(p) }, nil
}
func initBare(ctx context.Context, root string) (string, error) {
	repo := filepath.Join(root, "repo.git")
	_, err := git(ctx, root, "init", "--bare", "--quiet", repo)
	return repo, err
}

// Create reads only regular object files. Repository config, hooks, alternates,
// replace refs, paths and credential helpers never reach an executing process.
func Create(ctx context.Context, source, task, base, head string) (Bundle, error) {
	if !taskPattern.MatchString(task) || !shaPattern.MatchString(base) || !shaPattern.MatchString(head) {
		return Bundle{}, errors.New("fixed task and commit required")
	}
	if err := canonicalDir(source); err != nil {
		return Bundle{}, err
	}
	objects := filepath.Join(source, ".git", "objects")
	if err := canonicalDir(filepath.Join(source, ".git")); err != nil {
		return Bundle{}, err
	}
	if err := canonicalDir(objects); err != nil {
		return Bundle{}, err
	}
	root, cleanup, err := quarantine()
	if err != nil {
		return Bundle{}, err
	}
	defer cleanup()
	repo, err := initBare(ctx, root)
	if err != nil {
		return Bundle{}, err
	}
	sourceRoot, err := os.OpenRoot(source)
	if err != nil {
		return Bundle{}, err
	}
	defer sourceRoot.Close()
	objectRoot, err := sourceRoot.OpenRoot(".git/objects")
	if err != nil {
		return Bundle{}, errors.New("object root escapes task checkout")
	}
	defer objectRoot.Close()
	if err := copyObjectsAt(ctx, objectRoot, objects, repo); err != nil {
		return Bundle{}, err
	}

	return createIsolated(ctx, repo, root, task, base, head)
}
func createIsolated(ctx context.Context, repo, root, task, base, head string) (Bundle, error) {
	out, err := git(ctx, repo, "rev-parse", "--verify", head+"^{commit}")
	if err != nil || strings.TrimSpace(string(out)) != head {
		return Bundle{}, errors.New("fixed commit missing")
	}
	if _, err := git(ctx, repo, "merge-base", "--is-ancestor", base, head); err != nil {
		return Bundle{}, errors.New("fixed base is not ancestor")
	}
	if _, err := git(ctx, repo, "fsck", "--strict", "--no-dangling", "--no-reflogs", head); err != nil {
		return Bundle{}, err
	}
	if _, err := git(ctx, repo, "update-ref", bundleRef(task, head), head); err != nil {
		return Bundle{}, err
	}
	path := filepath.Join(root, "objects.bundle")
	if _, err := git(ctx, repo, "bundle", "create", path, base+".."+bundleRef(task, head)); err != nil {
		return Bundle{}, err
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() > MaxBundleBytes {
		return Bundle{}, errors.New("bundle exceeds transfer limit")
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return Bundle{}, err
	}
	h := sha256.Sum256(body)
	return Bundle{TaskID: task, BaseSHA: base, HeadSHA: head, SHA256: hex.EncodeToString(h[:]), Data: body}, nil
}

// Import verifies an incremental bundle in quarantine before importing objects
// and one host-owned ref. It never checks out files or changes main/HEAD.
func Import(ctx context.Context, trusted string, b Bundle) error {
	if err := b.Validate(); err != nil {
		return err
	}
	if err := canonicalDir(trusted); err != nil {
		return err
	}
	separator := bytes.Index(b.Data, []byte("\n\n"))
	if separator < 0 || separator > 8192 {
		return errors.New("invalid bundle header")
	}
	lines := strings.Split(string(b.Data[:separator]), "\n")
	if len(lines) != 3 || lines[0] != "# v2 git bundle" || !strings.HasPrefix(lines[1], "-"+b.BaseSHA+" ") || lines[2] != b.HeadSHA+" "+bundleRef(b.TaskID, b.HeadSHA) {
		return errors.New("bundle prerequisite differs from fixed base")
	}
	root, cleanup, err := quarantine()
	if err != nil {
		return err
	}
	defer cleanup()
	repo, err := initBare(ctx, root)
	if err != nil {
		return err
	}
	objectPath, err := git(ctx, trusted, "rev-parse", "--path-format=absolute", "--git-path", "objects")
	if err != nil {
		return errors.New("trusted object storage unavailable")
	}
	objects := strings.TrimSpace(string(objectPath))
	if err := canonicalDir(objects); err != nil {
		return err
	}
	if err := copyObjects(ctx, objects, repo); err != nil {
		return err
	}
	if _, err := git(ctx, repo, "cat-file", "-e", b.BaseSHA+"^{commit}"); err != nil {
		return errors.New("trusted base commit missing")
	}
	path := filepath.Join(root, "received.bundle")
	if err := os.WriteFile(path, b.Data, 0600); err != nil {
		return err
	}
	refs, err := git(ctx, repo, "bundle", "list-heads", path)
	if err != nil || !bytes.Equal(refs, []byte(b.HeadSHA+" "+bundleRef(b.TaskID, b.HeadSHA)+"\n")) {
		return errors.New("bundle references differ from fixed commit")
	}
	if _, err := git(ctx, repo, "bundle", "verify", path); err != nil {
		return err
	}
	if _, err := git(ctx, repo, "fetch", "--no-tags", path, bundleRef(b.TaskID, b.HeadSHA)+":"+bundleRef(b.TaskID, b.HeadSHA)); err != nil {
		return err
	}
	if _, err := git(ctx, repo, "merge-base", "--is-ancestor", b.BaseSHA, b.HeadSHA); err != nil {
		return errors.New("bundle base differs from graph")
	}
	if _, err := git(ctx, repo, "fsck", "--strict", "--no-dangling", "--no-reflogs", b.HeadSHA); err != nil {
		return err
	}
	_, err = git(ctx, trusted, "fetch", "--no-tags", "--no-write-fetch-head", path, bundleRef(b.TaskID, b.HeadSHA)+":"+bundleRef(b.TaskID, b.HeadSHA))
	return err
}

// FetchMerged runs only in the credentialed publisher process, in a clean bare
// repository. The URL, commit and askpass executable are host-owned inputs.
func FetchMerged(ctx context.Context, task, base, revision, askpass string) (Bundle, error) {
	if !taskPattern.MatchString(task) || !shaPattern.MatchString(base) || !shaPattern.MatchString(revision) {
		return Bundle{}, errors.New("fixed merged revision required")
	}
	info, err := os.Lstat(askpass)
	canonical, canonicalErr := filepath.EvalSymlinks(askpass)
	if canonicalErr != nil || canonical != askpass || err != nil || !filepath.IsAbs(askpass) || !info.Mode().IsRegular() || info.Mode().Perm()&0022 != 0 || info.Mode().Perm()&0111 == 0 {
		return Bundle{}, errors.New("trusted askpass required")
	}
	root, cleanup, err := quarantine()
	if err != nil {
		return Bundle{}, err
	}
	defer cleanup()
	repo, err := initBare(ctx, root)
	if err != nil {
		return Bundle{}, err
	}
	fetchCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := exec.CommandContext(fetchCtx, "git", "-c", "http.followRedirects=false", "-c", "http.proxy=", "-c", "http.sslVerify=true", "-c", "core.hooksPath=/dev/null", "-c", "credential.helper=", "-c", "protocol.allow=never", "-c", "protocol.https.allow=always", "-c", "transfer.fsckObjects=true", "-C", repo, "fetch", "--no-tags", "https://github.com/yu-min3/kensan-lab.git", revision)
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=/nonexistent", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=" + askpass}
	if err := cmd.Start(); err != nil {
		return Bundle{}, errors.New("verified merged graph unavailable")
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case err := <-done:
			if err != nil {
				return Bundle{}, errors.New("verified merged graph unavailable")
			}
			if objectStorageSize(filepath.Join(repo, "objects")) > MaxObjectBytes {
				return Bundle{}, errors.New("merged graph exceeds transfer limit")
			}
			return createIsolated(ctx, repo, root, task, base, revision)
		case <-ticker.C:
			if objectStorageSize(filepath.Join(repo, "objects")) > MaxObjectBytes {
				cancel()
				<-done
				return Bundle{}, errors.New("merged graph exceeds transfer limit")
			}
		case <-ctx.Done():
			cancel()
			<-done
			return Bundle{}, ctx.Err()
		}
	}

}

func copyObjects(ctx context.Context, objects, repo string) error {
	objectRoot, err := os.OpenRoot(objects)
	if err != nil {
		return errors.New("object root unavailable")
	}
	defer objectRoot.Close()
	return copyObjectsAt(ctx, objectRoot, objects, repo)
}
func copyObjectsAt(ctx context.Context, objectRoot *os.Root, objects, repo string) error {
	var total int64
	err := filepath.WalkDir(objects, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return errors.New("object storage unavailable")
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		rel, _ := filepath.Rel(objects, path)
		if rel == "." {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("indirect object storage rejected")
		}
		if entry.IsDir() {
			if rel != "pack" && rel != "info" && !regexp.MustCompile(`^[a-f0-9]{2}$`).MatchString(rel) {
				return errors.New("unexpected object directory")
			}
			return nil
		}
		if !objectPattern.MatchString(filepath.ToSlash(rel)) {
			return errors.New("non-object storage rejected")
		}
		file, err := objectRoot.Open(rel)
		if err != nil {
			return errors.New("object unavailable")
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() {
			return errors.New("regular object required")
		}
		total += info.Size()
		if total > MaxObjectBytes {
			return errors.New("object graph exceeds transfer limit")
		}
		dest := filepath.Join(repo, "objects", rel)
		if err := os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
			return err
		}
		out, err := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		n, err := io.Copy(out, io.LimitReader(file, info.Size()+1))
		closeErr := out.Close()
		if err != nil || closeErr != nil || n != info.Size() {
			return errors.New("object changed during capture")
		}
		return nil
	})
	return err
}

// Contains checks only a fixed commit in an operator-owned repository.
func Contains(ctx context.Context, trusted, head string) bool {
	if !shaPattern.MatchString(head) || canonicalDir(trusted) != nil {
		return false
	}
	out, err := git(ctx, trusted, "rev-parse", "--verify", head+"^{commit}")
	return err == nil && strings.TrimSpace(string(out)) == head
}

func objectStorageSize(path string) int64 {
	var total int64
	err := filepath.WalkDir(path, func(_ string, e os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() {
			return nil
		}
		info, err := e.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		if total > MaxObjectBytes {
			return errors.New("over limit")
		}
		return nil
	})
	if err != nil && total <= MaxObjectBytes {
		return MaxObjectBytes + 1
	}
	return total
}

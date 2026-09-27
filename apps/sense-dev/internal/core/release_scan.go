package core

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const maxGitOutput = 16 << 20

type limitedBuffer struct{ bytes.Buffer }

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > maxGitOutput {
		return 0, errors.New("Git evidence exceeds scan limit")
	}
	return b.Buffer.Write(p)
}

func gitEvidence(ctx context.Context, repo string, args ...string) ([]byte, error) {
	safeArgs := append([]string{"--no-optional-locks", "-c", "core.fsmonitor=false", "-c", "diff.renames=false"}, args...)
	cmd := exec.CommandContext(ctx, "git", safeArgs...)
	cmd.Dir = repo
	cmd.Env = []string{"PATH=/usr/bin:/bin:/usr/local/bin:/opt/homebrew/bin", "HOME=/nonexistent", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_NO_REPLACE_OBJECTS=1"}
	var out limitedBuffer
	cmd.Stdout, cmd.Stderr = &out, io.Discard
	if err := cmd.Run(); err != nil {
		return nil, errors.New("Git evidence command failed or exceeded limit")
	}
	return out.Bytes(), nil
}

func fullSHA(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	for _, c := range value {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return true
}

var credentialPattern = regexp.MustCompile(`(?im)(-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----|github_pat_[a-z0-9_]{12,}|ghp_[a-z0-9]{12,}|AKIA[0-9A-Z]{16}|(?:ANTHROPIC_API_KEY|OPENAI_API_KEY|AWS_SECRET_ACCESS_KEY)\s*[:=]\s*["']?[^\s"']{8,})`)
var publicRoutePattern = regexp.MustCompile(`(?i)(trycloudflare|cloudflared|ngrok|tailscale\s+funnel|kind:\s*(?:Ingress|Gateway|HTTPRoute)|type:\s*(?:NodePort|LoadBalancer)|github\.io|pages\.github\.com)`)

func sensitivePath(path string) bool {
	p := strings.ToLower(path)
	return strings.Contains(p, "/.env") || strings.HasPrefix(p, ".env") || strings.Contains(p, "/secrets/") || strings.HasPrefix(p, "secrets/") || strings.HasPrefix(p, "daily/") || strings.HasPrefix(p, "private/") || strings.HasPrefix(p, "attachments/")
}

func highRiskPath(path string) bool {
	p := strings.ToLower(path)
	return strings.HasPrefix(p, ".github/workflows/") || strings.HasPrefix(p, "clusters/") || strings.HasPrefix(p, "infra/") || strings.HasPrefix(p, "gitops/") || strings.Contains(p, "cloudflared") || strings.Contains(p, "tunnel") || strings.Contains(p, "ingress") || strings.Contains(p, "gateway") || strings.Contains(p, "kustomization") || strings.HasPrefix(p, "apps/sense-dev/deploy/") || strings.HasPrefix(p, "apps/sense-dev/internal/core/release") || strings.Contains(p, "/publisher")
}

// ScanGitRange inspects every commit proposed for transfer, not only the final
// tree. A candidate is necessary but never sufficient for a release: CI,
// visibility, rendered manifests and the independent agent still need review.
func ScanGitRange(repoPath, baseSHA, headSHA, operation, ref string) (ReleaseScan, error) {
	if !fullSHA(baseSHA) || !fullSHA(headSHA) || !allowedOperations[operation] || !strings.HasPrefix(ref, "refs/heads/") {
		return ReleaseScan{}, errors.New("invalid release scan target")
	}
	root, err := filepath.Abs(repoPath)
	if err != nil {
		return ReleaseScan{}, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return ReleaseScan{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	top, err := gitEvidence(ctx, root, "rev-parse", "--show-toplevel")
	if err != nil || strings.TrimSpace(string(top)) != root {
		return ReleaseScan{}, errors.New("scan path must be the Git worktree root")
	}
	head, err := gitEvidence(ctx, root, "rev-parse", "HEAD")
	if err != nil || strings.TrimSpace(string(head)) != headSHA {
		return ReleaseScan{}, errors.New("worktree HEAD changed before scan")
	}
	if _, err := gitEvidence(ctx, root, "merge-base", "--is-ancestor", baseSHA, headSHA); err != nil {
		return ReleaseScan{}, errors.New("base is not an ancestor of head")
	}
	status, err := gitEvidence(ctx, root, "status", "--porcelain=v1", "--untracked-files=all")
	if err != nil || len(status) != 0 {
		return ReleaseScan{}, errors.New("worktree must be clean before release scan")
	}
	commitsRaw, err := gitEvidence(ctx, root, "rev-list", "--reverse", baseSHA+".."+headSHA)
	if err != nil {
		return ReleaseScan{}, err
	}
	commits := strings.Fields(string(commitsRaw))
	if len(commits) == 0 || len(commits) > 100 {
		return ReleaseScan{}, errors.New("scan requires 1 to 100 proposed commits")
	}
	findings := map[string]bool{}
	paths := map[string]bool{}
	hash := sha256.New()
	for _, commit := range commits {
		if !fullSHA(commit) {
			return ReleaseScan{}, errors.New("invalid commit in range")
		}
		parents, err := gitEvidence(ctx, root, "rev-list", "--parents", "-n", "1", commit)
		if err != nil {
			return ReleaseScan{}, err
		}
		if len(strings.Fields(string(parents))) != 2 {
			findings["merge or root commit needs manual review"] = true
		}
		changed, err := gitEvidence(ctx, root, "diff-tree", "--no-renames", "--no-commit-id", "--name-only", "-r", "-z", commit)
		if err != nil {
			return ReleaseScan{}, err
		}
		for _, raw := range bytes.Split(changed, []byte{0}) {
			if len(raw) == 0 {
				continue
			}
			path := string(raw)
			paths[path] = true
			if sensitivePath(path) {
				findings["sensitive path: "+path] = true
			}
			if highRiskPath(path) {
				findings["publication or policy path needs review: "+path] = true
			}
		}
		patch, err := gitEvidence(ctx, root, "show", "--no-renames", "--format=fuller", "--binary", "--no-ext-diff", commit)
		if err != nil {
			return ReleaseScan{}, err
		}
		_, _ = hash.Write([]byte(commit))
		_, _ = hash.Write(patch)
		if credentialPattern.Match(patch) {
			findings["credential-like content in proposed commit"] = true
		}
		if publicRoutePattern.Match(patch) {
			findings["potential public route or publication in proposed commit"] = true
		}
		if bytes.Contains(patch, []byte("GIT binary patch")) || bytes.Contains(patch, []byte("Binary files ")) {
			findings["binary content needs manual review"] = true
		}
	}
	report := ReleaseScan{BaseSHA: baseSHA, HeadSHA: headSHA, Repository: "yu-min3/kensan-lab", Ref: ref, Operation: operation, PolicyVersion: ReleasePolicyVersion, CommitCount: len(commits), DiffSHA256: hex.EncodeToString(hash.Sum(nil)), ScannedAt: time.Now().UTC()}
	for path := range paths {
		report.ChangedPaths = append(report.ChangedPaths, path)
	}
	for finding := range findings {
		report.Findings = append(report.Findings, finding)
	}
	sort.Strings(report.ChangedPaths)
	sort.Strings(report.Findings)
	if len(report.Findings) == 0 {
		report.Status = "candidate"
	} else {
		report.Status = "needs_human"
	}
	return report, nil
}

func (s *Store) ScanAndRecordRelease(repoPath, baseSHA, headSHA, operation, ref string) (ReleaseScan, ArtifactRef, error) {
	scan, err := ScanGitRange(repoPath, baseSHA, headSHA, operation, ref)
	if err != nil {
		return ReleaseScan{}, ArtifactRef{}, err
	}
	refResult, err := s.recordReleaseScan(scan)
	return scan, refResult, err
}

func (s *Store) recordReleaseScan(scan ReleaseScan) (ArtifactRef, error) {
	if !fullSHA(scan.BaseSHA) || !fullSHA(scan.HeadSHA) || scan.Repository != "yu-min3/kensan-lab" || scan.PolicyVersion != ReleasePolicyVersion || scan.DiffSHA256 == "" || scan.CommitCount < 1 || scan.ScannedAt.IsZero() {
		return ArtifactRef{}, errors.New("incomplete release scan")
	}
	b, err := json.Marshal(scan)
	if err != nil {
		return ArtifactRef{}, err
	}
	a, err := s.PutArtifact("system", "release_scan", b)
	if err != nil {
		return ArtifactRef{}, err
	}
	return artifactRef(a), nil
}

func (s *Store) checkReleaseScan(ref ArtifactRef, decision ReleaseDecision) (ReleaseScan, error) {
	if err := s.verifyRef(ref); err != nil {
		return ReleaseScan{}, err
	}
	st := s.Snapshot()
	a := st.Artifacts[ref.ID]
	if a.AgentID != "system" || a.Kind != "release_scan" {
		return ReleaseScan{}, errors.New("release scan must be controller-owned")
	}
	b, err := s.ReadArtifact(ref.ID)
	if err != nil {
		return ReleaseScan{}, err
	}
	var scan ReleaseScan
	if err := json.Unmarshal(b, &scan); err != nil {
		return ReleaseScan{}, errors.New("invalid release scan artifact")
	}
	if scan.Status != "candidate" || len(scan.Findings) != 0 || scan.Repository != decision.Repository || scan.Ref != decision.Ref || scan.Operation != decision.Operation || scan.HeadSHA != decision.HeadSHA || scan.PolicyVersion != decision.PolicyVersion || scan.DiffSHA256 == "" || scan.CommitCount < 1 || time.Since(scan.ScannedAt) > time.Hour || scan.ScannedAt.After(time.Now().Add(time.Minute)) {
		return ReleaseScan{}, errors.New("release scan is stale, risky or targets another operation")
	}
	return scan, nil
}

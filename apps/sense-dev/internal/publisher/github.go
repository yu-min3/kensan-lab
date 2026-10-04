package publisher

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/core"
)

const repository = "yu-min3/kensan-lab"

var commitSHA = regexp.MustCompile(`^[0-9a-f]{40}$`)

// GitHub has no controller or worker credential. It runs only in the separate
// publisher process with a repo-scoped token file and a dedicated askpass.
type GitHub struct {
	RepoPath    string
	TokenFile   string
	Askpass     string
	Client      *http.Client
	API         string // test override; production must use api.github.com
	RegistryAPI string // loopback-only test override; production uses ghcr.io
}

func (g GitHub) validate(i core.PublishIntent) (string, error) {
	if i.Repository != repository || !strings.HasPrefix(i.Ref, "refs/heads/") || i.Ref == "refs/heads/main" || i.Ref == "refs/heads/master" || !commitSHA.MatchString(i.HeadSHA) || i.Operation != "branch_push" && i.Operation != "pr_create" && i.Operation != "merge" && i.Operation != "deploy" && i.Operation != "image_publish" {
		return "", errors.New("publish target outside limited GitHub scope")
	}
	if i.Operation == "image_publish" && (i.ImageRelease == nil || i.TargetEnvironment != "private-canary" || i.PolicyVersion != core.ReleasePolicyVersion || i.ExpiresAt.IsZero() || i.ImageRelease.SourceSHA != i.HeadSHA || core.ValidateImageReleaseSpec(*i.ImageRelease) != nil) {
		return "", errors.New("image publication outside fixed private source/workflow scope")
	}
	if i.Operation != "image_publish" && i.ImageRelease != nil {
		return "", errors.New("image spec supplied for another publisher operation")
	}
	if (i.Operation == "merge" || i.Operation == "deploy") && (i.TargetEnvironment != "private-canary" || i.PolicyVersion != core.ReleasePolicyVersion || !commitSHA.MatchString(i.BaseSHA) || i.BaseSHA == i.HeadSHA || i.ExpiresAt.IsZero()) {
		return "", errors.New("GitOps merge needs fixed private environment, policy and base SHA")
	}
	if i.TargetEnvironment == "private-canary" && (i.PolicyVersion != core.ReleasePolicyVersion || i.ExpiresAt.IsZero()) {
		return "", errors.New("private canary publication needs fixed live policy identity")
	}
	branch := strings.TrimPrefix(i.Ref, "refs/heads/")
	if branch == "" || strings.Contains(branch, "..") || strings.HasPrefix(branch, "/") || strings.HasSuffix(branch, "/") {
		return "", errors.New("invalid branch")
	}
	if g.API != "" && g.API != "https://api.github.com" && !strings.HasPrefix(g.API, "http://127.0.0.1:") {
		return "", errors.New("non-GitHub API endpoint forbidden")
	}
	return branch, nil
}

func (g GitHub) endpoint() string {
	if g.API != "" {
		return strings.TrimRight(g.API, "/")
	}
	return "https://api.github.com"
}

func (g GitHub) request(ctx context.Context, method, path string, body any, out any) (int, error) {
	info, err := os.Lstat(g.TokenFile)
	if err != nil || info.Mode().Perm()&0077 != 0 || !info.Mode().IsRegular() {
		return 0, errors.New("publisher token file must be private and regular")
	}
	tokenBytes, err := os.ReadFile(g.TokenFile)
	if err != nil {
		return 0, err
	}
	token := strings.TrimSpace(string(tokenBytes))
	if token == "" {
		return 0, errors.New("empty publisher token")
	}
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return 0, err
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, g.endpoint()+path, reader)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := g.Client
	if client == nil {
		client = &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("GitHub API redirect rejected") }}
	}
	response, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return response.StatusCode, fmt.Errorf("GitHub API returned %d", response.StatusCode)
	}
	if out != nil {
		if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(out); err != nil {
			return response.StatusCode, err
		}
	}
	return response.StatusCode, nil
}

func (g GitHub) remoteHead(ctx context.Context, branch string) (string, bool, error) {
	var result struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	status, err := g.request(ctx, http.MethodGet, "/repos/"+repository+"/git/ref/heads/"+url.PathEscape(branch), nil, &result)
	if status == http.StatusNotFound {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return result.Object.SHA, true, nil
}

func (g GitHub) Inspect(ctx context.Context, i core.PublishIntent) (string, bool, error) {
	branch, err := g.validate(i)
	if err != nil {
		return "", false, err
	}
	if i.Operation == "merge" || i.Operation == "deploy" {
		return g.inspectMerge(ctx, i, branch)
	}
	if i.Operation == "image_publish" {
		return g.inspectImage(ctx, i)
	}
	sha, exists, err := g.remoteHead(ctx, branch)
	if err != nil {
		return "", false, err
	}
	if exists && sha != i.HeadSHA {
		return "", false, errors.New("remote branch has another SHA")
	}
	if i.Operation == "branch_push" {
		return sha, exists, nil
	}
	if !exists {
		return "", false, errors.New("PR branch was not pushed at approved SHA")
	}
	var pulls []struct {
		Number int `json:"number"`
		Head   struct {
			SHA string `json:"sha"`
		} `json:"head"`
	}
	_, err = g.request(ctx, http.MethodGet, "/repos/"+repository+"/pulls?state=all&head=yu-min3:"+url.QueryEscape(branch), nil, &pulls)
	if err != nil {
		return "", false, err
	}
	if len(pulls) > 1 {
		return "", false, errors.New("multiple PRs for approved branch")
	}
	if len(pulls) == 0 {
		return "", false, nil
	}
	if pulls[0].Head.SHA != i.HeadSHA {
		return "", false, errors.New("PR head has another SHA")
	}
	return fmt.Sprintf("%d", pulls[0].Number), true, nil
}

func (g GitHub) Execute(ctx context.Context, i core.PublishIntent) (string, error) {
	branch, err := g.validate(i)
	if err != nil {
		return "", err
	}
	if !i.ExpiresAt.IsZero() && !time.Now().Before(i.ExpiresAt) {
		return "", errors.New("publisher authorization expired")
	}
	if i.Operation == "merge" || i.Operation == "deploy" {
		return g.executeMerge(ctx, i, branch)
	}
	if i.Operation == "image_publish" {
		return g.executeImage(ctx, i, branch)
	}
	if i.Operation == "pr_create" {
		if strings.TrimSpace(i.PullRequestSummary) == "" {
			return "", errors.New("approved PR summary required")
		}
		sha, exists, err := g.remoteHead(ctx, branch)
		if err != nil || !exists || sha != i.HeadSHA {
			return "", errors.New("approved branch SHA must exist before PR")
		}
		body := i.PullRequestSummary + "\n\nRelease decision: " + i.DecisionID + "\nHead SHA: " + i.HeadSHA
		title := strings.SplitN(strings.TrimSpace(i.PullRequestSummary), "\n", 2)[0]
		if len(title) > 120 {
			title = title[:120]
		}
		var result struct {
			Number int `json:"number"`
		}
		if !i.ExpiresAt.IsZero() && !time.Now().Before(i.ExpiresAt) {
			return "", errors.New("publisher authorization expired before PR creation")
		}
		if !i.ExpiresAt.IsZero() && !time.Now().Before(i.ExpiresAt) {
			return "", errors.New("publisher authorization expired before PR creation")
		}
		_, err = g.request(ctx, http.MethodPost, "/repos/"+repository+"/pulls", map[string]any{"title": title, "body": body, "head": branch, "base": "main", "draft": i.TargetEnvironment != "private-canary"}, &result)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%d", result.Number), nil
	}
	if g.RepoPath == "" || !filepath.IsAbs(g.RepoPath) || g.Askpass == "" || !filepath.IsAbs(g.Askpass) {
		return "", errors.New("absolute trusted repo and askpass paths required")
	}
	remote, err := exec.CommandContext(ctx, "git", "-C", g.RepoPath, "remote", "get-url", "origin").Output()
	if err != nil || strings.TrimSpace(string(remote)) != "https://github.com/yu-min3/kensan-lab.git" {
		return "", errors.New("origin is not the approved HTTPS repository")
	}
	commit, err := exec.CommandContext(ctx, "git", "-C", g.RepoPath, "rev-parse", "--verify", i.HeadSHA+"^{commit}").Output()
	if err != nil || strings.TrimSpace(string(commit)) != i.HeadSHA {
		return "", errors.New("approved commit not present locally")
	}
	cmd := exec.CommandContext(ctx, "git", "-c", "credential.helper=", "-C", g.RepoPath, "push", "--porcelain", "origin", i.HeadSHA+":"+i.Ref)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS="+g.Askpass, "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	if !i.ExpiresAt.IsZero() && !time.Now().Before(i.ExpiresAt) {
		return "", errors.New("publisher authorization expired before branch push")
	}
	if !i.ExpiresAt.IsZero() && !time.Now().Before(i.ExpiresAt) {
		return "", errors.New("publisher authorization expired before branch push")
	}
	if err := cmd.Run(); err != nil {
		return "", errors.New("approved branch push failed; inspect remote before retry")
	}
	return i.HeadSHA, nil
}

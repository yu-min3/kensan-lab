package publisher

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/core"
)

const canaryRequiredCheck = "canary (locked tests and runtime image)"

type pullRequest struct {
	Number         int    `json:"number"`
	State          string `json:"state"`
	Draft          bool   `json:"draft"`
	Merged         bool   `json:"merged"`
	MergeSHA       string `json:"merge_commit_sha"`
	Mergeable      *bool  `json:"mergeable"`
	MergeableState string `json:"mergeable_state"`
	Head           struct {
		SHA  string `json:"sha"`
		Ref  string `json:"ref"`
		Repo struct {
			FullName string `json:"full_name"`
		} `json:"repo"`
	} `json:"head"`
	Base struct {
		SHA  string `json:"sha"`
		Ref  string `json:"ref"`
		Repo struct {
			FullName string `json:"full_name"`
		} `json:"repo"`
	} `json:"base"`
}

func (g GitHub) fixedPull(ctx context.Context, i core.PublishIntent, branch string) (pullRequest, error) {
	var list []struct {
		Number int `json:"number"`
	}
	_, err := g.request(ctx, http.MethodGet, "/repos/"+repository+"/pulls?state=all&head=yu-min3:"+url.QueryEscape(branch)+"&per_page=100", nil, &list)
	if err != nil || len(list) != 1 || list[0].Number < 1 {
		return pullRequest{}, errors.New("exactly one existing PR required for GitOps merge")
	}
	var p pullRequest
	_, err = g.request(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/pulls/%d", repository, list[0].Number), nil, &p)
	if err != nil {
		return pullRequest{}, err
	}
	if p.Number != list[0].Number || p.Head.SHA != i.HeadSHA || p.Head.Ref != branch || p.Head.Repo.FullName != repository || p.Base.Ref != "main" || p.Base.Repo.FullName != repository || p.Draft {
		return pullRequest{}, errors.New("PR identity, head, base or draft differs from fixed private merge")
	}
	return p, nil
}

// strict required checks close the race where main advances after inspection.
// The GitHub merge endpoint must enforce up-to-date checks even for admins.
func (g GitHub) requiredChecks(ctx context.Context, head string) error {
	var protection struct {
		EnforceAdmins struct {
			Enabled bool `json:"enabled"`
		} `json:"enforce_admins"`
		RequiredStatusChecks *struct {
			Strict   bool     `json:"strict"`
			Contexts []string `json:"contexts"`
			Checks   []struct {
				Context string `json:"context"`
				AppID   int    `json:"app_id"`
			} `json:"checks"`
		} `json:"required_status_checks"`
	}
	if _, err := g.request(ctx, http.MethodGet, "/repos/"+repository+"/branches/main/protection", nil, &protection); err != nil {
		return errors.New("main branch protection cannot be verified")
	}
	if !protection.EnforceAdmins.Enabled || protection.RequiredStatusChecks == nil || !protection.RequiredStatusChecks.Strict {
		return errors.New("main requires strict CI protection enforced for admins")
	}
	required := map[string]int{}
	for _, name := range protection.RequiredStatusChecks.Contexts {
		required[name] = 0
	}
	for _, check := range protection.RequiredStatusChecks.Checks {
		required[check.Context] = check.AppID
	}
	if _, ok := required[canaryRequiredCheck]; !ok {
		return errors.New("fixed canary CI must be a protected required check")
	}
	var checks struct {
		TotalCount int `json:"total_count"`
		CheckRuns  []struct {
			Name       string `json:"name"`
			HeadSHA    string `json:"head_sha"`
			Status     string `json:"status"`
			Conclusion string `json:"conclusion"`
			App        struct {
				ID   int    `json:"id"`
				Slug string `json:"slug"`
			} `json:"app"`
		} `json:"check_runs"`
	}
	if _, err := g.request(ctx, http.MethodGet, "/repos/"+repository+"/commits/"+head+"/check-runs?filter=latest&per_page=100", nil, &checks); err != nil {
		return errors.New("required CI cannot be verified")
	}
	if checks.TotalCount > 100 || checks.TotalCount != len(checks.CheckRuns) {
		return errors.New("CI evidence incomplete or exceeds bounded inspection")
	}
	for name, appID := range required {
		found := 0
		for _, run := range checks.CheckRuns {
			if run.Name != name {
				continue
			}
			found++
			if run.HeadSHA != head || run.Status != "completed" || run.Conclusion != "success" || run.App.Slug != "github-actions" || appID > 0 && run.App.ID != appID {
				return errors.New("required CI is pending, failed or not trusted at approved SHA")
			}
		}
		if name == "" || found != 1 {
			return errors.New("required CI missing or ambiguous")
		}
	}
	return nil
}

func (g GitHub) readyToMerge(ctx context.Context, i core.PublishIntent, branch string, p pullRequest) error {
	if p.Merged || p.State != "open" || p.Mergeable == nil || !*p.Mergeable || p.MergeableState != "clean" || p.Base.SHA != i.BaseSHA {
		return errors.New("PR is closed, stale, blocked or not mergeable")
	}
	sha, exists, err := g.remoteHead(ctx, branch)
	if err != nil || !exists || sha != i.HeadSHA {
		return errors.New("approved branch head changed")
	}
	main, exists, err := g.remoteHead(ctx, "main")
	if err != nil || !exists || main != i.BaseSHA {
		return errors.New("GitOps main changed since reviewed base")
	}
	return g.requiredChecks(ctx, i.HeadSHA)
}

// A generated merge revision is valid only if its two parents are the exact
// reviewed base and PR head. It is never substituted for the reviewed task SHA.
func (g GitHub) verifyMergeRevision(ctx context.Context, i core.PublishIntent, sha string) error {
	if !commitSHA.MatchString(sha) {
		return errors.New("merge returned no full revision SHA")
	}
	var commit struct {
		SHA     string `json:"sha"`
		Parents []struct {
			SHA string `json:"sha"`
		} `json:"parents"`
	}
	_, err := g.request(ctx, http.MethodGet, "/repos/"+repository+"/git/commits/"+sha, nil, &commit)
	if err != nil || commit.SHA != sha || len(commit.Parents) != 2 || commit.Parents[0].SHA != i.BaseSHA || commit.Parents[1].SHA != i.HeadSHA {
		return errors.New("merge revision does not join exact approved base and head")
	}
	return nil
}

func (g GitHub) inspectMerge(ctx context.Context, i core.PublishIntent, branch string) (string, bool, error) {
	p, err := g.fixedPull(ctx, i, branch)
	if err != nil {
		return "", false, err
	}
	if p.Merged {
		if err := g.verifyMergeRevision(ctx, i, p.MergeSHA); err != nil {
			return "", false, err
		}
		return p.MergeSHA, true, nil
	}
	if err := g.readyToMerge(ctx, i, branch, p); err != nil {
		return "", false, err
	}
	return "", false, nil
}

// merge and deploy both publish GitOps state by merging the approved PR. Argo
// CD sync, image provenance and user-path health remain separate host evidence.
func (g GitHub) executeMerge(ctx context.Context, i core.PublishIntent, branch string) (string, error) {
	p, err := g.fixedPull(ctx, i, branch)
	if err != nil {
		return "", err
	}
	if err := g.readyToMerge(ctx, i, branch, p); err != nil {
		return "", err
	}
	if !time.Now().Before(i.ExpiresAt) {
		return "", errors.New("publisher authorization expired before merge")
	}
	var result struct {
		Merged bool   `json:"merged"`
		SHA    string `json:"sha"`
	}
	_, err = g.request(ctx, http.MethodPut, fmt.Sprintf("/repos/%s/pulls/%d/merge", repository, p.Number), map[string]string{"sha": i.HeadSHA, "merge_method": "merge"}, &result)
	if err != nil || !result.Merged {
		return "", errors.New("GitOps merge result unknown or rejected; inspect before any retry")
	}
	if err := g.verifyMergeRevision(ctx, i, result.SHA); err != nil {
		return "", err
	}
	return result.SHA, nil
}

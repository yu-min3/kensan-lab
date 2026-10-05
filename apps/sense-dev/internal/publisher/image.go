package publisher

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/core"
)

// ImageEvidence proves a successful trusted workflow built the reviewed App
// tree. Dispatch acceptance alone never produces this evidence or a digest.
type ImageEvidence struct {
	Repository       string `json:"repository"`
	Image            string `json:"image"`
	SourceSHA        string `json:"source_sha"`
	SourceAppTreeSHA string `json:"source_app_tree_sha"`
	ImageTag         string `json:"image_tag"`
	Digest           string `json:"digest"`
	Visibility       string `json:"visibility"`
	DispatchID       string `json:"dispatch_id"`
	WorkflowSHA      string `json:"workflow_sha"`
	WorkflowSHA256   string `json:"workflow_sha256"`
	WorkflowRef      string `json:"workflow_ref"`
	WorkflowRunID    int64  `json:"workflow_run_id"`
}

func (g GitHub) trustedWorkflow(ctx context.Context, spec core.ImageReleaseSpec) error {
	var ref struct {
		Object struct{ Type, SHA string } `json:"object"`
	}
	_, err := g.request(ctx, http.MethodGet, "/repos/"+repository+"/git/ref/tags/"+url.PathEscape(strings.TrimPrefix(spec.WorkflowRef, "refs/tags/")), nil, &ref)
	if err != nil {
		return errors.New("trusted workflow tag unavailable")
	}
	if ref.Object.Type == "tag" {
		var tag struct {
			Object struct{ Type, SHA string } `json:"object"`
		}
		_, err = g.request(ctx, http.MethodGet, "/repos/"+repository+"/git/tags/"+ref.Object.SHA, nil, &tag)
		if err != nil {
			return errors.New("trusted workflow annotated tag unavailable")
		}
		ref.Object = tag.Object
	}
	if ref.Object.Type != "commit" || ref.Object.SHA != spec.WorkflowSHA {
		return errors.New("trusted workflow tag moved or names another commit")
	}
	var file struct {
		Encoding, Content string
		Type              string
	}
	_, err = g.request(ctx, http.MethodGet, "/repos/"+repository+"/contents/"+spec.WorkflowPath+"?ref="+spec.WorkflowSHA, nil, &file)
	if err != nil || file.Type != "file" || file.Encoding != "base64" {
		return errors.New("trusted workflow contents unavailable")
	}
	content, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(file.Content, "\n", ""))
	if err != nil {
		return errors.New("trusted workflow content encoding invalid")
	}
	sum := sha256.Sum256(content)
	if hex.EncodeToString(sum[:]) != spec.WorkflowSHA256 {
		return errors.New("trusted workflow file differs from fixed policy hash")
	}
	return nil
}

func (g GitHub) sourceTree(ctx context.Context, spec core.ImageReleaseSpec) error {
	var commit struct {
		SHA  string
		Tree struct{ SHA string }
	}
	_, err := g.request(ctx, http.MethodGet, "/repos/"+repository+"/git/commits/"+spec.SourceSHA, nil, &commit)
	if err != nil || commit.SHA != spec.SourceSHA || !commitSHA.MatchString(commit.Tree.SHA) {
		return errors.New("reviewed image source commit unavailable")
	}
	current := commit.Tree.SHA
	for _, component := range []string{"apps", "canary"} {
		var tree struct {
			SHA       string
			Truncated bool
			Tree      []struct{ Path, Mode, Type, SHA string }
		}
		_, err = g.request(ctx, http.MethodGet, "/repos/"+repository+"/git/trees/"+current, nil, &tree)
		if err != nil || tree.SHA != current || tree.Truncated {
			return errors.New("source App tree inspection incomplete")
		}
		found := ""
		for _, entry := range tree.Tree {
			if entry.Path == component {
				if found != "" || entry.Type != "tree" || entry.Mode != "040000" || !commitSHA.MatchString(entry.SHA) {
					return errors.New("source App path is ambiguous or indirect")
				}
				found = entry.SHA
			}
		}
		if found == "" {
			return errors.New("source App tree absent")
		}
		current = found
	}
	if current != spec.SourceAppTreeSHA {
		return errors.New("source App tree differs from reviewed scan")
	}
	return nil
}

func (g GitHub) existingPrivatePackage(ctx context.Context) error {
	var p struct {
		Visibility string
		Repository struct {
			FullName string `json:"full_name"`
		}
	}
	_, err := g.packageRequest(ctx, "/user/packages/container/kensan-lab%2Fcanary", &p)
	if err != nil || p.Visibility != "private" || p.Repository.FullName != repository {
		return errors.New("existing repository-bound private package required; bootstrap is forbidden")
	}
	return nil
}

func (g GitHub) unusedImageTag(ctx context.Context, tag string) error {
	for page := 1; page <= 10; page++ {
		var versions []struct {
			Metadata struct{ Container struct{ Tags []string } }
		}
		_, err := g.packageRequest(ctx, fmt.Sprintf("/user/packages/container/kensan-lab%%2Fcanary/versions?per_page=100&page=%d", page), &versions)
		if err != nil {
			return errors.New("immutable image tag history unavailable")
		}
		for _, version := range versions {
			for _, existing := range version.Metadata.Container.Tags {
				if existing == tag {
					return errors.New("immutable image tag already exists")
				}
			}
		}
		if len(versions) < 100 {
			return nil
		}
	}
	return errors.New("image tag history exceeds bounded inspection")
}

type imageRun struct {
	ID                              int64  `json:"id"`
	HeadSHA                         string `json:"head_sha"`
	DisplayTitle                    string `json:"display_title"`
	Event, Status, Conclusion, Path string
	Attempt                         int `json:"run_attempt"`
	Repository                      struct {
		FullName string `json:"full_name"`
	}
	HeadRepository struct {
		FullName string `json:"full_name"`
	} `json:"head_repository"`
}

func (g GitHub) imageRun(ctx context.Context, spec core.ImageReleaseSpec) (imageRun, bool, error) {
	var found imageRun
	for page := 1; page <= 10; page++ {
		var result struct {
			TotalCount int        `json:"total_count"`
			Runs       []imageRun `json:"workflow_runs"`
		}
		path := fmt.Sprintf("/repos/%s/actions/workflows/canary-image.yml/runs?head_sha=%s&event=workflow_dispatch&per_page=100&page=%d", repository, spec.WorkflowSHA, page)
		if _, err := g.request(ctx, http.MethodGet, path, nil, &result); err != nil {
			return imageRun{}, false, err
		}
		if result.TotalCount > 1000 {
			return imageRun{}, false, errors.New("workflow run history exceeds bounded inspection")
		}
		for _, run := range result.Runs {
			if run.DisplayTitle != "sense-image-"+spec.DispatchID {
				continue
			}
			if found.ID != 0 || run.ID <= 0 || run.HeadSHA != spec.WorkflowSHA || run.Event != "workflow_dispatch" || run.Attempt != 1 || run.Repository.FullName != repository || run.HeadRepository.FullName != repository || !imageRunPathMatches(run.Path, spec) {
				return imageRun{}, false, errors.New("image dispatch run is duplicate, rerun or has different workflow identity")
			}
			found = run
		}
		if len(result.Runs) < 100 {
			return found, found.ID != 0, nil
		}
	}
	return imageRun{}, false, errors.New("workflow run history inspection incomplete")
}

func (g GitHub) inspectImage(ctx context.Context, i core.PublishIntent) (string, bool, error) {
	if err := g.trustedWorkflow(ctx, *i.ImageRelease); err != nil {
		return "", false, err
	}
	run, exists, err := g.imageRun(ctx, *i.ImageRelease)
	if err != nil || !exists {
		return "", false, err
	}
	if run.Status == "completed" && run.Conclusion != "success" {
		return "", false, errors.New("image workflow failed; never redispatch this intent")
	}
	return i.ImageRelease.DispatchID, true, nil
}

func (g GitHub) executeImage(ctx context.Context, i core.PublishIntent, branch string) (string, error) {
	spec := *i.ImageRelease
	if err := g.trustedWorkflow(ctx, spec); err != nil {
		return "", err
	}
	sha, exists, err := g.remoteHead(ctx, branch)
	if err != nil || !exists || sha != spec.SourceSHA {
		return "", errors.New("image source branch is not at approved SHA")
	}
	if err := g.sourceTree(ctx, spec); err != nil {
		return "", err
	}
	if err := g.existingPrivatePackage(ctx); err != nil {
		return "", err
	}
	if err := g.unusedImageTag(ctx, spec.ImageTag); err != nil {
		return "", err
	}
	if _, exists, err := g.imageRun(ctx, spec); err != nil || exists {
		return "", errors.New("image dispatch already exists or cannot be reconciled")
	}
	if !time.Now().Before(i.ExpiresAt) {
		return "", errors.New("image dispatch authorization expired")
	}
	inputs := map[string]string{"source_sha": spec.SourceSHA, "source_app_tree_sha": spec.SourceAppTreeSHA, "image_tag": spec.ImageTag, "dispatch_id": spec.DispatchID, "expected_workflow_sha": spec.WorkflowSHA}
	status, err := g.request(ctx, http.MethodPost, "/repos/"+repository+"/actions/workflows/canary-image.yml/dispatches", map[string]any{"ref": strings.TrimPrefix(spec.WorkflowRef, "refs/tags/"), "inputs": inputs}, nil)
	if err != nil || status != http.StatusNoContent {
		return "", errors.New("image dispatch result unknown; inspect only, never resend")
	}
	return spec.DispatchID, nil
}

func imageRunPathMatches(path string, spec core.ImageReleaseSpec) bool {
	return path == spec.WorkflowPath || path == spec.WorkflowPath+"@"+spec.WorkflowRef || path == repository+"/"+spec.WorkflowPath+"@"+spec.WorkflowRef
}

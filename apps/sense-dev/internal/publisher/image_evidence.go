package publisher

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/core"
)

var imageDigest = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// ImageEvidence reads successful build evidence; it never dispatches, retries,
// publishes, or infers a digest from an accepted dispatch.
func (g GitHub) ImageEvidence(ctx context.Context, i core.PublishIntent) (ImageEvidence, error) {
	if _, err := g.validate(i); err != nil {
		return ImageEvidence{}, err
	}
	if i.Operation != "image_publish" {
		return ImageEvidence{}, errors.New("image evidence requires an image publication intent")
	}
	spec := *i.ImageRelease
	if err := g.trustedWorkflow(ctx, spec); err != nil {
		return ImageEvidence{}, err
	}
	if err := g.sourceTree(ctx, spec); err != nil {
		return ImageEvidence{}, err
	}
	if err := g.existingPrivatePackage(ctx); err != nil {
		return ImageEvidence{}, err
	}
	run, exists, err := g.imageRun(ctx, spec)
	if err != nil {
		return ImageEvidence{}, err
	}
	if !exists || run.Status != "completed" || run.Conclusion != "success" {
		return ImageEvidence{}, errors.New("successful image workflow evidence unavailable")
	}
	var result struct {
		Total     int `json:"total_count"`
		Artifacts []struct {
			ID          int64
			Name        string
			Expired     bool
			WorkflowRun struct {
				ID      int64
				HeadSHA string `json:"head_sha"`
			} `json:"workflow_run"`
		} `json:"artifacts"`
	}
	_, err = g.request(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/actions/runs/%d/artifacts?per_page=100", repository, run.ID), nil, &result)
	if err != nil || result.Total != len(result.Artifacts) || result.Total > 100 {
		return ImageEvidence{}, errors.New("image artifact inspection incomplete")
	}
	var artifactID int64
	for _, a := range result.Artifacts {
		if a.Name != "canary-image-"+spec.DispatchID {
			continue
		}
		if artifactID != 0 || a.ID <= 0 || a.Expired || a.WorkflowRun.ID != run.ID || a.WorkflowRun.HeadSHA != spec.WorkflowSHA {
			return ImageEvidence{}, errors.New("image artifact identity ambiguous or expired")
		}
		artifactID = a.ID
	}
	if artifactID == 0 {
		return ImageEvidence{}, errors.New("image artifact absent")
	}
	data, err := g.downloadImageArtifact(ctx, artifactID)
	if err != nil {
		return ImageEvidence{}, err
	}
	// A fixed, exact schema also rejects duplicate JSON keys and trailing data.
	var fields map[string]json.RawMessage
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil || tok != json.Delim('{') {
		return ImageEvidence{}, errors.New("image provenance object required")
	}
	fields = make(map[string]json.RawMessage)
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			return ImageEvidence{}, err
		}
		name, ok := key.(string)
		if !ok || fields[name] != nil {
			return ImageEvidence{}, errors.New("duplicate image provenance field")
		}
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return ImageEvidence{}, err
		}
		fields[name] = value
	}
	if _, err := dec.Token(); err != nil {
		return ImageEvidence{}, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return ImageEvidence{}, errors.New("trailing image provenance data")
	}
	names := []string{"repository", "image", "source_sha", "source_app_tree_sha", "tag", "digest", "visibility", "dispatch_id", "workflow_sha"}
	if len(fields) != len(names) {
		return ImageEvidence{}, errors.New("image provenance schema differs from trusted workflow")
	}
	values := map[string]string{}
	for _, name := range names {
		var value string
		if err := json.Unmarshal(fields[name], &value); err != nil || fields[name] == nil || string(fields[name]) == "null" {
			return ImageEvidence{}, errors.New("image provenance field missing or non-string")
		}
		values[name] = value
	}
	if values["repository"] != repository || values["image"] != core.CanaryImageRepository || values["source_sha"] != spec.SourceSHA || values["source_app_tree_sha"] != spec.SourceAppTreeSHA || values["tag"] != spec.ImageTag || values["visibility"] != "private" || values["dispatch_id"] != spec.DispatchID || values["workflow_sha"] != spec.WorkflowSHA || !imageDigest.MatchString(values["digest"]) {
		return ImageEvidence{}, errors.New("image provenance differs from fixed source/workflow/private image")
	}
	if err := g.registryImage(ctx, spec, values["digest"]); err != nil {
		return ImageEvidence{}, err
	}
	if err := g.existingPrivatePackage(ctx); err != nil {
		return ImageEvidence{}, err
	}
	return ImageEvidence{Repository: repository, Image: core.CanaryImageRepository, SourceSHA: spec.SourceSHA, SourceAppTreeSHA: spec.SourceAppTreeSHA, ImageTag: spec.ImageTag, Digest: values["digest"], Visibility: "private", DispatchID: spec.DispatchID, WorkflowSHA: spec.WorkflowSHA, WorkflowSHA256: spec.WorkflowSHA256, WorkflowRef: spec.WorkflowRef, WorkflowRunID: run.ID}, nil
}

func (g GitHub) downloadImageArtifact(ctx context.Context, id int64) ([]byte, error) {
	info, err := os.Lstat(g.TokenFile)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("publisher token file must be private and regular")
	}
	token, err := os.ReadFile(g.TokenFile)
	if err != nil || strings.TrimSpace(string(token)) == "" {
		return nil, errors.New("publisher token unavailable")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/repos/%s/actions/artifacts/%d/zip", g.endpoint(), repository, id), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(string(token)))
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	client := &http.Client{Timeout: 20 * time.Second, Transport: &http.Transport{Proxy: nil}}
	if g.Client != nil {
		*client = *g.Client
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	response, err := client.Do(req)
	if err != nil {
		return nil, errors.New("image artifact download unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusFound || response.StatusCode == http.StatusTemporaryRedirect {
		target, err := url.Parse(response.Header.Get("Location"))
		if err != nil || target.Scheme != "https" || target.User != nil || target.Port() != "" || !(target.Hostname() == "objects.githubusercontent.com" || target.Hostname() == "pipelines.actions.githubusercontent.com" || strings.HasSuffix(target.Hostname(), ".blob.core.windows.net")) {
			return nil, errors.New("image artifact redirect outside trusted storage")
		}
		unsigned, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
		if err != nil {
			return nil, err
		}
		response.Body.Close()
		response, err = client.Do(unsigned)
		if err != nil {
			return nil, errors.New("unsigned image artifact download failed")
		}
		defer response.Body.Close()
	}
	if response.StatusCode != http.StatusOK {
		return nil, errors.New("image artifact download failed")
	}
	archive, err := io.ReadAll(io.LimitReader(response.Body, (4<<20)+1))
	if err != nil || len(archive) > 4<<20 {
		return nil, errors.New("image archive too large")
	}
	z, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil || len(z.File) != 1 || z.File[0].Name != "canary-image.json" || z.File[0].UncompressedSize64 > 16<<10 {
		return nil, errors.New("image archive shape invalid")
	}
	f, err := z.File[0].Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, (16<<10)+1))
	if err != nil || len(data) > 16<<10 {
		return nil, errors.New("image provenance too large")
	}
	return data, nil
}

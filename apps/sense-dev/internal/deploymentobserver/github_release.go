package deploymentobserver

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/packageauth"
)

// GitHubRelease reads the successful trusted workflow W and its source-A image
// artifact, then independently checks Git trees at A/B/C and GHCR index leaves.
// Its separate repository and package tokens belong only to the host observer.
type GitHubRelease struct {
	TokenFile        string
	PackageTokenFile string
	Client           *http.Client // loopback-only package API test override
	API              string
}

func (g GitHubRelease) Release(ctx context.Context, p Plan, spec ImageSpec, reviewed, revision string) (ReleaseProof, error) {
	if err := p.validate(); err != nil {
		return ReleaseProof{}, err
	}
	if !shaPattern.MatchString(reviewed) || !shaPattern.MatchString(revision) || spec.Digest == "" || spec.WorkflowRunID <= 0 {
		return ReleaseProof{}, errors.New("reviewed, merge or image identity missing")
	}
	token, err := g.token()
	if err != nil {
		return ReleaseProof{}, err
	}
	packageToken, err := g.packageToken(ctx)
	if err != nil {
		return ReleaseProof{}, err
	}
	if err := g.successfulRun(ctx, token, spec); err != nil {
		return ReleaseProof{}, err
	}
	if err := workflowHashAt(ctx, token, spec.WorkflowSHA, spec.WorkflowSHA256); err != nil {
		return ReleaseProof{}, err
	}
	artifactID, err := g.releaseArtifact(ctx, token, spec.WorkflowRunID, spec.DispatchID)
	if err != nil {
		return ReleaseProof{}, err
	}
	record, err := g.downloadRecord(ctx, token, artifactID)
	if err != nil {
		return ReleaseProof{}, err
	}
	if record.Repository != repository || record.Image != image || record.SourceSHA != spec.SourceSHA || record.SourceAppTreeSHA != spec.SourceAppTreeSHA || record.Tag != spec.ImageTag || record.Digest != spec.Digest || record.Visibility != "private" || record.DispatchID != spec.DispatchID || record.WorkflowSHA != spec.WorkflowSHA {
		return ReleaseProof{}, errors.New("CI image artifact differs from approved source and workflow")
	}
	for _, commit := range []string{spec.SourceSHA, reviewed, revision} {
		tree, err := appTreeAt(ctx, token, commit)
		if err != nil || tree != spec.SourceAppTreeSHA {
			return ReleaseProof{}, errors.New("App tree changed between image source, review and merge")
		}
	}
	if err := imageValuesAt(ctx, token, revision, spec.Digest); err != nil {
		return ReleaseProof{}, err
	}
	private, err := g.packagePrivate(ctx, packageToken)
	if err != nil || !private {
		return ReleaseProof{}, errors.New("current GHCR package privacy could not be verified")
	}
	children, err := g.registryChildren(ctx, packageToken, spec.Digest)
	if err != nil {
		return ReleaseProof{}, err
	}
	return ReleaseProof{Repository: repository, Image: image, SourceSHA: spec.SourceSHA, SourceAppTreeSHA: spec.SourceAppTreeSHA, ImageTag: spec.ImageTag, Digest: spec.Digest, WorkflowRef: spec.WorkflowRef, WorkflowSHA: spec.WorkflowSHA, WorkflowSHA256: spec.WorkflowSHA256, WorkflowRunID: spec.WorkflowRunID, DispatchID: spec.DispatchID, Visibility: "private", ChildDigests: children, Succeeded: true}, nil
}

func (g GitHubRelease) packageToken(ctx context.Context) (string, error) {
	return packageauth.OwnerToken(ctx, g.TokenFile, g.PackageTokenFile, g.Client, g.API)
}

func (g GitHubRelease) token() (string, error) {
	info, err := os.Lstat(g.TokenFile)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return "", errors.New("observer token must be a private regular file")
	}
	data, err := os.ReadFile(g.TokenFile)
	if err != nil {
		return "", errors.New("observer token unavailable")
	}
	token := strings.TrimSpace(string(data))
	if token == "" || strings.ContainsAny(token, "\r\n") {
		return "", errors.New("observer token invalid")
	}
	return token, nil
}

func githubGET(ctx context.Context, token, path string, out any) error {
	return githubGETWithClient(ctx, token, path, out, nil, "https://api.github.com")
}

func githubGETWithClient(ctx context.Context, token, path string, out any, override *http.Client, endpoint string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	client := &http.Client{Timeout: 20 * time.Second, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("GitHub API redirect rejected") }}
	if override != nil {
		*client = *override
		client.CheckRedirect = func(*http.Request, []*http.Request) error { return errors.New("GitHub API redirect rejected") }
	}
	response, err := client.Do(req)
	if err != nil {
		return errors.New("GitHub read-only API unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub read-only API status %d", response.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(out)
}

func (g GitHubRelease) successfulRun(ctx context.Context, token string, spec ImageSpec) error {
	var run struct {
		ID                              int64
		RunAttempt                      int    `json:"run_attempt"`
		HeadSHA                         string `json:"head_sha"`
		DisplayTitle                    string `json:"display_title"`
		Path, Event, Conclusion, Status string
		Repository                      struct {
			FullName string `json:"full_name"`
		}
	}
	if err := githubGET(ctx, token, fmt.Sprintf("/repos/%s/actions/runs/%d", repository, spec.WorkflowRunID), &run); err != nil {
		return err
	}
	if run.ID != spec.WorkflowRunID || run.RunAttempt != 1 || run.HeadSHA != spec.WorkflowSHA || run.DisplayTitle != "sense-image-"+spec.DispatchID || !validWorkflowRunPath(run.Path, spec.WorkflowRef) || run.Event != "workflow_dispatch" || run.Status != "completed" || run.Conclusion != "success" || run.Repository.FullName != repository {
		return errors.New("CI run is not the successful trusted image workflow")
	}
	return nil
}

func validWorkflowRunPath(path, ref string) bool {
	const fixed = ".github/workflows/canary-image.yml"
	return path == fixed || path == fixed+"@"+ref || path == repository+"/"+fixed+"@"+ref
}

func (g GitHubRelease) releaseArtifact(ctx context.Context, token string, runID int64, dispatchID string) (int64, error) {
	var result struct {
		TotalCount int `json:"total_count"`
		Artifacts  []struct {
			ID      int64
			Name    string
			Expired bool
		} `json:"artifacts"`
	}
	if err := githubGET(ctx, token, fmt.Sprintf("/repos/%s/actions/runs/%d/artifacts?per_page=100", repository, runID), &result); err != nil {
		return 0, err
	}
	if result.TotalCount > 100 {
		return 0, errors.New("CI artifact list is incomplete")
	}
	var id int64
	for _, a := range result.Artifacts {
		if a.Name == "canary-image-"+dispatchID && !a.Expired {
			if id != 0 {
				return 0, errors.New("duplicate release artifacts")
			}
			id = a.ID
		}
	}
	if id == 0 {
		return 0, errors.New("release artifact unavailable")
	}
	return id, nil
}

type releaseRecord struct {
	Repository, Image, Tag, Digest, Visibility string
	SourceSHA                                  string `json:"source_sha"`
	SourceAppTreeSHA                           string `json:"source_app_tree_sha"`
	DispatchID                                 string `json:"dispatch_id"`
	WorkflowSHA                                string `json:"workflow_sha"`
}

func (g GitHubRelease) downloadRecord(ctx context.Context, token string, artifactID int64) (releaseRecord, error) {
	endpoint := fmt.Sprintf("https://api.github.com/repos/%s/actions/artifacts/%d/zip", repository, artifactID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return releaseRecord{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	client := &http.Client{Timeout: 20 * time.Second, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(req)
	if err != nil {
		return releaseRecord{}, errors.New("CI release artifact unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusFound || response.StatusCode == http.StatusTemporaryRedirect {
		location, err := url.Parse(response.Header.Get("Location"))
		if err != nil || location.Scheme != "https" || location.User != nil || !artifactDownloadHost(location.Hostname()) {
			return releaseRecord{}, errors.New("CI artifact redirect outside trusted download host")
		}
		unsigned, err := http.NewRequestWithContext(ctx, http.MethodGet, location.String(), nil)
		if err != nil {
			return releaseRecord{}, err
		}
		response.Body.Close()
		response, err = client.Do(unsigned)
		if err != nil {
			return releaseRecord{}, errors.New("CI artifact download failed")
		}
		defer response.Body.Close()
	}
	if response.StatusCode != http.StatusOK {
		return releaseRecord{}, errors.New("CI artifact download was not successful")
	}
	archive, err := io.ReadAll(io.LimitReader(response.Body, 4<<20+1))
	if err != nil || len(archive) > 4<<20 {
		return releaseRecord{}, errors.New("CI artifact is too large")
	}
	z, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil || len(z.File) != 1 || z.File[0].Name != "canary-image.json" || z.File[0].UncompressedSize64 > 16<<10 {
		return releaseRecord{}, errors.New("CI release archive shape invalid")
	}
	file, err := z.File[0].Open()
	if err != nil {
		return releaseRecord{}, err
	}
	defer file.Close()
	decoded, err := io.ReadAll(io.LimitReader(file, 16<<10+1))
	if err != nil || len(decoded) > 16<<10 {
		return releaseRecord{}, errors.New("CI image record is too large")
	}
	var record releaseRecord
	decoder := json.NewDecoder(bytes.NewReader(decoded))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&record); err != nil {
		return releaseRecord{}, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return releaseRecord{}, errors.New("CI image record has trailing data")
	}
	return record, nil
}

func artifactDownloadHost(host string) bool {
	return host == "objects.githubusercontent.com" || host == "pipelines.actions.githubusercontent.com" || strings.HasSuffix(host, ".blob.core.windows.net")
}

func (g GitHubRelease) packagePrivate(ctx context.Context, token string) (bool, error) {
	var result struct {
		Visibility string
		Repository struct {
			FullName string `json:"full_name"`
		}
	}
	endpoint := g.API
	if endpoint == "" {
		endpoint = "https://api.github.com"
	}
	err := githubGETWithClient(ctx, token, "/user/packages/container/kensan-lab%2Fcanary", &result, g.Client, endpoint)
	return result.Visibility == "private" && result.Repository.FullName == repository, err
}

func (g GitHubRelease) registryChildren(ctx context.Context, packageToken, digest string) ([]string, error) {
	tokenURL := "https://ghcr.io/token?service=ghcr.io&scope=" + url.QueryEscape("repository:yu-min3/kensan-lab/canary:pull")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tokenURL, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth("yu-min3", packageToken)
	client := &http.Client{Timeout: 20 * time.Second, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("registry redirect rejected") }}
	response, err := client.Do(req)
	if err != nil {
		return nil, errors.New("private registry auth unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, errors.New("private registry auth rejected")
	}
	var credential struct{ Token string }
	if err := json.NewDecoder(io.LimitReader(response.Body, 16<<10)).Decode(&credential); err != nil || credential.Token == "" {
		return nil, errors.New("private registry token unavailable")
	}
	manifestURL := "https://ghcr.io/v2/yu-min3/kensan-lab/canary/manifests/" + digest
	manifestReq, err := http.NewRequestWithContext(ctx, http.MethodGet, manifestURL, nil)
	if err != nil {
		return nil, err
	}
	manifestReq.Header.Set("Authorization", "Bearer "+credential.Token)
	manifestReq.Header.Set("Accept", "application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.list.v2+json")
	manifestResponse, err := client.Do(manifestReq)
	if err != nil {
		return nil, errors.New("private registry manifest unavailable")
	}
	defer manifestResponse.Body.Close()
	if manifestResponse.StatusCode != http.StatusOK {
		return nil, errors.New("private registry manifest rejected")
	}
	body, err := io.ReadAll(io.LimitReader(manifestResponse.Body, 1<<20+1))
	if err != nil || len(body) > 1<<20 {
		return nil, errors.New("registry manifest too large")
	}
	sum := sha256.Sum256(body)
	if "sha256:"+hex.EncodeToString(sum[:]) != digest {
		return nil, errors.New("registry manifest digest differs from CI")
	}
	var index struct {
		MediaType string `json:"mediaType"`
		Manifests []struct {
			MediaType, Digest string
			Platform          struct{ Architecture, OS string }
		}
	}
	if err := json.Unmarshal(body, &index); err != nil {
		return nil, err
	}
	if index.MediaType != "application/vnd.oci.image.index.v1+json" && index.MediaType != "application/vnd.docker.distribution.manifest.list.v2+json" {
		return nil, errors.New("release digest is not a multiarch index")
	}
	children := []string{}
	for _, child := range index.Manifests {
		if child.Platform.OS == "linux" && (child.Platform.Architecture == "amd64" || child.Platform.Architecture == "arm64") && digestPattern.MatchString(child.Digest) {
			children = append(children, child.Digest)
		}
	}
	if len(children) == 0 {
		return nil, errors.New("registry index has no trusted runtime manifests")
	}
	return children, nil
}

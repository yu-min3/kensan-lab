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
	"path/filepath"
	"strings"
	"time"
)

// GitHubRelease reads a successful, merge-pinned workflow artifact, verifies
// the package remains private, then obtains the immutable GHCR index children.
// Its token belongs only to the host observer process.
type GitHubRelease struct{ TokenFile string }

func (g GitHubRelease) Release(ctx context.Context, p Plan, revision string) (ReleaseProof, error) {
	if err := p.validate(); err != nil {
		return ReleaseProof{}, err
	}
	if !shaPattern.MatchString(revision) {
		return ReleaseProof{}, errors.New("merge revision is not a full SHA")
	}
	token, err := g.token()
	if err != nil {
		return ReleaseProof{}, err
	}
	runID, err := g.successfulRun(ctx, token, revision)
	if err != nil {
		return ReleaseProof{}, err
	}
	artifactID, err := g.releaseArtifact(ctx, token, runID, revision)
	if err != nil {
		return ReleaseProof{}, err
	}
	record, err := g.downloadRecord(ctx, token, artifactID)
	if err != nil {
		return ReleaseProof{}, err
	}
	if record.Repository != repository || record.HeadSHA != revision || record.Image != image || record.Visibility != "private" || !digestPattern.MatchString(record.Digest) || !strings.HasPrefix(record.Tag, "v") {
		return ReleaseProof{}, errors.New("CI release record does not bind private image to merge revision")
	}
	private, err := g.packagePrivate(ctx, token)
	if err != nil || !private {
		return ReleaseProof{}, errors.New("current GHCR package privacy could not be verified")
	}
	children, err := g.registryChildren(ctx, token, record.Digest)
	if err != nil {
		return ReleaseProof{}, err
	}
	return ReleaseProof{Repository: repository, Image: image, SourceSHA: revision, Digest: record.Digest, Visibility: "private", ChildDigests: children, Succeeded: true}, nil
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com"+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	client := &http.Client{Timeout: 20 * time.Second, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("GitHub API redirect rejected") }}
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

func (g GitHubRelease) successfulRun(ctx context.Context, token, revision string) (int64, error) {
	var result struct {
		TotalCount int `json:"total_count"`
		Runs       []struct {
			ID         int64
			HeadSHA    string `json:"head_sha"`
			Event      string
			Conclusion string
		} `json:"workflow_runs"`
	}
	path := "/repos/" + repository + "/actions/workflows/canary-ci.yml/runs?head_sha=" + revision + "&status=completed&per_page=100"
	if err := githubGET(ctx, token, path, &result); err != nil {
		return 0, err
	}
	if result.TotalCount > 100 {
		return 0, errors.New("CI run list is incomplete")
	}
	var id int64
	for _, run := range result.Runs {
		if run.HeadSHA == revision && run.Event == "workflow_dispatch" && run.Conclusion == "success" {
			if id != 0 {
				return 0, errors.New("multiple successful release runs for revision")
			}
			id = run.ID
		}
	}
	if id == 0 {
		return 0, errors.New("successful private release workflow absent")
	}
	return id, nil
}

func (g GitHubRelease) releaseArtifact(ctx context.Context, token string, runID int64, revision string) (int64, error) {
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
		if a.Name == "canary-release-"+revision && !a.Expired {
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
	Repository                     string `json:"repository"`
	HeadSHA                        string `json:"head_sha"`
	Image, Tag, Digest, Visibility string
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
	if err != nil || len(z.File) != 1 || filepath.Base(z.File[0].Name) != "canary-release.json" || z.File[0].UncompressedSize64 > 16<<10 {
		return releaseRecord{}, errors.New("CI release archive shape invalid")
	}
	file, err := z.File[0].Open()
	if err != nil {
		return releaseRecord{}, err
	}
	defer file.Close()
	var record releaseRecord
	if err := json.NewDecoder(io.LimitReader(file, 16<<10)).Decode(&record); err != nil {
		return releaseRecord{}, err
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
	err := githubGET(ctx, token, "/users/yu-min3/packages/container/kensan-lab%2Fcanary", &result)
	return result.Visibility == "private" && result.Repository.FullName == repository, err
}

func (g GitHubRelease) registryChildren(ctx context.Context, githubToken, digest string) ([]string, error) {
	tokenURL := "https://ghcr.io/token?service=ghcr.io&scope=" + url.QueryEscape("repository:yu-min3/kensan-lab/canary:pull")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tokenURL, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth("yu-min3", githubToken)
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

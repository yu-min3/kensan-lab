package publisher

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/core"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// registryImage verifies actual private registry objects before any digest child
// can be planned. The CI artifact is an identity claim, not registry proof.
func (g GitHub) registryImage(ctx context.Context, spec core.ImageReleaseSpec, digest string) error {
	endpoint := "https://ghcr.io"
	if g.RegistryAPI != "" {
		parsed, err := url.Parse(g.RegistryAPI)
		if err != nil || parsed.Scheme != "http" || parsed.Hostname() != "127.0.0.1" || parsed.Port() == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" {
			return errors.New("registry test endpoint outside loopback")
		}
		endpoint = g.RegistryAPI
	}
	info, err := os.Lstat(g.TokenFile)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return errors.New("private registry credential unavailable")
	}
	credential, err := os.ReadFile(g.TokenFile)
	if err != nil || strings.TrimSpace(string(credential)) == "" {
		return errors.New("private registry credential unavailable")
	}
	client := &http.Client{Timeout: 20 * time.Second, Transport: &http.Transport{Proxy: nil}}
	if g.Client != nil {
		*client = *g.Client
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return errors.New("registry redirect forbidden") }
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"/token?service=ghcr.io&scope="+url.QueryEscape("repository:yu-min3/kensan-lab/canary:pull"), nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth("yu-min3", strings.TrimSpace(string(credential)))
	response, err := client.Do(req)
	if err != nil {
		return errors.New("private registry auth unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return errors.New("private registry auth rejected")
	}
	var access struct{ Token string }
	if json.NewDecoder(io.LimitReader(response.Body, 16<<10)).Decode(&access) != nil || access.Token == "" {
		return errors.New("private registry token missing")
	}
	get := func(path, expected string) ([]byte, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"/v2/yu-min3/kensan-lab/canary/"+path, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+access.Token)
		req.Header.Set("Accept", "application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.list.v2+json, application/vnd.oci.image.manifest.v1+json, application/vnd.docker.distribution.manifest.v2+json")
		response, err := client.Do(req)
		if err != nil {
			return nil, errors.New("registry proof unavailable")
		}
		defer response.Body.Close()
		if strings.HasPrefix(path, "blobs/") && (response.StatusCode == http.StatusTemporaryRedirect || response.StatusCode == http.StatusFound) {
			target, err := url.Parse(response.Header.Get("Location"))
			if err != nil || target.Scheme != "https" || target.User != nil || target.Port() != "" || target.Hostname() != "pkg-containers.githubusercontent.com" {
				return nil, errors.New("registry blob redirect outside trusted storage")
			}
			unsigned, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
			if err != nil {
				return nil, err
			}
			response.Body.Close()
			response, err = client.Do(unsigned)
			if err != nil {
				return nil, errors.New("registry blob download unavailable")
			}
			defer response.Body.Close()
		}
		if response.StatusCode != http.StatusOK {
			return nil, errors.New("registry proof rejected")
		}
		body, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
		if err != nil || len(body) > 1<<20 {
			return nil, errors.New("registry object exceeds bound")
		}
		hash := sha256.Sum256(body)
		if "sha256:"+hex.EncodeToString(hash[:]) != expected {
			return nil, errors.New("registry object digest differs from image evidence")
		}
		return body, nil
	}
	if !imageDigest.MatchString(digest) {
		return errors.New("invalid image registry digest")
	}
	if _, err := get("manifests/"+spec.ImageTag, digest); err != nil {
		return err
	}
	body, err := get("manifests/"+digest, digest)
	if err != nil {
		return err
	}
	var index struct {
		SchemaVersion int
		MediaType     string
		Manifests     []struct {
			MediaType, Digest string
			Platform          struct{ OS, Architecture string }
		}
	}
	if json.Unmarshal(body, &index) != nil || index.SchemaVersion != 2 || index.MediaType != "application/vnd.oci.image.index.v1+json" && index.MediaType != "application/vnd.docker.distribution.manifest.list.v2+json" || len(index.Manifests) > 20 {
		return errors.New("bounded multiarchitecture registry index required")
	}
	architectures := map[string]bool{}
	for _, child := range index.Manifests {
		if child.Platform.OS != "linux" || child.Platform.Architecture != "amd64" && child.Platform.Architecture != "arm64" {
			continue
		}
		if architectures[child.Platform.Architecture] || !imageDigest.MatchString(child.Digest) || child.MediaType != "application/vnd.oci.image.manifest.v1+json" && child.MediaType != "application/vnd.docker.distribution.manifest.v2+json" {
			return errors.New("registry runtime child ambiguous")
		}
		manifest, err := get("manifests/"+child.Digest, child.Digest)
		if err != nil {
			return err
		}
		var runtime struct {
			SchemaVersion int
			MediaType     string
			Config        struct{ MediaType, Digest string }
		}
		if json.Unmarshal(manifest, &runtime) != nil || runtime.SchemaVersion != 2 || runtime.MediaType != child.MediaType || !imageDigest.MatchString(runtime.Config.Digest) || runtime.Config.MediaType != "application/vnd.oci.image.config.v1+json" && runtime.Config.MediaType != "application/vnd.docker.container.image.v1+json" {
			return errors.New("registry runtime config invalid")
		}
		config, err := get("blobs/"+runtime.Config.Digest, runtime.Config.Digest)
		if err != nil {
			return err
		}
		var image struct {
			OS, Architecture string
			Config           struct{ Labels map[string]string }
		}
		if json.Unmarshal(config, &image) != nil || image.OS != "linux" || image.Architecture != child.Platform.Architecture || image.Config.Labels["org.opencontainers.image.source"] != "https://github.com/"+repository || image.Config.Labels["org.opencontainers.image.revision"] != spec.SourceSHA || image.Config.Labels["dev.kensan.workflow-sha"] != spec.WorkflowSHA {
			return errors.New("registry image config differs from approved source/workflow")
		}
		architectures[child.Platform.Architecture] = true
	}
	if !architectures["amd64"] || !architectures["arm64"] {
		return errors.New("both trusted runtime architectures required")
	}
	return nil
}

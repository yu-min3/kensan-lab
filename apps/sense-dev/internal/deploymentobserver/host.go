package deploymentobserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

// HostSources uses the operator's read-only kubeconfig and a separately
// authenticated release proof source. No kubeconfig or registry token enters
// the model worker manifest or an observation artifact.
type HostSources struct {
	Kube          KubeCLI
	ReleaseSource ReleaseSource
}
type ReleaseSource interface {
	Release(context.Context, Plan, string) (ReleaseProof, error)
}

func (h HostSources) Application(ctx context.Context, p Plan) (ApplicationState, error) {
	return h.Kube.Application(ctx, p)
}
func (h HostSources) Pods(ctx context.Context, p Plan) ([]PodState, error) {
	return h.Kube.Pods(ctx, p)
}
func (h HostSources) Release(ctx context.Context, p Plan, revision string) (ReleaseProof, error) {
	if h.ReleaseSource == nil {
		return ReleaseProof{}, errors.New("trusted CI and registry proof source unavailable")
	}
	return h.ReleaseSource.Release(ctx, p, revision)
}
func (h HostSources) Probe(ctx context.Context, p Plan) (UserPathState, error) {
	return HTTPProbe(ctx, p)
}

// KubeCLI shells out to kubectl with fixed read-only arguments, never a shell.
// The host process owns KUBECONFIG; operator policy must grant GET/LIST only.
type KubeCLI struct{ Binary, Kubeconfig string }

func (k KubeCLI) get(ctx context.Context, args ...string) ([]byte, error) {
	if !filepath.IsAbs(k.Kubeconfig) {
		return nil, errors.New("observer kubeconfig path must be absolute")
	}
	info, err := os.Lstat(k.Kubeconfig)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("observer kubeconfig must be a private regular file")
	}
	path := k.Binary
	if path == "" {
		path = "kubectl"
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, args...)
	// Kubeconfig and any exec authentication plugin are operator-controlled
	// host credentials; they are never mounted into model worker rootfs.
	cmd.Env = append(os.Environ(), "KUBECONFIG="+k.Kubeconfig)
	output, err := cmd.Output()
	if err != nil {
		return nil, errors.New("read-only kubectl observation failed")
	}
	if len(output) > 1<<20 {
		return nil, errors.New("kubectl observation too large")
	}
	return output, nil
}

func (k KubeCLI) Application(ctx context.Context, p Plan) (ApplicationState, error) {
	if err := p.validate(); err != nil {
		return ApplicationState{}, err
	}
	body, err := k.get(ctx, "-n", "argocd", "get", "applications.argoproj.io", application, "-o", "json")
	if err != nil {
		return ApplicationState{}, err
	}
	var document struct {
		Metadata struct{ Name, Namespace string } `json:"metadata"`
		Spec     struct {
			Sources []struct{ RepoURL, TargetRevision, Path, Ref string } `json:"sources"`
		} `json:"spec"`
		Status struct {
			Sync struct {
				Status    string
				Revisions []string
				Revision  string
			} `json:"sync"`
			Health struct{ Status string } `json:"health"`
		} `json:"status"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return ApplicationState{}, err
	}
	if document.Metadata.Name != application || document.Metadata.Namespace != "argocd" || len(document.Spec.Sources) != 3 || len(document.Status.Sync.Revisions) != 3 {
		return ApplicationState{}, errors.New("unexpected Argo Application identity or source count")
	}
	state := ApplicationState{Sync: document.Status.Sync.Status, Health: document.Status.Health.Status}
	for i, source := range document.Spec.Sources {
		state.Sources = append(state.Sources, ArgoSource{RepoURL: source.RepoURL, TargetRevision: source.TargetRevision, Path: source.Path, Ref: source.Ref, ResolvedRevision: document.Status.Sync.Revisions[i]})
	}
	return state, nil
}

func (k KubeCLI) Pods(ctx context.Context, p Plan) ([]PodState, error) {
	if err := p.validate(); err != nil {
		return nil, err
	}
	body, err := k.get(ctx, "-n", namespace, "get", "pods", "-l", "app.kubernetes.io/name=canary", "-o", "json")
	if err != nil {
		return nil, err
	}
	var document struct {
		Items []struct {
			Metadata struct {
				Namespace         string
				DeletionTimestamp *time.Time `json:"deletionTimestamp"`
			} `json:"metadata"`
			Spec struct {
				Containers []struct{ Image string } `json:"containers"`
			} `json:"spec"`
			Status struct {
				Phase             string
				PodIP             string `json:"podIP"`
				ContainerStatuses []struct {
					Ready   bool
					ImageID string `json:"imageID"`
				} `json:"containerStatuses"`
			} `json:"status"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return nil, err
	}
	if len(document.Items) == 0 {
		return nil, errors.New("canary pods missing")
	}
	result := make([]PodState, 0, len(document.Items))
	for _, pod := range document.Items {
		if pod.Metadata.Namespace != namespace || pod.Metadata.DeletionTimestamp != nil || len(pod.Spec.Containers) != 1 || len(pod.Status.ContainerStatuses) != 1 {
			return nil, errors.New("canary pod identity or container count changed")
		}
		podIP, err := netip.ParseAddr(pod.Status.PodIP)
		if err != nil {
			return nil, errors.New("canary PodIP missing or invalid")
		}
		result = append(result, PodState{Ready: pod.Status.Phase == "Running" && pod.Status.ContainerStatuses[0].Ready, Image: pod.Spec.Containers[0].Image, ImageID: pod.Status.ContainerStatuses[0].ImageID, PodIP: podIP})
	}
	return result, nil
}

// HTTPProbe reads only the bounded JSON release marker from the validated
// numeric Pod IP. Raw response content is never written to an artifact.
func HTTPProbe(ctx context.Context, p Plan) (UserPathState, error) {
	if err := p.validate(); err != nil {
		return UserPathState{}, err
	}
	address := net.JoinHostPort(p.ProbeIP.String(), strconv.Itoa(int(p.ProbePort)))
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirect rejected") }, Transport: &http.Transport{Proxy: nil, DisableKeepAlives: true, DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
		if network != "tcp" {
			return nil, errors.New("unexpected network")
		}
		return dialer.DialContext(ctx, "tcp4", address)
	}}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+address+p.ProbePath, nil)
	if err != nil {
		return UserPathState{}, err
	}
	response, err := client.Do(req)
	if err != nil {
		return UserPathState{}, fmt.Errorf("private probe failed: %w", err)
	}
	defer response.Body.Close()
	if response.Request == nil || response.Request.URL.Host != address || response.Request.URL.Scheme != "http" {
		return UserPathState{}, errors.New("private probe target changed")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return UserPathState{}, errors.New("private release endpoint did not succeed")
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<10+1))
	if err != nil {
		return UserPathState{}, errors.New("private release response unreadable")
	}
	release, err := parseReleaseResponse(body)
	if err != nil {
		return UserPathState{}, err
	}
	return UserPathState{StatusCode: response.StatusCode, Path: p.ProbePath, ObservedRelease: release}, nil
}

func parseReleaseResponse(body []byte) (string, error) {
	if len(body) > 4<<10 {
		return "", errors.New("private release response is too large")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var result struct {
		Release string `json:"release"`
	}
	if err := decoder.Decode(&result); err != nil || !releasePattern.MatchString(result.Release) {
		return "", errors.New("private release response is invalid")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return "", errors.New("private release response has trailing data")
	}
	return result.Release, nil
}

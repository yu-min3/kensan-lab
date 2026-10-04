package deploymentobserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/core"
)

const (
	namespace   = "app-canary"
	application = "app-canary"
	repository  = "yu-min3/kensan-lab"
	image       = "ghcr.io/yu-min3/kensan-lab/canary"
)

var (
	shaPattern     = regexp.MustCompile(`^[0-9a-f]{40}$`)
	digestPattern  = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	releasePattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)
)

// Plan is operator supplied and deliberately bound to one private canary. The
// endpoint must be a numeric RFC1918 address; discovery data cannot redirect it.
type Plan struct {
	SchemaVersion int        `json:"schema_version"`
	Namespace     string     `json:"namespace"`
	Application   string     `json:"application"`
	Repository    string     `json:"repository"`
	Image         string     `json:"image"`
	ProbeIP       netip.Addr `json:"probe_ip,omitempty"`
	// ProbeCIDR is an operator-reviewed private Pod network boundary. The host
	// selects only already-verified canary Pods; workers cannot supply URLs.
	ProbeCIDR       netip.Prefix `json:"probe_cidr,omitempty"`
	ProbePort       uint16       `json:"probe_port"`
	ProbePath       string       `json:"probe_path"`
	ExpectedRelease string       `json:"expected_release"`
}

// LoadPlan accepts only an operator-owned, bounded JSON file. Neither model
// output nor a task worktree may select the probe endpoint or target objects.
func LoadPlan(path string) (Plan, error) {
	if !filepath.IsAbs(path) {
		return Plan{}, errors.New("observer plan path must be absolute")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0022 != 0 || info.Size() > 16<<10 {
		return Plan{}, errors.New("observer plan must be a bounded non-writable regular file")
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return Plan{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var plan Plan
	if err := decoder.Decode(&plan); err != nil {
		return Plan{}, errors.New("invalid observer plan")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return Plan{}, errors.New("observer plan has trailing data")
	}
	return plan, plan.validate()
}

func (p Plan) validate() error {
	if p.SchemaVersion != 1 || p.Namespace != namespace || p.Application != application || p.Repository != repository || p.Image != image || !p.validProbeBoundary() || p.ProbePort == 0 || p.ProbePath != "/api/release" || !releasePattern.MatchString(p.ExpectedRelease) {
		return errors.New("observer plan is outside the fixed private canary")
	}
	return nil
}

func (p Plan) validProbeBoundary() bool {
	if p.ProbeIP.IsValid() == p.ProbeCIDR.IsValid() {
		return false
	}
	if p.ProbeIP.IsValid() {
		return p.ProbeIP.Is4() && p.ProbeIP.IsPrivate()
	}
	prefix := p.ProbeCIDR
	if !prefix.Addr().Is4() || prefix != prefix.Masked() {
		return false
	}
	for _, block := range []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8"), netip.MustParsePrefix("172.16.0.0/12"), netip.MustParsePrefix("192.168.0.0/16")} {
		if prefix.Bits() >= block.Bits() && block.Contains(prefix.Addr()) {
			return true
		}
	}
	return false
}

type ApplicationState struct {
	Revision, Sync, Health string
	Sources                []ArgoSource
}
type ArgoSource struct{ RepoURL, Path, Ref, TargetRevision, ResolvedRevision string }
type PodState struct {
	Ready          bool
	Image, ImageID string
	PodIP          netip.Addr
}

// ChildDigests are verified platform manifests under the registry's immutable
// multi-architecture index digest, not values asserted by the model or CI JSON.
type ReleaseProof struct {
	Repository, Image, SourceSHA, SourceAppTreeSHA, ImageTag, Digest, Visibility, WorkflowRef, WorkflowSHA, WorkflowSHA256, DispatchID string
	WorkflowRunID                                                                                                                      int64
	ChildDigests                                                                                                                       []string
	Succeeded                                                                                                                          bool
}
type ImageSpec = core.ImageDeploymentSpec
type UserPathState struct {
	StatusCode      int
	Path            string
	ObservedRelease string
}

type Sources interface {
	Application(context.Context, Plan) (ApplicationState, error)
	Pods(context.Context, Plan) ([]PodState, error)
	Release(context.Context, Plan, ImageSpec, string, string) (ReleaseProof, error)
	Probe(context.Context, Plan) (UserPathState, error)
}

type Observer struct{ Sources Sources }

// Observe requires four independent host observations for the same merge
// revision and immutable image digest. A healthy Argo status alone is not enough.
func (o Observer) Observe(ctx context.Context, p Plan, reviewedHead, revision string, spec ImageSpec) (core.DeploymentReceipt, error) {
	if err := p.validate(); err != nil {
		return core.DeploymentReceipt{}, err
	}
	if !shaPattern.MatchString(reviewedHead) || !shaPattern.MatchString(revision) || o.Sources == nil || core.ValidateImageDeploymentSpec(spec) != nil {
		return core.DeploymentReceipt{}, errors.New("reviewed head, merge revision or observer source missing")
	}
	app, err := o.Sources.Application(ctx, p)
	if err != nil {
		return core.DeploymentReceipt{}, fmt.Errorf("Argo observation: %w", err)
	}
	if !validArgoSources(app.Sources, revision) || app.Sync != "Synced" || app.Health != "Healthy" {
		return core.DeploymentReceipt{}, errors.New("Argo is not synced and healthy at the sent merge revision")
	}
	proof, err := o.Sources.Release(ctx, p, spec, reviewedHead, revision)
	if err != nil {
		return core.DeploymentReceipt{}, fmt.Errorf("CI release observation: %w", err)
	}
	if !proof.Succeeded || proof.Repository != repository || proof.Image != image || proof.SourceSHA != spec.SourceSHA || proof.SourceAppTreeSHA != spec.SourceAppTreeSHA || proof.ImageTag != spec.ImageTag || proof.Digest != spec.Digest || proof.WorkflowRef != spec.WorkflowRef || proof.WorkflowSHA != spec.WorkflowSHA || proof.WorkflowSHA256 != spec.WorkflowSHA256 || proof.WorkflowRunID != spec.WorkflowRunID || proof.DispatchID != spec.DispatchID || proof.Visibility != "private" {
		return core.DeploymentReceipt{}, errors.New("CI release provenance does not bind private image to merge revision")
	}
	pods, err := o.Sources.Pods(ctx, p)
	if err != nil {
		return core.DeploymentReceipt{}, fmt.Errorf("pod observation: %w", err)
	}
	if len(pods) == 0 {
		return core.DeploymentReceipt{}, errors.New("canary has no pods")
	}
	children := map[string]bool{}
	for _, digest := range proof.ChildDigests {
		if !digestPattern.MatchString(digest) {
			return core.DeploymentReceipt{}, errors.New("registry manifest has invalid child digest")
		}
		children[digest] = true
	}
	if len(children) == 0 {
		return core.DeploymentReceipt{}, errors.New("registry manifest children are unavailable")
	}
	selectedIP := netip.Addr{}
	for _, pod := range pods {
		runtimeDigest, ok := runtimeImageDigest(pod.ImageID)
		if !pod.Ready || !pod.PodIP.Is4() || !pod.PodIP.IsPrivate() || pod.Image != image+"@"+proof.Digest || !ok || !children[runtimeDigest] {
			return core.DeploymentReceipt{}, errors.New("canary pod is not ready at proven image digest")
		}
		if pod.PodIP == p.ProbeIP || p.ProbeCIDR.IsValid() && p.ProbeCIDR.Contains(pod.PodIP) {
			if !selectedIP.IsValid() || pod.PodIP.Compare(selectedIP) < 0 {
				selectedIP = pod.PodIP
			}
		}
	}
	if !selectedIP.IsValid() {
		return core.DeploymentReceipt{}, errors.New("private probe does not target a verified canary pod")
	}
	// Resolve the reviewed CIDR to a verified numeric Pod IP, then reuse the
	// static HTTP boundary. The operator plan remains unchanged across rollouts.
	probePlan := p
	probePlan.ProbeIP = selectedIP
	probePlan.ProbeCIDR = netip.Prefix{}
	user, err := o.Sources.Probe(ctx, probePlan)
	if err != nil {
		return core.DeploymentReceipt{}, fmt.Errorf("private user path: %w", err)
	}
	if user.Path != p.ProbePath || user.StatusCode < 200 || user.StatusCode >= 300 || user.ObservedRelease != p.ExpectedRelease {
		return core.DeploymentReceipt{}, errors.New("private user path did not pass")
	}
	return core.DeploymentReceipt{HeadSHA: reviewedHead, Revision: revision, ImageSourceSHA: spec.SourceSHA, ImageDigest: proof.Digest, Environment: "private-canary", Status: "healthy", UserPath: p.ProbePath, ObservedRelease: user.ObservedRelease}, nil
}

func validArgoSources(sources []ArgoSource, revision string) bool {
	if len(sources) != 3 {
		return false
	}
	seen := map[string]bool{}
	for _, source := range sources {
		if source.RepoURL != "https://github.com/yu-min3/kensan-lab" && source.RepoURL != "https://github.com/yu-min3/kensan-lab.git" {
			return false
		}
		switch {
		case source.Path == "charts/app-base" && source.Ref == "":
			if source.TargetRevision != "main" || source.ResolvedRevision != revision {
				return false
			}
			seen["chart"] = true
		case source.Path == "" && source.Ref == "values":
			if source.TargetRevision != "main" || source.ResolvedRevision != revision {
				return false
			}
			seen["values"] = true
		case source.Path == "kubernetes/apps/app-canary/resources" && source.Ref == "":
			if source.TargetRevision != "main" || source.ResolvedRevision != revision {
				return false
			}
			seen["resources"] = true
		default:
			return false
		}
	}
	return len(seen) == 3
}

func runtimeImageDigest(imageID string) (string, bool) {
	for _, prefix := range []string{"docker-pullable://" + image + "@", "containerd://" + image + "@", "containerd://", image + "@"} {
		if strings.HasPrefix(imageID, prefix) {
			digest := strings.TrimPrefix(imageID, prefix)
			return digest, digestPattern.MatchString(digest)
		}
	}
	return "", false
}

// Record writes a system-owned observation only after all read-only probes
// agree, then delegates authority checks to the Store's receipt validator.
func (o Observer) Record(ctx context.Context, s *core.Store, p Plan, taskID, decisionID, intentID, reviewedHead, revision string) error {
	if s == nil || taskID == "" || decisionID == "" || intentID == "" {
		return errors.New("deployment identity is incomplete")
	}
	st := s.Snapshot()
	intent, ok := st.Intents[intentID]
	if !ok || intent.ImageDeployment == nil {
		return errors.New("sent image deployment spec is missing")
	}
	input := intent.ImageDeployment
	spec := *input
	r, err := o.Observe(ctx, p, reviewedHead, revision, spec)
	if err != nil {
		return err
	}
	r.TaskID, r.DecisionID, r.IntentID = taskID, decisionID, intentID
	body, err := json.Marshal(r)
	if err != nil {
		return err
	}
	a, err := s.PutArtifact("system", "deployment_observation", body)
	if err != nil {
		return err
	}
	r.EvidenceRef = core.ArtifactRef{ID: a.ID, Version: a.Version, SHA256: a.SHA256}
	return s.RecordDeploymentReceipt(r)
}

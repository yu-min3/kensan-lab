package deploymentobserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeSources struct {
	app   ApplicationState
	pods  []PodState
	proof ReleaseProof
	user  UserPathState
}

type recordingProbeSources struct {
	fakeSources
	plans []Plan
}

func (f *recordingProbeSources) Probe(_ context.Context, p Plan) (UserPathState, error) {
	f.plans = append(f.plans, p)
	return f.user, nil
}

func TestLoadPlanRejectsMutableAndTrailingInput(t *testing.T) {
	p := Plan{SchemaVersion: 1, Namespace: namespace, Application: application, Repository: repository, Image: image, ProbeIP: netip.MustParseAddr("10.0.0.8"), ProbePort: 8080, ProbePath: "/api/release", ExpectedRelease: "v1"}
	body, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "observer.json")
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	if got, err := LoadPlan(path); err != nil || got != p {
		t.Fatalf("valid plan=%+v %v", got, err)
	}
	for name, content := range map[string][]byte{"trailing": append(append([]byte{}, body...), []byte(" true")...), "unknown": append(append([]byte{}, body[:len(body)-1]...), []byte(",\"worker_command\":\"echo\"}")...)} {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(path, content, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadPlan(path); err == nil {
				t.Fatal("untrusted plan accepted")
			}
		})
	}
	if err := os.WriteFile(path, body, 0666); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0666); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPlan(path); err == nil {
		t.Fatal("group/world writable plan accepted")
	}
}

func (f fakeSources) Application(context.Context, Plan) (ApplicationState, error) { return f.app, nil }
func (f fakeSources) Pods(context.Context, Plan) ([]PodState, error)              { return f.pods, nil }
func (f fakeSources) Release(context.Context, Plan, ImageSpec, string, string) (ReleaseProof, error) {
	return f.proof, nil
}
func (f fakeSources) Probe(context.Context, Plan) (UserPathState, error) { return f.user, nil }

func TestObserveBindsMergeRevisionPrivateImageAndUserPath(t *testing.T) {
	head, merge := strings.Repeat("a", 40), strings.Repeat("b", 40)
	source, tree, workflow, dispatch := strings.Repeat("e", 40), strings.Repeat("f", 40), strings.Repeat("9", 40), strings.Repeat("1", 32)
	index, leaf := "sha256:"+strings.Repeat("c", 64), "sha256:"+strings.Repeat("d", 64)
	spec := ImageSpec{SourceSHA: source, SourceAppTreeSHA: tree, ImageTag: "sense-" + source + "-" + dispatch, Digest: index, WorkflowRef: "refs/tags/sense-image-workflow-v1", WorkflowSHA: workflow, WorkflowSHA256: strings.Repeat("8", 64), DispatchID: dispatch, WorkflowRunID: 42}
	plan := Plan{SchemaVersion: 1, Namespace: namespace, Application: application, Repository: repository, Image: image, ProbeIP: netip.MustParseAddr("10.0.0.8"), ProbePort: 8080, ProbePath: "/api/release", ExpectedRelease: "v1"}
	sources := []ArgoSource{{RepoURL: "https://github.com/yu-min3/kensan-lab", Path: "charts/app-base", TargetRevision: "main", ResolvedRevision: merge}, {RepoURL: "https://github.com/yu-min3/kensan-lab", Ref: "values", TargetRevision: "main", ResolvedRevision: merge}, {RepoURL: "https://github.com/yu-min3/kensan-lab", Path: "kubernetes/apps/app-canary/resources", TargetRevision: "main", ResolvedRevision: merge}}
	f := fakeSources{app: ApplicationState{Sources: sources, Sync: "Synced", Health: "Healthy"}, pods: []PodState{{Ready: true, Image: image + "@" + index, ImageID: "containerd://" + leaf, PodIP: plan.ProbeIP}}, proof: ReleaseProof{Repository: repository, Image: image, SourceSHA: source, SourceAppTreeSHA: tree, ImageTag: spec.ImageTag, Digest: index, WorkflowRef: spec.WorkflowRef, WorkflowSHA: workflow, WorkflowSHA256: spec.WorkflowSHA256, WorkflowRunID: spec.WorkflowRunID, DispatchID: dispatch, ChildDigests: []string{leaf}, Visibility: "private", Succeeded: true}, user: UserPathState{StatusCode: 200, Path: "/api/release", ObservedRelease: "v1"}}
	r, err := (Observer{Sources: f}).Observe(context.Background(), plan, head, merge, spec)
	if err != nil || r.HeadSHA != head || r.Revision != merge || r.ImageSourceSHA != source || r.ImageDigest != index || r.ObservedRelease != "v1" {
		t.Fatalf("valid observation=%+v %v", r, err)
	}
	// The same operator CIDR works across a rollout with new Pod addresses.
	cidrPlan := plan
	cidrPlan.ProbeIP = netip.Addr{}
	cidrPlan.ProbeCIDR = netip.MustParsePrefix("10.0.0.0/24")
	recorder := &recordingProbeSources{fakeSources: f}
	recorder.pods = append([]PodState(nil), f.pods...)
	for _, ip := range []string{"10.0.0.12", "10.0.0.17"} {
		recorder.pods[0].PodIP = netip.MustParseAddr(ip)
		if _, err := (Observer{Sources: recorder}).Observe(context.Background(), cidrPlan, head, merge, spec); err != nil {
			t.Fatalf("rollout IP %s: %v", ip, err)
		}
		chosen := recorder.plans[len(recorder.plans)-1]
		if chosen.ProbeIP != recorder.pods[0].PodIP || chosen.ProbeCIDR.IsValid() || chosen.ProbePort != plan.ProbePort || chosen.ProbePath != plan.ProbePath {
			t.Fatal("probe was not resolved to the verified Pod")
		}
	}
	recorder.pods = append(recorder.pods, recorder.pods[0])
	recorder.pods[0].PodIP = netip.MustParseAddr("10.0.0.21")
	recorder.pods[1].PodIP = netip.MustParseAddr("10.0.0.18")
	if _, err := (Observer{Sources: recorder}).Observe(context.Background(), cidrPlan, head, merge, spec); err != nil || recorder.plans[len(recorder.plans)-1].ProbeIP.String() != "10.0.0.18" {
		t.Fatalf("deterministic selection: %v", err)
	}
	for _, bad := range []string{"10.0.1.1", "8.8.8.8"} {
		recorder.pods = append([]PodState(nil), f.pods...)
		recorder.pods[0].PodIP = netip.MustParseAddr(bad)
		before := len(recorder.plans)
		if _, err := (Observer{Sources: recorder}).Observe(context.Background(), cidrPlan, head, merge, spec); err == nil || len(recorder.plans) != before {
			t.Fatal("unverified IP was probed")
		}
	}
	recorder.pods = append([]PodState(nil), f.pods...)
	recorder.pods[0].Ready = false
	before := len(recorder.plans)
	if _, err := (Observer{Sources: recorder}).Observe(context.Background(), cidrPlan, head, merge, spec); err == nil || len(recorder.plans) != before {
		t.Fatal("unready CIDR Pod was probed")
	}
	if _, err := HTTPProbe(context.Background(), cidrPlan); err == nil {
		t.Fatal("unresolved CIDR performed an HTTP probe")
	}
	// API transport selects a Pod from the same validated CIDR; no name is
	// accepted from serialized plan input and a new rollout is selected afresh.
	proxyPlan := cidrPlan
	proxyPlan.ProbeTransport = "kubernetes-api"
	proxyPlan.ProbePort = 8000
	proxy := &recordingProbeSources{fakeSources: f}
	proxy.pods = append([]PodState(nil), f.pods...)
	for n, ip := range []string{"10.0.0.12", "10.0.0.17"} {
		proxy.pods[0].PodIP = netip.MustParseAddr(ip)
		proxy.pods[0].Name = fmt.Sprintf("canary-rollout-%d", n)
		proxy.pods[0].Namespace = namespace
		proxy.pods[0].UID = fmt.Sprintf("uid-%d", n)
		proxy.pods[0].AppLabel = "canary"
		if _, err := (Observer{Sources: proxy}).Observe(context.Background(), proxyPlan, head, merge, spec); err != nil {
			t.Fatal(err)
		}
		chosen := proxy.plans[len(proxy.plans)-1]
		if chosen.probePod == nil || *chosen.probePod != proxy.pods[0] || chosen.ProbeIP != proxy.pods[0].PodIP || chosen.ProbeCIDR.IsValid() {
			t.Fatal("API probe was not linked to selected verified Pod")
		}
	}
	for _, name := range []string{"../pod", "pod:8000", "pod%2Fother", "pod/name", "Pod", "-pod", "pod-", strings.Repeat("a", 64)} {
		proxy.pods[0].Name = name
		before := len(proxy.plans)
		if _, err := (Observer{Sources: proxy}).Observe(context.Background(), proxyPlan, head, merge, spec); err == nil || len(proxy.plans) != before {
			t.Fatal("unsafe Pod name selected")
		}
	}
	proxy.pods[0].Name = "canary-rollout"
	proxy.pods[0].Namespace = "other"
	if _, err := (Observer{Sources: proxy}).Observe(context.Background(), proxyPlan, head, merge, spec); err == nil {
		t.Fatal("external namespace selected")
	}
	cases := map[string]func(*Plan, *fakeSources){
		"public endpoint":     func(p *Plan, _ *fakeSources) { p.ProbeIP = netip.MustParseAddr("8.8.8.8") },
		"redirect path":       func(p *Plan, _ *fakeSources) { p.ProbePath = "//other-host" },
		"health only":         func(p *Plan, _ *fakeSources) { p.ProbePath = "/health" },
		"wrong PodIP":         func(_ *Plan, f *fakeSources) { f.pods[0].PodIP = netip.MustParseAddr("10.0.0.9") },
		"wrong release":       func(_ *Plan, f *fakeSources) { f.user.ObservedRelease = "v2" },
		"Argo stale":          func(_ *Plan, f *fakeSources) { f.app.Sources[2].ResolvedRevision = head },
		"public package":      func(_ *Plan, f *fakeSources) { f.proof.Visibility = "public" },
		"wrong CI source":     func(_ *Plan, f *fakeSources) { f.proof.SourceSHA = head },
		"wrong tree":          func(_ *Plan, f *fakeSources) { f.proof.SourceAppTreeSHA = head },
		"wrong workflow":      func(_ *Plan, f *fakeSources) { f.proof.WorkflowSHA = head },
		"wrong workflow ref":  func(_ *Plan, f *fakeSources) { f.proof.WorkflowRef = "refs/tags/other" },
		"wrong workflow hash": func(_ *Plan, f *fakeSources) { f.proof.WorkflowSHA256 = strings.Repeat("0", 64) },
		"wrong run":           func(_ *Plan, f *fakeSources) { f.proof.WorkflowRunID = 43 },
		"wrong dispatch":      func(_ *Plan, f *fakeSources) { f.proof.DispatchID = strings.Repeat("2", 32) },
		"wrong tag":           func(_ *Plan, f *fakeSources) { f.proof.ImageTag = "sense-stale" },
		"wrong digest":        func(_ *Plan, f *fakeSources) { f.proof.Digest = "sha256:" + strings.Repeat("0", 64) },
		"unproven leaf":       func(_ *Plan, f *fakeSources) { f.proof.ChildDigests = nil },
		"mutable pod tag":     func(_ *Plan, f *fakeSources) { f.pods[0].Image = image + ":v1" },
		"unhealthy pod":       func(_ *Plan, f *fakeSources) { f.pods[0].Ready = false },
		"user path failed":    func(_ *Plan, f *fakeSources) { f.user.StatusCode = 401 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			p := plan
			copy := f
			copy.pods = append([]PodState(nil), f.pods...)
			copy.app.Sources = append([]ArgoSource(nil), f.app.Sources...)
			mutate(&p, &copy)
			if _, err := (Observer{Sources: copy}).Observe(context.Background(), p, head, merge, spec); err == nil {
				t.Fatal("unsafe observation accepted")
			}
		})
	}
}

func TestParseReleaseResponseIsBoundedAndStrict(t *testing.T) {
	got, err := parseReleaseResponse([]byte(`{"release":"v1"}`))
	if err != nil || got != "v1" {
		t.Fatalf("valid release=%q %v", got, err)
	}
	for name, body := range map[string]string{
		"missing": `{}`, "non scalar": `{"release":{"secret":"x"}}`, "unknown field": `{"release":"v1","token":"x"}`,
		"trailing": `{"release":"v1"} true`, "unsafe marker": `{"release":"../../secret"}`,
		"oversize": strings.Repeat(" ", 4<<10) + `{"release":"v1"}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseReleaseResponse([]byte(body)); err == nil {
				t.Fatal("unsafe release response accepted")
			}
		})
	}
}

func TestProbeCIDRPlanBoundary(t *testing.T) {
	plan := Plan{SchemaVersion: 1, Namespace: namespace, Application: application, Repository: repository, Image: image, ProbePort: 8080, ProbePath: "/api/release", ExpectedRelease: "v1"}
	for _, cidr := range []string{"10.42.0.0/16", "172.16.0.0/12", "192.168.5.0/24", "10.0.0.8/32"} {
		plan.ProbeCIDR = netip.MustParsePrefix(cidr)
		if err := plan.validate(); err != nil {
			t.Fatalf("private CIDR %s: %v", cidr, err)
		}
	}
	for _, cidr := range []string{"0.0.0.0/0", "10.0.0.0/7", "172.0.0.0/8", "192.168.0.0/15", "8.8.8.0/24", "127.0.0.0/8", "169.254.0.0/16", "100.64.0.0/10", "fd00::/64", "::ffff:10.0.0.0/120", "10.42.0.1/16"} {
		plan.ProbeCIDR = netip.MustParsePrefix(cidr)
		if err := plan.validate(); err == nil {
			t.Fatalf("unbounded/non-private/noncanonical CIDR %s accepted", cidr)
		}
	}
	plan.ProbeCIDR = netip.MustParsePrefix("10.42.0.0/16")
	plan.ProbeIP = netip.MustParseAddr("10.42.0.8")
	if plan.validate() == nil {
		t.Fatal("both static IP and CIDR accepted")
	}
	plan.ProbeIP = netip.Addr{}
	body, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "observer.json")
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	if got, err := LoadPlan(path); err != nil || got != plan {
		t.Fatalf("CIDR plan load %+v %v", got, err)
	}
	plan.ProbeCIDR = netip.Prefix{}
	if plan.validate() == nil {
		t.Fatal("no probe boundary accepted")
	}
}

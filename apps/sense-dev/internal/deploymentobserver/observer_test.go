package deploymentobserver

import (
	"context"
	"encoding/json"
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
func (f fakeSources) Release(context.Context, Plan, string) (ReleaseProof, error) {
	return f.proof, nil
}
func (f fakeSources) Probe(context.Context, Plan) (UserPathState, error) { return f.user, nil }

func TestObserveBindsMergeRevisionPrivateImageAndUserPath(t *testing.T) {
	head, merge := strings.Repeat("a", 40), strings.Repeat("b", 40)
	index, leaf := "sha256:"+strings.Repeat("c", 64), "sha256:"+strings.Repeat("d", 64)
	plan := Plan{SchemaVersion: 1, Namespace: namespace, Application: application, Repository: repository, Image: image, ProbeIP: netip.MustParseAddr("10.0.0.8"), ProbePort: 8080, ProbePath: "/api/release", ExpectedRelease: "v1"}
	sources := []ArgoSource{{RepoURL: "https://github.com/yu-min3/kensan-lab", Path: "charts/app-base", TargetRevision: "main", ResolvedRevision: merge}, {RepoURL: "https://github.com/yu-min3/kensan-lab", Ref: "values", TargetRevision: "main", ResolvedRevision: merge}, {RepoURL: "https://github.com/yu-min3/kensan-lab", Path: "kubernetes/apps/app-canary/resources", TargetRevision: "main", ResolvedRevision: merge}}
	f := fakeSources{app: ApplicationState{Sources: sources, Sync: "Synced", Health: "Healthy"}, pods: []PodState{{Ready: true, Image: image + "@" + index, ImageID: "containerd://" + leaf, PodIP: plan.ProbeIP}}, proof: ReleaseProof{Repository: repository, Image: image, SourceSHA: merge, Digest: index, ChildDigests: []string{leaf}, Visibility: "private", Succeeded: true}, user: UserPathState{StatusCode: 200, Path: "/api/release", ObservedRelease: "v1"}}
	r, err := (Observer{Sources: f}).Observe(context.Background(), plan, head, merge)
	if err != nil || r.HeadSHA != head || r.Revision != merge || r.ImageDigest != index || r.ObservedRelease != "v1" {
		t.Fatalf("valid observation=%+v %v", r, err)
	}
	cases := map[string]func(*Plan, *fakeSources){
		"public endpoint":  func(p *Plan, _ *fakeSources) { p.ProbeIP = netip.MustParseAddr("8.8.8.8") },
		"redirect path":    func(p *Plan, _ *fakeSources) { p.ProbePath = "//other-host" },
		"health only":      func(p *Plan, _ *fakeSources) { p.ProbePath = "/health" },
		"wrong PodIP":      func(_ *Plan, f *fakeSources) { f.pods[0].PodIP = netip.MustParseAddr("10.0.0.9") },
		"wrong release":    func(_ *Plan, f *fakeSources) { f.user.ObservedRelease = "v2" },
		"Argo stale":       func(_ *Plan, f *fakeSources) { f.app.Sources[2].ResolvedRevision = head },
		"public package":   func(_ *Plan, f *fakeSources) { f.proof.Visibility = "public" },
		"wrong CI source":  func(_ *Plan, f *fakeSources) { f.proof.SourceSHA = head },
		"unproven leaf":    func(_ *Plan, f *fakeSources) { f.proof.ChildDigests = nil },
		"mutable pod tag":  func(_ *Plan, f *fakeSources) { f.pods[0].Image = image + ":v1" },
		"unhealthy pod":    func(_ *Plan, f *fakeSources) { f.pods[0].Ready = false },
		"user path failed": func(_ *Plan, f *fakeSources) { f.user.StatusCode = 401 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			p := plan
			copy := f
			copy.pods = append([]PodState(nil), f.pods...)
			copy.app.Sources = append([]ArgoSource(nil), f.app.Sources...)
			mutate(&p, &copy)
			if _, err := (Observer{Sources: copy}).Observe(context.Background(), p, head, merge); err == nil {
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

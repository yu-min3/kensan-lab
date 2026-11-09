package importer

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/agentclient"
)

type routedFake struct {
	label                  string
	creates, gets, actions int
	state                  string
	fail                   bool
}

func (f *routedFake) Create(context.Context, agentclient.Request) (agentclient.Job, error) {
	f.creates++
	if f.fail {
		return agentclient.Job{}, errors.New("unavailable")
	}
	return agentclient.Job{ID: f.label, State: "queued"}, nil
}
func (f *routedFake) Get(_ context.Context, id string) (agentclient.Job, error) {
	f.gets++
	if f.fail {
		return agentclient.Job{}, errors.New("unavailable")
	}
	state := f.state
	if state == "" {
		state = "waiting"
	}
	return agentclient.Job{ID: id, State: state, Output: json.RawMessage(`{"title":"スープ","servings":"","ingredients":["玉ねぎ"],"steps":["煮る"]}`)}, nil
}
func (f *routedFake) Action(_ context.Context, id, action string) (agentclient.Job, error) {
	f.actions++
	if action == "retry" {
		return agentclient.Job{ID: id + "-next", State: "queued"}, nil
	}
	return agentclient.Job{ID: id, State: "cancelled"}, nil
}
func routingFixture(t *testing.T, dir, mode string, direct, platform agentclient.Gateway) *Service {
	t.Helper()
	d := Backend{"direct", "direct-fixed", direct}
	p := Backend{"platform", "platform-fixed", platform}
	current := d
	if mode == "platform" {
		current = p
	}
	s, e := NewRouted(dir, Routing{Current: current, LegacyDirect: d, Retained: []Backend{p}})
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func TestDraftBackendSurvivesFlagSwitchAndUnavailableCredentials(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	direct, platform := &routedFake{label: "direct-job"}, &routedFake{label: "platform-job"}
	s := routingFixture(t, dir, "platform", direct, platform)
	d, e := s.Create(ctx, "", "スープ 玉ねぎ 煮る")
	if e != nil {
		t.Fatal(e)
	}
	d, e = s.Start(ctx, d.ID, d.Revision, "steps")
	if e != nil || d.BackendKind != "platform" || platform.creates != 1 {
		t.Fatal(d, e)
	}
	s = routingFixture(t, dir, "direct", direct, platform)
	d, e = s.Get(ctx, d.ID)
	if e != nil || platform.gets != 1 || direct.gets != 0 {
		t.Fatal("flag rerouted old job", d, e)
	}
	s = routingFixture(t, dir, "direct", direct, nil)
	d, e = s.Get(ctx, d.ID)
	if e != nil || d.Job.State != "waiting" || d.Job.Reason != "backend_unavailable" || d.Source.Text == "" || d.AgentAvailable {
		t.Fatal("unavailable origin lost draft", d, e)
	}
	platform.state = "succeeded"
	s = routingFixture(t, dir, "direct", direct, platform)
	d, e = s.Get(ctx, d.ID)
	if e != nil || d.Content.Title != "スープ" {
		t.Fatal("origin recovery failed", d, e)
	}
	d, e = s.Save(d.ID, d.Revision)
	if e != nil || d.SavedFile == "" {
		t.Fatal("confirmed save failed", d, e)
	}
}
func TestLegacyDraftUsesOnlyDirect(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	direct := &routedFake{label: "legacy-job"}
	s := New(dir, direct)
	d, _ := s.Create(ctx, "", "スープ 玉ねぎ 煮る")
	d, _ = s.Start(ctx, d.ID, d.Revision, "steps")
	raw, _ := os.ReadFile(s.draftPath(d.ID))
	if string(raw) == "" {
		t.Fatal("draft missing")
	}
	platform := &routedFake{label: "new-platform"}
	s = routingFixture(t, dir, "platform", direct, platform)
	d, e := s.Get(ctx, d.ID)
	if e != nil || direct.gets != 1 || platform.gets != 0 {
		t.Fatal("legacy job moved", d, e)
	}
	d, e = s.Action(ctx, d.ID, d.Revision, "cancel")
	if e != nil || direct.actions != 1 || platform.actions != 0 {
		t.Fatal("legacy cancellation moved", d, e)
	}
}
func TestNoAutomaticFallbackAndExplicitFailedPlatformRetry(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	direct, platform := &routedFake{label: "direct"}, &routedFake{label: "platform", fail: true}
	s := routingFixture(t, dir, "platform", direct, platform)
	d, _ := s.Create(ctx, "", "スープ 玉ねぎ 煮る")
	d, e := s.Start(ctx, d.ID, d.Revision, "steps")
	if e == nil || !d.Pending || direct.creates != 0 {
		t.Fatal("failed request fell back", d, e)
	}
	d, e = s.Get(ctx, d.ID)
	if e != nil || d.Source.Text == "" || !d.Pending || direct.creates != 0 {
		t.Fatal("pending recovery lost input", d, e)
	}
	platform.fail = false
	d, e = s.Get(ctx, d.ID)
	if e != nil || d.Pending {
		t.Fatal("pending create did not recover", d, e)
	}
	platform.state = "failed"
	d, e = s.Get(ctx, d.ID)
	if e != nil || d.Job.State != "failed" {
		t.Fatal(d, e)
	}
	d, e = s.Action(ctx, d.ID, d.Revision, "retry")
	if e != nil || d.Job.ID != "platform-next" {
		t.Fatal("new platform attempt refused", d, e)
	}
}

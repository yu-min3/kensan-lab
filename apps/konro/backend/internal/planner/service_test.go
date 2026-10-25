package planner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/agentclient"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/mealplan"
	"os"
	"path/filepath"
	"testing"
)

type fakeAgent struct {
	creates int
	job     agentclient.Job
	output  string
	request agentclient.Request
}

func (a *fakeAgent) Create(_ context.Context, r agentclient.Request) (agentclient.Job, error) {
	a.creates++
	a.request = r
	a.job = agentclient.Job{ID: "job-1", State: "running"}
	return a.job, nil
}
func (a *fakeAgent) Get(_ context.Context, id string) (agentclient.Job, error) {
	return agentclient.Job{ID: id, State: "succeeded", Output: json.RawMessage(a.output)}, nil
}
func (a *fakeAgent) Action(_ context.Context, id, action string) (agentclient.Job, error) {
	state := "running"
	if action == "cancel" {
		state = "cancelled"
	}
	return agentclient.Job{ID: id, State: state}, nil
}
func fixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for i := 1; i <= 3; i++ {
		raw := fmt.Sprintf("---\ntitle: 料理%d\nservings: 2人分\ntags: [副菜]\n---\n\n## 材料\n- 玉ねぎ 1個\n\n## 手順\n1. 煮る\n", i)
		if e := os.WriteFile(filepath.Join(dir, fmt.Sprint(i)+".md"), []byte(raw), 0600); e != nil {
			t.Fatal(e)
		}
	}
	return dir
}
func TestJobRestartAndSnapshot(t *testing.T) {
	root := fixture(t)
	agent := &fakeAgent{output: `{"interpretation":"2品の野菜料理を提案します","menus":[{"picks":[{"file":"1"},{"file":"2"}]}]}`}
	plans := mealplan.New(root)
	s := New(root, agent, plans)
	c := Conditions{Count: 2, Servings: "4", Side: 2, Fixed: []string{"1"}}
	p, e := s.Create(context.Background(), c, "ai")
	if e != nil {
		t.Fatal(e)
	}
	if agent.request.Profile != "text" {
		t.Fatal("unexpected provider boundary")
	}
	again, e := s.Create(context.Background(), c, "ai")
	if e != nil || again.ID != p.ID || agent.creates != 1 {
		t.Fatal("duplicate job", e)
	}
	s = New(root, agent, plans)
	p, e = s.Get(context.Background(), p.ID)
	if e != nil || p.State != "succeeded" || len(p.Menus) != 1 {
		t.Fatalf("%+v %v", p, e)
	}
	plan, e := s.Adopt(p.ID, p.Revision, p.Menus[0].ID, "adopt-one")
	if e != nil || len(plan.Items) != 2 || plan.Items[0].Scale != "2" {
		t.Fatalf("%+v %v", plan, e)
	}
	same, e := s.Adopt(p.ID, p.Revision, p.Menus[0].ID, "adopt-one")
	if e != nil || same.ID != plan.ID {
		t.Fatal("duplicate adoption", e)
	}
	c.Seed = 1
	p, e = s.Create(context.Background(), c, "rules")
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(root, "1.md"), []byte("---\ntitle: 変更\n---\n"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Adopt(p.ID, p.Revision, p.Menus[0].ID, "changed-source"); e == nil {
		t.Fatal("adopted a different recipe version")
	}
}
func TestRejectInvalidOutputAndOneRepair(t *testing.T) {
	root := fixture(t)
	agent := &fakeAgent{output: `{"interpretation":"bad","menus":[{"picks":[{"file":"missing"},{"file":"1"}]}]}`}
	s := New(root, agent, mealplan.New(root))
	p, e := s.Create(context.Background(), Conditions{Count: 2, Servings: "2"}, "ai")
	if e != nil {
		t.Fatal(e)
	}
	p, e = s.Get(context.Background(), p.ID)
	if e != nil || p.State != "failed" || len(p.Menus) != 0 {
		t.Fatal(p, e)
	}
	p, e = s.Action(context.Background(), p.ID, p.Revision, "retry")
	if e != nil || p.Repairs != 1 {
		t.Fatal(p, e)
	}
	p, e = s.Get(context.Background(), p.ID)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Action(context.Background(), p.ID, p.Revision, "retry"); e == nil {
		t.Fatal("unlimited repair")
	}
}

func TestCancelAndStaleJob(t *testing.T) {
	root := fixture(t)
	agent := &fakeAgent{output: `{"menus":[{"picks":[{"file":"1"}]}]}`}
	s := New(root, agent, mealplan.New(root))
	p, err := s.Create(context.Background(), Conditions{Count: 1, Servings: "2"}, "ai")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Action(context.Background(), p.ID, p.Revision+1, "cancel"); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	p, err = s.Action(context.Background(), p.ID, p.Revision, "cancel")
	if err != nil || p.State != "cancelled" {
		t.Fatal(p, err)
	}
	p, err = s.Get(context.Background(), p.ID)
	if err != nil || p.State != "cancelled" || len(p.Menus) != 0 {
		t.Fatal(p, err)
	}
	if _, err = s.Adopt(p.ID, p.Revision, "menu-1", "cancelled"); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
}

func TestAdoptionRejectsChangedPreferenceAndPolicy(t *testing.T) {
	for _, change := range []string{"preference", "policy"} {
		t.Run(change, func(t *testing.T) {
			root := fixture(t)
			s := New(root, nil, mealplan.New(root))
			p, err := s.Create(context.Background(), Conditions{Count: 1, Servings: "2"}, "rules")
			if err != nil {
				t.Fatal(err)
			}
			if change == "preference" {
				err = AtomicJSON(filepath.Join(root, ".settings", "meal-preferences.json"), map[string]any{"revision": 1, "avoid": []string{"玉ねぎ"}})
			} else {
				err = AtomicJSON(filepath.Join(root, "planning-policy.json"), Policy{Version: "new", RatingWeight: 2})
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.Adopt(p.ID, p.Revision, p.Menus[0].ID, "stale"); !errors.Is(err, ErrConflict) {
				t.Fatal(err)
			}
		})
	}
}

package importer

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"strings"
	"testing"

	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/agentclient"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/recipe"
)

func TestJSONLDGraphAndSections(t *testing.T) {
	src := Source{URL: "https://example.org/recipe", Raw: `<h1>other</h1><script type="application/ld+json">{"@graph":[{"@type":"WebPage"},{"@type":["Recipe"],"name":"<b>魚の煮物</b>","recipeYield":"2人分","recipeIngredient":["魚 2切れ","水 100ml"],"recipeInstructions":[{"@type":"HowToSection","itemListElement":[{"@type":"HowToStep","text":"<p>水を沸かす</p>"},{"text":"魚を10分煮る"}]}]}]}</script>`}
	c, w := Extract(src)
	if c.Title != "魚の煮物" || len(c.Ingredients) != 2 || len(c.Steps) != 2 || c.Steps[1] != "魚を10分煮る" || len(w) != 0 {
		t.Fatalf("%+v %v", c, w)
	}
}
func TestSSRF(t *testing.T) {
	for _, ip := range []string{"127.0.0.1", "::1", "::ffff:127.0.0.1", "10.0.0.1", "172.16.1.2", "192.168.0.1", "169.254.169.254", "100.64.0.1", "0.0.0.0", "224.0.0.1", "fc00::1", "fe80::1", "2002:7f00:1::", "64:ff9b::7f00:1"} {
		if PublicIP(net.ParseIP(ip)) {
			t.Errorf("accepted %s", ip)
		}
	}
	for _, ip := range []string{"1.1.1.1", "8.8.8.8", "2606:4700:4700::1111"} {
		if !PublicIP(net.ParseIP(ip)) {
			t.Errorf("rejected %s", ip)
		}
	}
	for _, u := range []string{"file:///etc/passwd", "https://user:pw@example.org", "http://example.org:8080", "http://127.0.0.1", "http://localhost"} {
		if _, e := Fetch(context.Background(), u); e == nil {
			t.Errorf("fetched %s", u)
		}
	}
}

type fakeAgent struct {
	requests []agentclient.Request
	result   agentclient.Job
}

func (a *fakeAgent) Create(_ context.Context, r agentclient.Request) (agentclient.Job, error) {
	a.requests = append(a.requests, r)
	return agentclient.Job{ID: "generic-job", State: "queued"}, nil
}
func (a *fakeAgent) Get(context.Context, string) (agentclient.Job, error) { return a.result, nil }
func (a *fakeAgent) Action(_ context.Context, id, action string) (agentclient.Job, error) {
	return agentclient.Job{ID: id, State: "cancelled"}, nil
}
func TestDraftAgentConfirmAndReload(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	a := &fakeAgent{result: agentclient.Job{ID: "generic-job", State: "succeeded", Output: json.RawMessage(`{"title":"煮魚","servings":"2人分","ingredients":["魚 2切れ"],"steps":["10分煮る"]}`)}}
	s := New(dir, a)
	d, e := s.Create(ctx, "", "煮魚 2人分 魚2切れ 10分煮る")
	if e != nil {
		t.Fatal(e)
	}
	id := d.ID
	d, e = s.Start(ctx, id, d.Revision, "steps")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Save(id, d.Revision); e != ErrConflict {
		t.Fatalf("saved running job: %v", e)
	}
	if _, e = os.Stat(s.draftPath(id)); e != nil {
		t.Fatal(e)
	}
	// Reopen the app's service with only file state, then poll the generic job.
	s = New(dir, a)
	d, e = s.Get(ctx, id)
	if e != nil || d.Content.Title != "煮魚" {
		t.Fatalf("%+v %v", d, e)
	}
	if len(a.requests) != 1 || a.requests[0].Profile != "text" || strings.Contains(a.requests[0].IdempotencyKey, "/") {
		t.Fatal("contract")
	}
	if metas, _ := recipe.List(dir); len(metas) != 0 {
		t.Fatal("saved before confirmation")
	}
	d, e = s.Save(id, d.Revision)
	if e != nil {
		t.Fatal(e)
	}
	r, e := recipe.Load(dir, d.SavedFile)
	if e != nil || r.Title != "煮魚" || r.Steps[0] != "10分煮る" {
		t.Fatalf("%+v %v", r, e)
	}
	if _, e = s.Save(id, 1); e != nil {
		t.Fatal("confirm not idempotent", e)
	}
	if metas, _ := recipe.List(dir); len(metas) != 1 {
		t.Fatal(metas)
	}
	if _, e = s.Edit(id, d.Revision-1, d.Content); e != ErrConflict {
		t.Fatal("stale edit accepted")
	}
}
func TestAgentNumbersAndInvalidRecipeRejected(t *testing.T) {
	for _, out := range []string{`{"title":"魚","servings":"","ingredients":["魚 2切れ"],"steps":["200度で焼く"]}`, `{"title":"魚","servings":"","ingredients":["魚"],"steps":["焼く"],"execute":"command"}`, `{"title":"魚","ingredients":["魚"],"steps":["焼く"]}`, `{"title":"魚","servings":null,"ingredients":["魚"],"steps":["焼く"]}`, `{"title":"魚","ingredients":[],"steps":[]}`} {
		a := &fakeAgent{result: agentclient.Job{ID: "generic-job", State: "succeeded", Output: json.RawMessage(out)}}
		s := New(t.TempDir(), a)
		d, _ := s.Create(context.Background(), "", "魚 2切れ 焼く")
		d, _ = s.Start(context.Background(), d.ID, d.Revision, "faithful")
		d, e := s.Get(context.Background(), d.ID)
		if e != nil || d.Job.State != "failed" || d.Content.Title != "" {
			t.Fatalf("accepted %s: %+v %v", out, d, e)
		}
	}
}
func TestManualAndNamespace(t *testing.T) {
	s := New(t.TempDir(), nil)
	d, _ := s.Create(context.Background(), "", "some source")
	d, e := s.Edit(d.ID, d.Revision, Content{Title: `魚 "焼き"`, Ingredients: []string{"魚 1切れ"}, Steps: []string{"焼く"}})
	if e != nil {
		t.Fatal(e)
	}
	d, e = s.Save(d.ID, d.Revision)
	if e != nil {
		t.Fatal(e)
	}
	r, e := recipe.Load(s.dir, d.SavedFile)
	if e != nil || r.Title != `魚 "焼き"` {
		t.Fatal(r, e)
	}
	for _, id := range []string{"../x", "imported/../../x", "imported/abc", "imported/" + strings.Repeat("a", 32) + "/other"} {
		if _, e := recipe.Load(s.dir, id); e == nil {
			t.Fatal("path accepted", id)
		}
	}
}

func TestManualDetachesLateOutput(t *testing.T) {
	ctx := context.Background()
	a := &fakeAgent{result: agentclient.Job{ID: "generic-job", State: "succeeded", Output: json.RawMessage(`{"title":"late","servings":"","ingredients":["fish"],"steps":["cook"]}`)}}
	s := New(t.TempDir(), a)
	d, _ := s.Create(ctx, "", "fish cook")
	d, _ = s.Start(ctx, d.ID, d.Revision, "steps")
	d, e := s.Manual(d.ID, d.Revision)
	if e != nil {
		t.Fatal(e)
	}
	d, e = s.Edit(d.ID, d.Revision, Content{Title: "manual", Ingredients: []string{"fish"}, Steps: []string{"cook"}})
	if e != nil {
		t.Fatal(e)
	}
	d, e = s.Get(ctx, d.ID)
	if e != nil || d.Content.Title != "manual" {
		t.Fatal("late result overwrote manual work", e)
	}
	if _, e = s.Save(d.ID, d.Revision); e != nil {
		t.Fatal(e)
	}
}

func TestURLSchemeCompletion(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"www.sirogohan.com/recipe/oyakodon/", "https://www.sirogohan.com/recipe/oyakodon/"},
		{" example.org/recipe?a=1#step ", "https://example.org/recipe?a=1"},
		{"//example.org/recipe", "https://example.org/recipe"},
		{"http://example.org/recipe", "http://example.org/recipe"},
		{"https://example.org/recipe", "https://example.org/recipe"},
		{"example.org:443/recipe", "https://example.org:443/recipe"},
	} {
		t.Run(tc.in, func(t *testing.T) {
			u, err := validURL(tc.in)
			if err != nil || u.String() != tc.want {
				t.Fatalf("%v %v", u, err)
			}
		})
	}
	for _, in := range []string{"", "javascript:alert(1)", "file:///etc/passwd", "user:pw@example.org", "example.org:8080/recipe"} {
		if _, err := validURL(in); err == nil {
			t.Errorf("accepted %q", in)
		}
	}
	for _, in := range []string{"127.0.0.1/recipe", "localhost/recipe", "//192.168.0.1/recipe"} {
		if _, err := Fetch(context.Background(), in); err == nil {
			t.Errorf("fetched private URL %q", in)
		}
	}
}

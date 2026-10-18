package discovery

import (
	"context"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/creatorprefs"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/webpage"
	"strings"
	"testing"
)

func TestResolveProfilesAndRejectWrongIdentity(t *testing.T) {
	s := New(func(ctx context.Context, u string) (webpage.Page, error) {
		body := `<script>var ytInitialData={"metadata":{"channelMetadataRenderer":{"externalId":"UCabcdefghijklmnopqrstuv","title":"新しい料理人"}}};</script>`
		if strings.Contains(u, "kurashiru") {
			body = `<h1 class="ChefsShow-name">別の料理人</h1>`
		}
		return webpage.Page{URL: u, HTML: body}, nil
	})
	c, err := s.ResolveCreator(context.Background(), "youtube.com/@newchef")
	if err != nil || c.ID != "youtube:UCabcdefghijklmnopqrstuv" || c.URL != "https://www.youtube.com/channel/UCabcdefghijklmnopqrstuv" {
		t.Fatal(c, err)
	}
	c, err = s.ResolveCreator(context.Background(), "https://www.kurashiru.com/chefs/15")
	if err != nil || c.ID != "kurashiru:chef:15" {
		t.Fatal(c, err)
	}
	if _, err = s.ResolveCreator(context.Background(), "https://www.youtube.com/channel/UCLcqrpFhRlJnm6etuLQblWg"); err == nil {
		t.Fatal("channel mismatch accepted")
	}
	for _, raw := range []string{"http://127.0.0.1/private", "https://youtube.com.evil.test/@newchef", "https://user:pw@youtube.com/@newchef", "https://youtube.com/watch?v=h67vdKlPGzg", "https://kurashiru.com/recipes/6938e7e7-dbcc-4da0-8760-f1286f19684e", "https://youtube.com:8443/@newchef"} {
		if _, err := s.ResolveCreator(context.Background(), raw); err == nil {
			t.Fatal("invalid profile", raw)
		}
	}
}
func TestPreferencesDriveGeneralizedSearch(t *testing.T) {
	store := creatorprefs.New(t.TempDir())
	snap, _ := store.Get()
	for _, c := range append([]creatorprefs.Creator(nil), snap.Creators...) {
		next, err := store.Remove(snap.Revision, c.ID)
		if err != nil {
			t.Fatal(err)
		}
		snap = next.Snapshot
	}
	custom := creatorprefs.Creator{ID: "youtube:UCabcdefghijklmnopqrstuv", Name: "新しい料理人", URL: "https://www.youtube.com/channel/UCabcdefghijklmnopqrstuv"}
	saved, err := store.Add(snap.Revision, custom)
	if err != nil {
		t.Fatal(err)
	}
	s := New(func(ctx context.Context, u string) (webpage.Page, error) {
		return webpage.Page{URL: u, HTML: ytPage(ytResult("abcdefghijk", "UCabcdefghijklmnopqrstuv", "なすの料理") + "," + ytResult("h67vdKlPGzg", "UCLcqrpFhRlJnm6etuLQblWg", "なすの料理"))}, nil
	}).WithPreferences(store)
	result, err := s.Search(context.Background(), "なす", "youtube")
	if err != nil || len(result.Results) != 2 || !result.Results[0].Preferred || result.Results[1].Preferred {
		t.Fatal(result, err)
	}
	result, err = s.SearchCreator(context.Background(), "なす", custom.ID)
	if err != nil || len(result.Results) != 1 || result.Results[0].Info.Creator.ID != custom.ID {
		t.Fatal(result, err)
	}
	if _, err = s.SearchCreator(context.Background(), "なす", "youtube:UCLcqrpFhRlJnm6etuLQblWg"); err == nil {
		t.Fatal("nonfavorite selected")
	}
	if _, err = store.Remove(saved.Revision, custom.ID); err != nil {
		t.Fatal(err)
	}
	result, err = s.Search(context.Background(), "なす", "youtube")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range result.Results {
		if r.Preferred {
			t.Fatal("removed preference still applied")
		}
	}
}

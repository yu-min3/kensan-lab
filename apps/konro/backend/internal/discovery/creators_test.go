package discovery

import (
	"context"
	"fmt"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/provenance"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/webpage"
	"strings"
	"sync/atomic"
	"testing"
)

func ytResult(id, owner, title string) string {
	return fmt.Sprintf(`{"videoRenderer":{"videoId":%q,"title":{"simpleText":%q},"longBylineText":{"runs":[{"text":"投稿者","navigationEndpoint":{"browseEndpoint":{"browseId":%q}}}]}}}`, id, title, owner)
}
func ytPage(items string) string {
	return `<script>var ytInitialData={"contents":{"twoColumnSearchResultsRenderer":{"primaryContents":{"sectionListRenderer":{"contents":[{"itemSectionRenderer":{"contents":[` + items + `]}}]}}}}};</script>`
}
func TestFavoriteSearchUsesIdentityAndRelevance(t *testing.T) {
	s := New(func(ctx context.Context, u string) (webpage.Page, error) {
		body := ytPage(ytResult("abcdefghijk", "UCabcdefghijklmnopqrstuv", "通常のなす料理"))
		if strings.Contains(u, "%E7%AC%A0%E5%8E%9F") {
			body = ytPage(ytResult("h67vdKlPGzg", "UCLcqrpFhRlJnm6etuLQblWg", "なすの煮物") + "," + ytResult("otherperson", "UCabcdefghijklmnopqrstuv", "笠原将弘のなす再現") + "," + ytResult("unrelated00", "UCLcqrpFhRlJnm6etuLQblWg", "牛肉の煮物"))
		}
		if strings.Contains(u, "/chefs/") {
			body = `<h1 class="ChefsShow-name">中村和成</h1><ul class="ChefsShow-videosList"><li><a class="thumbnail-wrapper" href="/recipes/3b6424d5-6080-4399-a88e-fc2f87e9081e"><img alt="ナスのカルパッチョ" src="https://example.com/nasu.jpg"></a></li></ul><aside><a href="/recipes/256ae2fe-a3e8-4b3b-90c2-f3b60e728241"><img alt="なすのおすすめ"></a></aside>`
		}
		return webpage.Page{URL: u, HTML: body}, nil
	})
	r, err := s.Search(context.Background(), "なす", "all")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	favorites := 0
	for _, result := range r.Results {
		if seen[result.URL] {
			t.Fatal("duplicate", result)
		}
		seen[result.URL] = true
		if result.Preferred {
			favorites++
			if result.Title != "なすの煮物" && result.Title != "ナスのカルパッチョ" {
				t.Fatal("wrong favorite", result)
			}
		}
	}
	if favorites != 2 || !r.Results[0].Preferred || !r.Results[1].Preferred {
		t.Fatalf("favorites %d: %+v", favorites, r.Results)
	}
	if seen["https://www.youtube.com/watch?v=otherperson"] || seen["https://www.youtube.com/watch?v=unrelated00"] {
		t.Fatal("extra unrelated or impersonator")
	}
}
func TestPreviewCacheAndRedirectIdentity(t *testing.T) {
	var calls atomic.Int32
	s := New(func(ctx context.Context, u string) (webpage.Page, error) {
		calls.Add(1)
		return webpage.Page{URL: u, HTML: `<script type="application/ld+json">{"@type":"Recipe","author":{"name":"本人"},"aggregateRating":{"ratingValue":4.5,"ratingCount":25}}</script>`}, nil
	})
	for i := 0; i < 2; i++ {
		info, err := s.Preview(context.Background(), "https://delishkitchen.tv/recipes/123")
		if err != nil || info.Rating.Count != 25 {
			t.Fatal(info, err)
		}
	}
	if calls.Load() != 1 {
		t.Fatal("no cache", calls.Load())
	}
	if _, err := s.Preview(context.Background(), "https://127.0.0.1/private"); err == nil {
		t.Fatal("bad url")
	}
	s = New(func(ctx context.Context, u string) (webpage.Page, error) {
		return webpage.Page{URL: "https://delishkitchen.tv/recipes/999", HTML: `<script type="application/ld+json">{"@type":"Recipe","author":{"name":"別の料理"}}</script>`}, nil
	})
	if info, err := s.Preview(context.Background(), "https://delishkitchen.tv/recipes/123"); err != nil || info != nil {
		t.Fatal("redirect credit", info, err)
	}
}
func TestRankKeepsRegularOrderAndUnratedRecipes(t *testing.T) {
	base := []Result{{URL: "a"}, {URL: "b", Info: &provenance.Info{Rating: &provenance.Rating{Value: 5, Best: 5}}}, {URL: "c"}}
	out := rank(base, nil, nil)
	if len(out) != 3 || out[0].URL != "a" || out[1].URL != "b" || out[2].URL != "c" {
		t.Fatal(out)
	}
}

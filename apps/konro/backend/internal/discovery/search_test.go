package discovery

import (
	"context"
	"errors"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/webpage"
	"strings"
	"testing"
)

func TestCanonicalRejectsUntrustedURLs(t *testing.T) {
	for _, raw := range []string{"https://delishkitchen.tv.evil.test/recipes/123", "https://delishkitchen.tv@127.0.0.1/recipes/123", "javascript:alert(1)", "https://delishkitchen.tv:8443/recipes/123", "https://www.sirogohan.com/recipe/index/", "https://youtu.be/not-video", "https://www.youtube.com/channel/123", "https://127.0.0.1/recipes/123"} {
		if u, ok := Canonical(raw); ok {
			t.Errorf("accepted %s as %s", raw, u)
		}
	}
	for _, raw := range []string{"youtu.be/h67vdKlPGzg?t=123", "https://m.youtube.com/shorts/h67vdKlPGzg"} {
		u, ok := Canonical(raw)
		if !ok || u != "https://www.youtube.com/watch?v=h67vdKlPGzg" {
			t.Fatalf("canonical %s: %s %v", raw, u, ok)
		}
	}
}
func TestSearchOnlyUsesResultsAndKnownRecipePaths(t *testing.T) {
	for _, tc := range []struct{ site, container string }{
		{"delish", `class="delish-search"`}, {"kurashiru", `class="SearchIndex-DlyMasonry"`}, {"sirogohan", `id="recipe" class="recipe_list"`}, {"nhk", `class="recipe--sort-row"`}, {"ajinomoto", `class="search-results-area"`}, {"cookpad", `id="recipe_123"`}, {"macaroni", `class="search"`},
	} {
		path := map[string]string{"delish": "/recipes/123", "kurashiru": "/recipes/82b3201b-29f4-4db9-9fbb-82a1ce16d247", "sirogohan": "/recipe/nikujaga/", "nhk": "/recipe/123_test.html", "ajinomoto": "/recipe/card/123/", "cookpad": "/jp/recipes/123", "macaroni": "/123"}[tc.site]
		base := searchURL(tc.site, "test")
		outside := `<aside><a href="` + path + `">おすすめ</a></aside>`
		r := parseResults(webpage.Page{URL: base, HTML: outside}, tc.site)
		if len(r) != 0 {
			t.Fatalf("%s recommended link mistaken for result", tc.site)
		}
		doc := outside + `<section ` + tc.container + `><a href="` + path + `"><img alt="検索の料理">料理</a><a href="` + path + `">重複</a><a href="https://127.0.0.1/">不正</a></section>`
		r = parseResults(webpage.Page{URL: base, HTML: doc}, tc.site)
		if len(r) != 1 || r[0].Title != "検索の料理" {
			t.Fatalf("%s: %+v", tc.site, r)
		}
	}
}
func TestSearchPartialFailureAndQueryValidation(t *testing.T) {
	s := New(func(ctx context.Context, u string) (webpage.Page, error) {
		if strings.Contains(u, "delishkitchen") {
			return webpage.Page{URL: u, HTML: `<div class="delish-search"><a href="/recipes/987">試験の料理</a></div>`}, nil
		}
		return webpage.Page{}, errors.New("unavailable")
	})
	r, e := s.Search(context.Background(), "試験", "all")
	if e != nil || len(r.Results) != 1 || len(r.Warnings) != 4 {
		t.Fatalf("%+v %v", r, e)
	}
	for _, q := range []string{"", strings.Repeat("あ", 101), "a\nb"} {
		if _, e := s.Search(context.Background(), q, "all"); !errors.Is(e, ErrQuery) {
			t.Fatal("bad query accepted")
		}
	}
	if _, e := s.Search(context.Background(), "魚", "unknown"); e == nil {
		t.Fatal("unknown site")
	}
}
func TestYouTubeSearchParsesJSONWithoutEvaluatingScript(t *testing.T) {
	doc := `<script>var ytInitialData = {"contents":{"twoColumnSearchResultsRenderer":{"primaryContents":{"sectionListRenderer":{"contents":[{"itemSectionRenderer":{"contents":[{"videoRenderer":{"videoId":"h67vdKlPGzg","title":{"runs":[{"text":"動画の料理"}]}}},{"videoRenderer":{"videoId":"bad","title":{"simpleText":"無効"}}}]}}]}}}}};alert('must not run')</script>`
	r := parseResults(webpage.Page{URL: searchURL("youtube", "試験"), HTML: doc}, "youtube")
	if len(r) != 1 || r[0].Title != "動画の料理" {
		t.Fatalf("%+v", r)
	}
}
func TestSearchLargeHTMLIsStreamed(t *testing.T) {
	body := `<section class="delish-search">` + strings.Repeat("<span>x</span>", 150000) + `<a href="/recipes/987">試験の料理</a></section>`
	r := parseResults(webpage.Page{URL: searchURL("delish", "試験"), HTML: body}, "delish")
	if len(r) != 1 {
		t.Fatal(r)
	}
}

func TestSearchTitleDoesNotBecomeBadgeLabel(t *testing.T) {
	page := webpage.Page{URL: searchURL("delish", "魚"), HTML: `<section class="delish-search"><a href="/recipes/123"><img alt="魚の写真"><img alt="きょうの献立"><p class="item-card__title text-bold">検索した魚料理</p><p>材料や評価</p></a></section>`}
	r := parseResults(page, "delish")
	if len(r) != 1 || r[0].Title != "検索した魚料理" {
		t.Fatal(r)
	}
}

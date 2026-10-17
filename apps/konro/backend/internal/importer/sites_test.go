package importer

import (
	"context"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/webpage"
	"strings"
	"testing"
)

func TestSirogohanDOMRetainsQuantitiesAndParagraphs(t *testing.T) {
	raw := `<h1 id="recipe-name">試験丼のレシピ/作り方</h1><section class="material"><h2 class="material-ttl"><span>2人分</span></h2><ul><li>魚 <b>100g</b></li><li>水 200ml</li></ul></section><section class="howto"><div class="howto-block"><p>魚を切る。</p><p>水を加え10分煮る。</p></div></section><aside>広告 999g</aside>`
	c, w := Extract(Source{URL: "https://www.sirogohan.com/recipe/test/", Raw: raw})
	if c.Title != "試験丼" || c.Servings != "2人分" || len(c.Ingredients) != 2 || len(c.Steps) != 2 || len(w) != 1 || strings.Contains(strings.Join(c.Ingredients, " "), "999") {
		t.Fatalf("%+v %v", c, w)
	}
}
func TestYouTubeOnlyUsesDescriptionAndMissingSourceCannotStart(t *testing.T) {
	raw := `<script>var ytInitialPlayerResponse={"videoDetails":{"videoId":"h67vdKlPGzg","title":"試験動画","shortDescription":"材料\\n魚 100g\\n水 200ml"}};</script><main>広告の材料999g</main>`
	s, e := sourceFromPage(webpage.Page{URL: "https://www.youtube.com/watch?v=h67vdKlPGzg", HTML: raw})
	if e != nil || s.Kind != "youtube_description" || strings.Contains(s.Text, "999") {
		t.Fatalf("%+v %v", s, e)
	}
	c, w := Extract(s)
	if c.Title != "試験動画" || len(c.Steps) != 0 || len(w) != 1 {
		t.Fatal(c, w)
	}
	service := New(t.TempDir(), &fakeAgent{})
	d, e := service.Create(context.Background(), "", "placeholder")
	if e != nil {
		t.Fatal(e)
	}
	d.Source = Source{URL: s.URL, Kind: "youtube_description"}
	if e = service.write(d); e != nil {
		t.Fatal(e)
	}
	if _, e = service.Start(context.Background(), d.ID, d.Revision, "steps"); e == nil {
		t.Fatal("AI started without description")
	}
}

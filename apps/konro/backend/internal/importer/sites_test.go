package importer

import (
	"context"
	"encoding/json"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/agentclient"
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

func TestIncompleteAIKeepsIngredientsWithoutInventingSteps(t *testing.T) {
	a := &fakeAgent{result: agentclient.Job{ID: "generic-job", State: "succeeded", Output: json.RawMessage(`{"title":"試験の料理","servings":"","ingredients":["魚 100g"],"steps":[]}`)}}
	s := New(t.TempDir(), a)
	ctx := context.Background()
	d, e := s.Create(ctx, "", "試験の料理 魚100g")
	if e != nil {
		t.Fatal(e)
	}
	d.Source.Kind = "youtube_description"
	d.Warnings = []string{"概要欄のみ取得。字幕・音声・映像は取得していません"}
	if e = s.write(d); e != nil {
		t.Fatal(e)
	}
	d, e = s.Start(ctx, d.ID, d.Revision, "steps")
	if e != nil {
		t.Fatal(e)
	}
	d, e = s.Get(ctx, d.ID)
	if e != nil {
		t.Fatal(e)
	}
	if d.Job.State != "succeeded" || len(d.Content.Ingredients) != 1 || len(d.Content.Steps) != 0 {
		t.Fatalf("%+v", d)
	}
	warnings := strings.Join(d.Warnings, " ")
	if !strings.Contains(warnings, "不足") || !strings.Contains(warnings, "字幕・") {
		t.Fatal(warnings)
	}
	if _, e = s.Save(d.ID, d.Revision); e == nil {
		t.Fatal("incomplete draft saved as recipe")
	}
}

func TestPartialAINeverClearsExistingInput(t *testing.T) {
	a := &fakeAgent{result: agentclient.Job{ID: "generic-job", State: "succeeded", Output: json.RawMessage(`{"title":"","servings":"","ingredients":[],"steps":[]}`)}}
	s := New(t.TempDir(), a)
	ctx := context.Background()
	d, e := s.Create(ctx, "", "魚 100g 10分煮る")
	if e != nil {
		t.Fatal(e)
	}
	original := Content{Title: "手入力した料理", Ingredients: []string{"魚 100g"}, Steps: []string{"10分煮る"}}
	d, e = s.Edit(d.ID, d.Revision, original)
	if e != nil {
		t.Fatal(e)
	}
	d, e = s.Start(ctx, d.ID, d.Revision, "steps")
	if e != nil {
		t.Fatal(e)
	}
	d, e = s.Get(ctx, d.ID)
	if e != nil {
		t.Fatal(e)
	}
	if d.Content.Title != original.Title || len(d.Content.Ingredients) != 1 || d.Content.Ingredients[0] != original.Ingredients[0] || len(d.Content.Steps) != 1 || d.Content.Steps[0] != original.Steps[0] {
		t.Fatalf("input lost: %+v", d.Content)
	}
	if !strings.Contains(strings.Join(d.Warnings, " "), "入力済みの内容を残しました") {
		t.Fatal(d.Warnings)
	}
}

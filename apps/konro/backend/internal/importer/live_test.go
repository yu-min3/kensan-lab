package importer

import (
	"context"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/discovery"
	"os"
	"testing"
)

// Opt-in contract probe: live pages never run in normal CI and are not fixtures.
func TestLiveRecipeSites(t *testing.T) {
	if os.Getenv("KONRO_LIVE_TEST") != "1" {
		t.Skip("set KONRO_LIVE_TEST=1 for public site contract checks")
	}
	for _, url := range []string{
		"https://delishkitchen.tv/recipes/153240773708480781",
		"https://www.kurashiru.com/recipes/738b3ed8-5baf-4766-9c60-b66dc4b8b75d",
		"https://www.sirogohan.com/recipe/oyakodon/",
		"https://www.kyounoryouri.jp/recipe/20026_肉じゃが.html",
		"https://park.ajinomoto.co.jp/recipe/card/702434/",
		"https://www.kikkoman.co.jp/homecook/search/recipe/00004691/",
		"https://cookpad.com/jp/recipes/17564487",
		"https://macaro-ni.jp/162481",
		"https://youtu.be/h67vdKlPGzg",
	} {
		t.Run(discovery.SiteID(url), func(t *testing.T) {
			s, err := Fetch(context.Background(), url)
			if err != nil {
				t.Fatal(err)
			}
			c, w := Extract(s)
			t.Logf("title=%q ingredients=%d steps=%d sourceBytes=%d warnings=%d", c.Title, len(c.Ingredients), len(c.Steps), len(s.Text), len(w))
			if c.Title == "" || s.Text == "" {
				t.Fatal("missing source")
			}
			if s.Kind != "youtube_description" && (len(c.Ingredients) == 0 || len(c.Steps) == 0) {
				t.Fatal("missing recipe content")
			}
		})
	}
}

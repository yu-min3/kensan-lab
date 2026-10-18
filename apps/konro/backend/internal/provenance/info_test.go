package provenance

import (
	"strings"
	"testing"
)

func TestRecipeCreditRatingAndRelatedPeople(t *testing.T) {
	raw := `<div><div><h1>料理</h1><p>紹介</p></div><div><a href="/chefs/6"><img alt="中村和成"></a><a href="/chefs/6">中村和成</a><a href="/chefs/6">LA BONNE TABLE / シェフ</a></div></div><footer><a href="/chefs/999">別の料理人</a></footer><script type="application/ld+json">{"@type":"Recipe","url":"https://www.kurashiru.com/recipes/abc","author":{"name":"クラシル公式"},"aggregateRating":{"ratingValue":4.55,"bestRating":5,"ratingCount":486}}</script>`
	info := FromHTML(raw, "https://www.kurashiru.com/recipes/abc")
	if info == nil || info.Creator.ID != "kurashiru:chef:6" || info.Creator.Name != "中村和成" || info.Publisher.Name != "クラシル公式" || info.Rating.Value != 4.55 || info.Rating.Count != 486 {
		t.Fatalf("%+v", info)
	}
	// A footer-only mention must never override the real source credit.
	info = FromHTML(`<h1>別の料理</h1><footer><a href="/chefs/6">中村和成</a></footer>`+raw[strings.Index(raw, `<script`):], "https://www.kurashiru.com/recipes/abc")
	if info.Creator.ID != "" || info.Creator.Name != "クラシル公式" {
		t.Fatal(info)
	}
}
func TestOnlyCurrentRecipeHasRating(t *testing.T) {
	raw := `<script type="application/ld+json">{"@graph":[{"@type":"ItemList","itemListElement":[{"@type":"Recipe","author":{"name":"別人"},"aggregateRating":{"ratingValue":5}}]},{"@type":"Recipe","url":"https://example.com/other","author":{"name":"別人"}},{"@type":"Recipe","url":"https://example.com/current#recipe","author":{"name":"本人"},"aggregateRating":{"ratingValue":"4.3","reviewCount":12,"bestRating":5}}]}</script>`
	info := FromHTML(raw, "https://example.com/current")
	if info.Creator.Name != "本人" || info.Rating.Count != 12 {
		t.Fatal(info)
	}
	if FromHTML(`<script type="application/ld+json">{"@type":"ItemList","itemListElement":[{"@type":"Recipe","author":{"name":"別人"}}]}</script>`, "https://example.com/current") != nil {
		t.Fatal("recommended recipe accepted")
	}
	for _, r := range []string{`{"ratingValue":6,"bestRating":5}`, `{"ratingValue":-1}`, `{"ratingValue":4,"ratingCount":-1}`} {
		got := FromHTML(`<script type="application/ld+json">{"@type":"Recipe","aggregateRating":`+r+`}</script>`, "https://example.com")
		if got != nil && got.Rating != nil {
			t.Fatal("invalid rating", got)
		}
	}
}
func TestVideoCreditMatchesVideo(t *testing.T) {
	raw := `<script>var ytInitialPlayerResponse={"videoDetails":{"videoId":"h67vdKlPGzg","channelId":"UCLcqrpFhRlJnm6etuLQblWg","author":"笠原将弘"}};</script>`
	if got := FromHTML(raw, "https://www.youtube.com/watch?v=h67vdKlPGzg"); got == nil || got.Creator.ID != "youtube:UCLcqrpFhRlJnm6etuLQblWg" {
		t.Fatal(got)
	}
	if got := FromHTML(raw, "https://www.youtube.com/watch?v=other"); got != nil {
		t.Fatal("wrong video credit", got)
	}
}

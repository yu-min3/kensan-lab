package importer

import (
	"context"
	"encoding/json"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/discovery"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/webpage"
	"strings"
	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/provenance"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/recipe"
)

const MaxSource = webpage.MaxSource
const MaxText = 128 << 10

type Source struct {
	URL  string           `json:"url,omitempty"`
	Raw  string           `json:"raw"`
	Text string           `json:"text"`
	Kind string           `json:"kind,omitempty"`
	Info *provenance.Info `json:"info,omitempty"`
}
type Content = recipe.Content

// PublicIP remains available to callers validating import targets.
var PublicIP = webpage.PublicIP
var validURL = webpage.NormalizeURL

func Fetch(ctx context.Context, raw string) (Source, error) {
	if canonical, ok := discovery.Canonical(raw); ok {
		raw = canonical
	}
	p, err := webpage.Fetch(ctx, raw)
	if err != nil {
		return Source{}, err
	}
	if discovery.SiteID(raw) == "youtube" && discovery.SiteID(p.URL) != "youtube" {
		return Source{URL: raw, Raw: p.HTML, Kind: "youtube_description"}, nil
	}
	return sourceFromPage(p)
}
func sourceFromPage(p webpage.Page) (Source, error) {
	s := Source{URL: p.URL, Raw: p.HTML}
	doc, e := goquery.NewDocumentFromReader(strings.NewReader(s.Raw))
	if e != nil {
		return Source{}, e
	}
	s.Info = provenance.FromDocument(doc, s.URL)
	if discovery.SiteID(s.URL) == "youtube" {
		title, description, ok := youtubeDescription(s.Raw, s.URL)
		s.Kind = "youtube_description"
		if !ok {
			s.Text = ""
			return s, nil
		}
		s.Text = limitText(title + "\n\n" + description)
		return s, nil
	}
	selection := recipeSelection(doc, s.URL)
	selection.Find("script,style,nav,header,footer,iframe,noscript").Remove()
	s.Text = limitText(selection.Text())
	return s, nil
}
func limitText(s string) string {
	if len(s) <= MaxText {
		return s
	}
	// Truncate only at a UTF-8 boundary.
	b := []byte(s)[:MaxText]
	for len(b) > 0 && !utf8.Valid(b) {
		b = b[:len(b)-1]
	}
	return string(b)
}

func clean(s string) string {
	doc, e := goquery.NewDocumentFromReader(strings.NewReader("<body>" + s + "</body>"))
	if e != nil {
		return strings.Join(strings.Fields(s), " ")
	}
	return strings.Join(strings.Fields(doc.Find("body").Text()), " ")
}
func stringsOf(v any) []string {
	var out []string
	switch x := v.(type) {
	case string:
		if s := clean(x); s != "" {
			out = append(out, s)
		}
	case []any:
		for _, v := range x {
			out = append(out, stringsOf(v)...)
		}
	case map[string]any:
		if sub, ok := x["itemListElement"]; ok {
			out = append(out, stringsOf(sub)...)
		} else if sub, ok := x["text"]; ok {
			out = append(out, stringsOf(sub)...)
		}
	}
	return out
}
func Extract(s Source) (Content, []string) {
	c := Content{Ingredients: []string{}, Steps: []string{}}
	warnings := []string{}
	if s.URL == "" {
		return c, []string{"本文をAIで抽出するか、材料と手順を入力してください"}
	}
	doc, e := goquery.NewDocumentFromReader(strings.NewReader(s.Raw))
	if e != nil {
		return c, []string{"ページを解析できません"}
	}
	if c, w, ok := extractSite(s, doc); ok {
		return c, w
	}
	var recipes []map[string]any
	var walk func(any, int)
	walk = func(v any, depth int) {
		if depth > 32 {
			return
		}
		switch x := v.(type) {
		case []any:
			for _, v := range x {
				walk(v, depth+1)
			}
		case map[string]any:
			isRecipe := false
			for _, t := range stringsOf(x["@type"]) {
				if t == "Recipe" || t == "https://schema.org/Recipe" || t == "http://schema.org/Recipe" {
					isRecipe = true
				}
			}
			if isRecipe {
				recipes = append(recipes, x)
				return
			}
			for _, v := range x {
				walk(v, depth+1)
			}
		}
	}
	doc.Find(`script[type="application/ld+json"]`).Each(func(_ int, el *goquery.Selection) {
		var v any
		if json.Unmarshal([]byte(el.Text()), &v) == nil {
			walk(v, 0)
		}
	})
	if len(recipes) == 0 {
		c.Title = clean(doc.Find("h1").First().Text())
		return c, []string{"構造化レシピがありません。AIで抽出するか、本文から入力してください"}
	}
	if len(recipes) > 1 {
		warnings = append(warnings, "複数のレシピが見つかりました。最初のレシピを表示しています。原文を確認してください")
	}
	r := recipes[0]
	if v, ok := r["name"].(string); ok {
		c.Title = clean(v)
	}
	c.Servings = strings.Join(stringsOf(r["recipeYield"]), " / ")
	if n, ok := r["recipeYield"].(float64); ok {
		b, _ := json.Marshal(n)
		c.Servings = string(b)
	}
	c.Ingredients = stringsOf(r["recipeIngredient"])
	c.Steps = stringsOf(r["recipeInstructions"])
	if len(c.Ingredients) == 0 || len(c.Steps) == 0 {
		warnings = append(warnings, "材料または手順をすべて取得できていません。取り込み元を確認し、AIまたは手入力で補ってください")
	}
	return c, warnings
}

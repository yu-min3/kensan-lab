package importer

import (
	"github.com/PuerkitoBio/goquery"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/discovery"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/webpage"
	"strings"
)

func recipeSelection(doc *goquery.Document, rawURL string) *goquery.Selection {
	var selection *goquery.Selection
	switch discovery.SiteID(rawURL) {
	case "sirogohan":
		selection = doc.Find(".material, .howto")
	case "delish":
		selection = doc.Find(".recipe-content__main")
	case "kurashiru":
		selection = doc.Find("main")
	}
	if selection == nil || selection.Length() == 0 {
		selection = doc.Find("main,article").First()
	}
	if selection.Length() == 0 {
		selection = doc.Find("body")
	}
	return selection.Clone()
}
func youtubeDescription(raw, rawURL string) (title, description string, ok bool) {
	v, ok := webpage.ScriptObject(raw, "ytInitialPlayerResponse")
	if !ok {
		return "", "", false
	}
	details, ok := v["videoDetails"].(map[string]any)
	if !ok {
		return "", "", false
	}
	id, _ := details["videoId"].(string)
	canonical, valid := discovery.Canonical(rawURL)
	if !valid || id == "" || !strings.HasSuffix(canonical, "v="+id) {
		return "", "", false
	}
	title, _ = details["title"].(string)
	description, _ = details["shortDescription"].(string)
	return title, description, strings.TrimSpace(description) != ""
}
func extractSite(s Source, doc *goquery.Document) (Content, []string, bool) {
	c := Content{Ingredients: []string{}, Steps: []string{}}
	switch discovery.SiteID(s.URL) {
	case "youtube":
		title, _, ok := youtubeDescription(s.Raw, s.URL)
		c.Title = title
		if !ok {
			return c, []string{"YouTubeの概要欄を取得できませんでした。概要欄・材料・手順を本文として貼り付けてください。字幕・音声・映像は取得していません"}, true
		}
		return c, []string{"YouTubeのタイトル・概要欄を取得しました。AIで材料・手順を抽出できます。字幕・音声・映像は取得していません。概要欄にない手順は原文で補ってください"}, true
	case "sirogohan":
		c.Title = strings.TrimSuffix(clean(doc.Find("#recipe-name").Text()), "のレシピ/作り方")
		c.Servings = clean(doc.Find(".material-ttl span").First().Text())
		doc.Find(".material ul li").Each(func(_ int, el *goquery.Selection) {
			if v := clean(el.Text()); v != "" {
				c.Ingredients = append(c.Ingredients, v)
			}
		})
		doc.Find(".howto-block p").Each(func(_ int, el *goquery.Selection) {
			if v := clean(el.Text()); v != "" {
				c.Steps = append(c.Steps, v)
			}
		})
		if len(c.Ingredients) > 0 && len(c.Steps) > 0 {
			return c, []string{"白ごはん.comの材料と説明段落を読み取りました。材料の注記や補足も取り込み元で確認してください。必要ならAIで操作ごとに整形できます"}, true
		}
	}
	return c, nil, false
}

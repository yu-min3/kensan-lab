package discovery

import (
	"context"
	"encoding/xml"
	"errors"
	"net/url"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/webpage"
)

type Result struct {
	Title    string `json:"title"`
	URL      string `json:"url"`
	Site     string `json:"site"`
	SiteName string `json:"siteName"`
}
type Response struct {
	Results  []Result `json:"results"`
	Warnings []string `json:"warnings"`
}
type Fetcher func(context.Context, string) (webpage.Page, error)
type Service struct {
	fetch Fetcher
	slots chan struct{}
}

func New(fetch Fetcher) *Service {
	if fetch == nil {
		fetch = func(ctx context.Context, raw string) (webpage.Page, error) {
			if strings.HasPrefix(raw, "https://www.bing.com/search?") {
				return webpage.FetchSearch(ctx, raw, true)
			}
			return webpage.FetchSearch(ctx, raw, false)
		}
	}
	return &Service{fetch: fetch, slots: make(chan struct{}, 2)}
}

var ErrQuery = errors.New("料理名や材料を100文字以内で入力してください")

func searchURL(site, q string) string {
	switch site {
	case "delish":
		return "https://delishkitchen.tv/search?q=" + url.QueryEscape(q)
	case "kurashiru":
		return "https://www.kurashiru.com/search?query=" + url.QueryEscape(q)
	case "sirogohan":
		return "https://www.sirogohan.com/recipe/index/keyword:" + url.PathEscape(q)
	case "nhk":
		return "https://www.kyounoryouri.jp/search/recipe?keyword=" + url.QueryEscape(q)
	case "ajinomoto":
		return "https://park.ajinomoto.co.jp/search/recipe/?word=" + url.QueryEscape(q)
	case "kikkoman":
		return "https://www.bing.com/search?format=rss&setlang=ja-JP&cc=jp&q=" + url.QueryEscape(q+" レシピ site:kikkoman.co.jp/homecook/search/recipe/")
	case "cookpad":
		return "https://cookpad.com/jp/search/" + url.PathEscape(q)
	case "macaroni":
		return "https://macaro-ni.jp/search?q=" + url.QueryEscape(q)
	case "youtube":
		return "https://www.youtube.com/results?search_query=" + url.QueryEscape(q+" レシピ")
	}
	return ""
}
func (s *Service) Search(ctx context.Context, q, site string) (Response, error) {
	out := Response{Results: []Result{}, Warnings: []string{}}
	q = strings.TrimSpace(q)
	if q == "" || utf8.RuneCountInString(q) > 100 || !utf8.ValidString(q) || strings.ContainsAny(q, "\r\n\x00") {
		return out, ErrQuery
	}
	selected := []string{site}
	if site == "" || site == "all" {
		selected = []string{"delish", "kurashiru", "sirogohan", "nhk", "youtube"}
	} else if Name(site) == "" {
		return out, ErrQuery
	}
	groups := make([][]Result, len(selected))
	warnings := make([]string, len(selected))
	var wg sync.WaitGroup
	for i, id := range selected {
		wg.Add(1)
		go func(i int, id string) {
			defer wg.Done()
			select {
			case s.slots <- struct{}{}:
			case <-ctx.Done():
				warnings[i] = Name(id) + "の検索は時間切れです"
				return
			}
			defer func() { <-s.slots }()
			page, err := s.fetch(ctx, searchURL(id, q))
			if err != nil {
				warnings[i] = Name(id) + "を検索できませんでした。時間をおいて試すか、URLから取り込んでください"
				return
			}
			groups[i] = parseResults(page, id)
			if len(groups[i]) == 0 {
				warnings[i] = Name(id) + "では候補を取得できませんでした。別の言葉かURL取り込みを試してください"
			}
		}(i, id)
	}
	wg.Wait()
	// Round-robin keeps a large site's list from hiding the other sources.
	seen := map[string]bool{}
	for n := 0; n < 8; n++ {
		for _, group := range groups {
			if n < len(group) && !seen[group[n].URL] {
				seen[group[n].URL] = true
				out.Results = append(out.Results, group[n])
			}
		}
	}
	for _, w := range warnings {
		if w != "" {
			out.Warnings = append(out.Warnings, w)
		}
	}
	return out, nil
}
func parseResults(page webpage.Page, site string) []Result {
	results := []Result{}
	seen := map[string]bool{}
	add := func(raw, title string) {
		base, err := url.Parse(page.URL)
		if err != nil {
			return
		}
		u, err := base.Parse(raw)
		if err != nil {
			return
		}
		canonical, ok := Canonical(u.String())
		if !ok || SiteID(canonical) != site || seen[canonical] {
			return
		}
		title = strings.Join(strings.Fields(title), " ")
		if title == "" || len([]rune(title)) > 300 {
			return
		}
		seen[canonical] = true
		results = append(results, Result{title, canonical, site, Name(site)})
	}
	if site == "youtube" {
		if v, ok := webpage.ScriptObject(page.HTML, "ytInitialData"); ok {
			var walk func(any, int)
			walk = func(v any, depth int) {
				if depth > 40 || len(results) >= 8 {
					return
				}
				switch x := v.(type) {
				case map[string]any:
					if r, ok := x["videoRenderer"].(map[string]any); ok {
						id, _ := r["videoId"].(string)
						add("https://www.youtube.com/watch?v="+id, jsonText(r["title"]))
					}
					for _, key := range []string{"contents", "twoColumnSearchResultsRenderer", "primaryContents", "sectionListRenderer", "itemSectionRenderer", "richItemRenderer", "content"} {
						if child, ok := x[key]; ok {
							walk(child, depth+1)
						}
					}
				case []any:
					for _, child := range x {
						walk(child, depth+1)
					}
				}
			}
			walk(v, 0)
		}
		return results
	}
	if site == "kikkoman" {
		var feed struct {
			Items []struct {
				Title string `xml:"title"`
				Link  string `xml:"link"`
			} `xml:"channel>item"`
		}
		if xml.Unmarshal([]byte(page.HTML), &feed) == nil {
			for _, item := range feed.Items {
				add(item.Link, item.Title)
			}
		}
		if len(results) > 8 {
			results = results[:8]
		}
		return results
	}

	resultLinks(page.HTML, site, add, func() bool { return len(results) >= 8 })
	return results
}
func jsonText(v any) string {
	x, ok := v.(map[string]any)
	if !ok {
		return ""
	}
	if text, ok := x["simpleText"].(string); ok {
		return text
	}
	out := ""
	if runs, ok := x["runs"].([]any); ok {
		for _, run := range runs {
			if r, ok := run.(map[string]any); ok {
				t, _ := r["text"].(string)
				out += t
			}
		}
	}
	return out
}

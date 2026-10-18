// Package provenance reads source credits and public ratings without inference.
// These values are distinct from the owner's personal recipe rating.
package provenance

import (
	"encoding/json"
	"github.com/PuerkitoBio/goquery"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/webpage"
	"net/url"
	"strconv"
	"strings"
)

type Person struct {
	ID   string `json:"id,omitempty" yaml:"id,omitempty"`
	Name string `json:"name" yaml:"name"`
	URL  string `json:"url,omitempty" yaml:"url,omitempty"`
}
type Rating struct {
	Value float64 `json:"value" yaml:"value"`
	Best  float64 `json:"best" yaml:"best"`
	Count int     `json:"count,omitempty" yaml:"count,omitempty"`
}
type Info struct {
	Creator   *Person `json:"creator,omitempty" yaml:"creator,omitempty"`
	Publisher *Person `json:"publisher,omitempty" yaml:"publisher,omitempty"`
	Rating    *Rating `json:"rating,omitempty" yaml:"rating,omitempty"`
}

func text(s string) string { return strings.Join(strings.Fields(s), " ") }
func number(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case string:
		f, _ := strconv.ParseFloat(x, 64)
		return f
	}
	return 0
}
func person(v any) *Person {
	if a, ok := v.([]any); ok && len(a) > 0 {
		return person(a[0])
	}
	if s, ok := v.(string); ok && text(s) != "" {
		return &Person{Name: text(s)}
	}
	if m, ok := v.(map[string]any); ok {
		n, _ := m["name"].(string)
		u, _ := m["url"].(string)
		if text(n) != "" {
			return &Person{Name: text(n), URL: safeURL(u)}
		}
	}
	return nil
}
func safeURL(s string) string {
	u, e := url.Parse(s)
	if e != nil || u.User != nil || u.Host == "" || u.Scheme != "https" {
		return ""
	}
	return u.String()
}
func FromHTML(raw, source string) *Info {
	doc, e := goquery.NewDocumentFromReader(strings.NewReader(raw))
	if e != nil {
		return nil
	}
	return FromDocument(doc, source)
}
func FromDocument(doc *goquery.Document, source string) *Info {
	out := &Info{}
	var candidates []map[string]any
	var walk func(any, int)
	walk = func(v any, depth int) {
		if depth > 12 {
			return
		}
		switch x := v.(type) {
		case []any:
			for _, c := range x {
				walk(c, depth+1)
			}
		case map[string]any:
			kind := x["@type"]
			recipe := kind == "Recipe" || kind == "https://schema.org/Recipe" || kind == "http://schema.org/Recipe"
			if kinds, ok := kind.([]any); ok {
				for _, k := range kinds {
					if k == "Recipe" {
						recipe = true
					}
				}
			}
			if recipe {
				candidates = append(candidates, x)
				return
			}
			// Only document/graph roots. ItemList, review and recommendation subtrees
			// cannot provide the current recipe's author or rating.
			walk(x["@graph"], depth+1)
			walk(x["mainEntity"], depth+1)
		}
	}
	doc.Find(`script[type="application/ld+json"]`).Each(func(_ int, s *goquery.Selection) {
		var v any
		if json.Unmarshal([]byte(s.Text()), &v) == nil {
			walk(v, 0)
		}
	})
	var first map[string]any
	for _, c := range candidates {
		raw, _ := c["url"].(string)
		if raw == "" {
			raw, _ = c["@id"].(string)
		}
		if m, ok := c["mainEntityOfPage"].(map[string]any); ok {
			if id, ok := m["@id"].(string); ok {
				raw = id
			}
		}
		if raw != "" && samePage(raw, source) {
			first = c
			break
		}
		if raw == "" && len(candidates) == 1 {
			first = c
		}
	}

	if first != nil {
		out.Creator = person(first["author"])
		if r, ok := first["aggregateRating"].(map[string]any); ok {
			value, best, count := number(r["ratingValue"]), number(r["bestRating"]), number(r["ratingCount"])
			if best == 0 {
				best = 5
			}
			if count == 0 {
				count = number(r["reviewCount"])
			}
			if value > 0 && best > 0 && best <= 100 && value <= best && count >= 0 && count <= 1000000000 {
				out.Rating = &Rating{value, best, int(count)}
			}
		}
	}
	u, _ := url.Parse(source)
	if u != nil && u.Hostname() == "www.youtube.com" {
		doc.Find("script").Each(func(_ int, s *goquery.Selection) {
			if v, ok := webpage.ScriptObject(s.Text(), "ytInitialPlayerResponse"); ok {
				d, _ := v["videoDetails"].(map[string]any)
				id, _ := d["channelId"].(string)
				name, _ := d["author"].(string)
				videoID, _ := d["videoId"].(string)
				if videoID == u.Query().Get("v") && videoID != "" && strings.HasPrefix(id, "UC") && len(id) == 24 && name != "" {
					out.Creator = &Person{ID: "youtube:" + id, Name: name, URL: "https://www.youtube.com/channel/" + id}
				}
			}
		})
	}
	if u != nil && u.Hostname() == "www.kurashiru.com" {
		// The credit block immediately follows the title/intro. Never scan footer
		// or related-recipe links for people, and require avatar + matching name.
		credit := doc.Find("h1").First().Parent().Next()
		var chef *Person
		ambiguous := false
		credit.Find(`a[href^="/chefs/"]`).Each(func(_ int, a *goquery.Selection) {
			href := a.AttrOr("href", "")
			parts := strings.Split(strings.Trim(href, "/"), "/")
			if len(parts) != 2 || parts[0] != "chefs" {
				return
			}
			name := text(a.Find("img").First().AttrOr("alt", ""))
			if name == "" || len([]rune(name)) > 100 {
				return
			}
			match := false
			credit.Find("a").Each(func(_ int, b *goquery.Selection) {
				if b.AttrOr("href", "") == href && text(b.Text()) == name {
					match = true
				}
			})
			if !match {
				return
			}
			p := &Person{ID: "kurashiru:chef:" + parts[1], Name: name, URL: "https://www.kurashiru.com" + href}
			if chef != nil && chef.ID != p.ID {
				ambiguous = true
			}
			chef = p
		})
		if chef != nil && !ambiguous {
			out.Publisher = out.Creator
			out.Creator = chef
		}
	}

	if out.Creator == nil && out.Publisher == nil && out.Rating == nil {
		return nil
	}
	return out
}

func samePage(a, b string) bool {
	clean := func(s string) string {
		u, e := url.Parse(s)
		if e != nil {
			return ""
		}
		u.Fragment = ""
		u.RawQuery = ""
		u.Scheme = "https"
		return strings.TrimRight(u.String(), "/")
	}
	return clean(a) == clean(b)
}

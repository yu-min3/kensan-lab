package discovery

import (
	"context"
	"github.com/PuerkitoBio/goquery"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/provenance"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/webpage"
	"golang.org/x/text/unicode/norm"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode"
)

// Stable source IDs, verified against official profiles on 2026-10-18.
var favoriteCreators = []struct{ ID, Name, Query string }{
	{"youtube:UCLcqrpFhRlJnm6etuLQblWg", "笠原将弘", "笠原将弘"},
	{"youtube:UCP2gnyy_-ToZeIDw6qeI6HA", "ジョージ（城二郎）", "ジョージ 城二郎"},
	{"kurashiru:chef:6", "中村和成", ""},
}

func preferred(info *provenance.Info) bool {
	if info == nil || info.Creator == nil {
		return false
	}
	for _, c := range favoriteCreators {
		if info.Creator.ID == c.ID {
			return true
		}
	}
	return false
}
func channelPerson(v any) *provenance.Person {
	m, _ := v.(map[string]any)
	runs, _ := m["runs"].([]any)
	for _, run := range runs {
		r, _ := run.(map[string]any)
		e, _ := r["navigationEndpoint"].(map[string]any)
		b, _ := e["browseEndpoint"].(map[string]any)
		id, _ := b["browseId"].(string)
		name, _ := r["text"].(string)
		if strings.HasPrefix(id, "UC") && len(id) == 24 && name != "" {
			return &provenance.Person{ID: "youtube:" + id, Name: name, URL: "https://www.youtube.com/channel/" + id}
		}
	}
	return nil
}
func folded(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'ァ' && r <= 'ヶ' {
			r -= 0x60
		}
		return unicode.ToLower(r)
	}, norm.NFKC.String(s))
}
func queryMatches(q, title string) bool {
	title = folded(title)
	for _, word := range strings.Fields(folded(q)) {
		if !strings.Contains(title, word) {
			return false
		}
	}
	return true
}
func (s *Service) fetchPage(ctx context.Context, raw string) (webpage.Page, error) {
	select {
	case s.slots <- struct{}{}:
	case <-ctx.Done():
		return webpage.Page{}, ctx.Err()
	}
	defer func() { <-s.slots }()
	return s.fetch(ctx, raw)
}
func (s *Service) favoriteResults(ctx context.Context, q, site string) ([][]Result, bool) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	groups := make([][]Result, 3)
	var wg sync.WaitGroup
	var mu sync.Mutex
	failed := false
	for i, c := range favoriteCreators {
		if i < 2 && site != "all" && site != "" && site != "youtube" {
			continue
		}
		if i == 2 && site != "all" && site != "" && site != "kurashiru" {
			continue
		}
		wg.Add(1)
		go func(i int, id, name, term string) {
			defer wg.Done()
			var candidates []Result
			var err error
			if i < 2 {
				var p webpage.Page
				p, err = s.fetchPage(ctx, searchURL("youtube", q+" "+term))
				if err == nil {
					candidates = parseResults(p, "youtube")
				}
			} else {
				candidates, err = s.chefCatalog(ctx)
			}
			if err != nil {
				mu.Lock()
				failed = true
				mu.Unlock()
			}
			for _, r := range candidates {
				if r.Info == nil || r.Info.Creator == nil || r.Info.Creator.ID != id || !queryMatches(q, r.Title) {
					continue
				}
				r.Preferred = true
				groups[i] = append(groups[i], r)
				if len(groups[i]) == 3 {
					break
				}
			}
		}(i, c.ID, c.Name, c.Query)
	}
	wg.Wait()
	return groups, failed
}
func (s *Service) chefCatalog(ctx context.Context) ([]Result, error) {
	s.chefMu.Lock()
	defer s.chefMu.Unlock()
	if time.Now().Before(s.chefUntil) {
		return append([]Result(nil), s.chefResults...), nil
	}
	var out []Result
	for page := 1; page <= 3; page++ {
		raw := "https://www.kurashiru.com/chefs/6"
		if page > 1 {
			raw += "?page=" + string(rune('0'+page))
		}
		p, err := s.fetchPage(ctx, raw)
		if err != nil {
			return out, err
		}
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(p.HTML))
		if err != nil {
			return out, err
		}
		if u, e := url.Parse(p.URL); e != nil || u.Hostname() != "www.kurashiru.com" || u.Path != "/chefs/6" || strings.TrimSpace(doc.Find(".ChefsShow-name").First().Text()) != "中村和成" {
			return out, ErrQuery
		}
		// Only this official chef's own recipe list, never page recommendations.
		doc.Find(`.ChefsShow-videosList a.thumbnail-wrapper[href^="/recipes/"]`).Each(func(_ int, a *goquery.Selection) {
			href, _ := a.Attr("href")
			canonical, ok := Canonical("https://www.kurashiru.com" + href)
			if !ok {
				return
			}
			img := a.Find("img").First()
			title := strings.TrimSpace(img.AttrOr("alt", ""))
			if title == "" {
				title = strings.TrimSpace(a.Text())
			}
			if title == "" || len([]rune(title)) > 300 {
				return
			}
			image := img.AttrOr("src", "")
			if image == "" {
				a.Find("noscript").Each(func(_ int, n *goquery.Selection) {
					d, _ := goquery.NewDocumentFromReader(strings.NewReader(n.Text()))
					if d != nil {
						image = d.Find("img").First().AttrOr("src", "")
					}
				})
			}
			out = append(out, Result{Title: title, URL: canonical, Site: "kurashiru", SiteName: Name("kurashiru"), Image: previewImage(p.URL, image), Info: &provenance.Info{Creator: &provenance.Person{ID: "kurashiru:chef:6", Name: "中村和成", URL: "https://www.kurashiru.com/chefs/6"}}})
		})
	}
	s.chefResults = out
	s.chefUntil = time.Now().Add(15 * time.Minute)
	return append([]Result(nil), out...), nil
}
func rank(base []Result, groups [][]Result) []Result {
	out := []Result{}
	seen := map[string]bool{}
	add := func(r Result) {
		if !seen[r.URL] {
			seen[r.URL] = true
			out = append(out, r)
		}
	}
	for n := 0; n < 3; n++ {
		for _, g := range groups {
			if n < len(g) {
				add(g[n])
			}
		}
	}
	for _, r := range base {
		if preferred(r.Info) {
			r.Preferred = true
			add(r)
		}
	}
	for _, r := range base {
		add(r)
	}
	return out
}

// Preview is metadata-only. All network access keeps the existing public-URL
// validation; no source body, inference or recipe write occurs here.
type previewEntry struct {
	info  *provenance.Info
	until time.Time
}

func (s *Service) Preview(ctx context.Context, raw string) (*provenance.Info, error) {
	canonical, ok := Canonical(raw)
	if !ok {
		return nil, ErrQuery
	}
	u, _ := url.Parse(canonical)
	if SiteID(canonical) == "youtube" {
		return nil, nil
	}
	s.previewMu.Lock()
	entry, hit := s.previews[canonical]
	s.previewMu.Unlock()
	if hit && time.Now().Before(entry.until) {
		return entry.info, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	p, err := s.fetchPage(ctx, u.String())
	if err != nil {
		return nil, err
	}
	final, ok := Canonical(p.URL)
	if !ok || final != canonical {
		return nil, nil
	}
	info := provenance.FromHTML(p.HTML, p.URL)
	s.previewMu.Lock()
	defer s.previewMu.Unlock()
	if len(s.previews) >= 128 {
		for k := range s.previews {
			delete(s.previews, k)
			break
		}
	}
	s.previews[canonical] = previewEntry{info, time.Now().Add(10 * time.Minute)}
	return info, nil
}

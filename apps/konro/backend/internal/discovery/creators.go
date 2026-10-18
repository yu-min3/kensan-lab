package discovery

import (
	"context"
	"github.com/PuerkitoBio/goquery"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/creatorprefs"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/provenance"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/webpage"
	"golang.org/x/text/unicode/norm"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

// The old three creators are migration defaults, not the runtime preference list.
var favoriteCreators = creatorprefs.Initial()

func (s *Service) Creators() ([]creatorprefs.Creator, error) {
	if s.preferences == nil {
		return creatorprefs.Initial(), nil
	}
	snap, err := s.preferences.Get()
	return snap.Creators, err
}
func preferred(info *provenance.Info, creators []creatorprefs.Creator) bool {
	if info == nil || info.Creator == nil {
		return false
	}
	for _, c := range creators {
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

func creatorSite(c creatorprefs.Creator) string {
	if strings.HasPrefix(c.ID, "youtube:") {
		return "youtube"
	}
	if strings.HasPrefix(c.ID, "kurashiru:chef:") {
		return "kurashiru"
	}
	return ""
}
func (s *Service) creatorResults(ctx context.Context, q string, c creatorprefs.Creator, limit int) ([]Result, error) {
	var candidates []Result
	var err error
	switch creatorSite(c) {
	case "youtube":
		term := strings.NewReplacer("（", " ", "）", " ").Replace(c.Name)
		var p webpage.Page
		p, err = s.fetchPage(ctx, searchURL("youtube", q+" "+term))
		if err == nil {
			candidates = parseResults(p, "youtube")
		}
	case "kurashiru":
		candidates, err = s.chefCatalog(ctx, c)
	default:
		return nil, creatorprefs.ErrInvalid
	}
	out := []Result{}
	seen := map[string]bool{}
	for _, r := range candidates {
		if r.Info == nil || r.Info.Creator == nil || r.Info.Creator.ID != c.ID || !queryMatches(q, r.Title) || seen[r.URL] {
			continue
		}
		r.Preferred = true
		seen[r.URL] = true
		out = append(out, r)
		if len(out) == limit {
			break
		}
	}
	return out, err
}
func (s *Service) favoriteResults(ctx context.Context, q, site string, creators []creatorprefs.Creator) ([][]Result, bool) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	chosen := []creatorprefs.Creator{}
	for _, c := range creators {
		if c.Additional && (site == "all" || site == "" || site == creatorSite(c)) {
			chosen = append(chosen, c)
			if len(chosen) == 3 {
				break
			}
		}
	}
	groups := make([][]Result, len(chosen))
	var wg sync.WaitGroup
	var mu sync.Mutex
	failed := false
	for i, c := range chosen {
		wg.Add(1)
		go func(i int, c creatorprefs.Creator) {
			defer wg.Done()
			results, err := s.creatorResults(ctx, q, c, 3)
			groups[i] = results
			if err != nil {
				mu.Lock()
				failed = true
				mu.Unlock()
			}
		}(i, c)
	}
	wg.Wait()
	return groups, failed
}

type chefEntry struct {
	results []Result
	until   time.Time
}

func (s *Service) chefCatalog(ctx context.Context, c creatorprefs.Creator) ([]Result, error) {
	s.chefMu.Lock()
	defer s.chefMu.Unlock()
	if entry, ok := s.chefs[c.ID]; ok && time.Now().Before(entry.until) {
		return append([]Result(nil), entry.results...), nil
	}
	if !creatorprefs.Valid(c) || creatorSite(c) != "kurashiru" {
		return nil, creatorprefs.ErrInvalid
	}
	expected, _ := url.Parse(c.URL)
	out := []Result{}
	for page := 1; page <= 3; page++ {
		raw := c.URL
		if page > 1 {
			raw += "?page=" + strconv.Itoa(page)
		}
		p, err := s.fetchPage(ctx, raw)
		if err != nil {
			return out, err
		}
		actual, e := url.Parse(p.URL)
		if e != nil || actual.Hostname() != expected.Hostname() || actual.Path != expected.Path {
			return out, creatorprefs.ErrInvalid
		}
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(p.HTML))
		if err != nil {
			return out, err
		}
		name := strings.Join(strings.Fields(doc.Find(".ChefsShow-name").First().Text()), " ")
		if name == "" || len([]rune(name)) > 120 {
			return out, creatorprefs.ErrInvalid
		}
		doc.Find(`.ChefsShow-videosList a.thumbnail-wrapper[href^="/recipes/"]`).Each(func(_ int, a *goquery.Selection) {
			href := a.AttrOr("href", "")
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
			out = append(out, Result{Title: title, URL: canonical, Site: "kurashiru", SiteName: Name("kurashiru"), Image: previewImage(p.URL, image), Info: &provenance.Info{Creator: &provenance.Person{ID: c.ID, Name: name, URL: c.URL}}})
		})
	}
	if len(s.chefs) >= 32 {
		for id := range s.chefs {
			delete(s.chefs, id)
			break
		}
	}
	s.chefs[c.ID] = chefEntry{out, time.Now().Add(15 * time.Minute)}
	return append([]Result(nil), out...), nil
}
func rank(base []Result, groups [][]Result, creators []creatorprefs.Creator) []Result {
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
		if preferred(r.Info, creators) {
			r.Preferred = true
			add(r)
		}
	}
	for _, r := range base {
		r.Preferred = preferred(r.Info, creators)
		add(r)
	}
	return out
}
func (s *Service) SearchCreator(ctx context.Context, q, id string) (Response, error) {
	out := Response{Results: []Result{}, Warnings: []string{}}
	if !validQuery(q) {
		return out, ErrQuery
	}
	creators, err := s.Creators()
	if err != nil {
		return out, err
	}
	for _, c := range creators {
		if c.ID == id {
			ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
			defer cancel()
			results, err := s.creatorResults(ctx, strings.TrimSpace(q), c, 8)
			out.Results = results
			if err != nil {
				out.Warnings = append(out.Warnings, "この作り手の候補を取得できませんでした。時間をおいて、もう一度試してください")
			}
			return out, nil
		}
	}
	return out, creatorprefs.ErrInvalid
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

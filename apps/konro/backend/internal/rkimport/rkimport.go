// Package rkimport converts a Recipe Keeper export zip (recipes.html +
// images/) into konro's file-based recipe representation.
//
// Recipe Keeper has no official format spec. The structure assumed here is
// reverse-engineered from third-party importers (Mealie PR #3642): one
// recipes.html where each recipe is a `div.recipe-details` and every field is
// an element carrying an `itemprop` attribute (meta/content for scalar
// values, block elements for multi-line text, img/src for photos).
// Because the assumption may drift from real exports, every itemprop we do
// not recognize is collected into Result.UnknownProps instead of being
// silently dropped — validating those against a real export is the whole
// point of the Phase 0 PoC.
package rkimport

import (
	"archive/zip"
	"fmt"
	"io"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

type Recipe struct {
	RKID        string // Recipe Keeper internal UUID — stable upsert key for re-imports
	Title       string
	Ingredients []string
	Steps       []string
	PrepTime    string // ISO8601 duration (PT15M)
	CookTime    string
	Yield       string
	Collections []string // user-curated collections (the primary tags)
	Categories  []string
	Courses     []string
	Rating      string
	Source      string
	Notes       string
	Images      []string // paths as referenced from recipes.html
}

type Result struct {
	Recipes      []Recipe
	UnknownProps map[string]int // itemprop -> occurrence count
	Warnings     []string
}

// ParseZip reads a Recipe Keeper export zip. Image bytes are returned keyed
// by the path used in the HTML (resolved relative to recipes.html) so the
// caller can materialize them next to the generated markdown.
func ParseZip(zipPath string) (*Result, map[string][]byte, error) {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, nil, fmt.Errorf("open zip: %w", err)
	}
	defer zr.Close()

	var htmlFile *zip.File
	entries := map[string]*zip.File{}
	for _, f := range zr.File {
		entries[f.Name] = f
		if strings.EqualFold(path.Base(f.Name), "recipes.html") {
			htmlFile = f
		}
	}
	if htmlFile == nil {
		return nil, nil, fmt.Errorf("recipes.html not found in %s (entries: %d)", zipPath, len(zr.File))
	}

	rc, err := htmlFile.Open()
	if err != nil {
		return nil, nil, fmt.Errorf("open recipes.html: %w", err)
	}
	defer rc.Close()

	res, err := ParseHTML(rc)
	if err != nil {
		return nil, nil, err
	}

	htmlDir := path.Dir(htmlFile.Name)
	images := map[string][]byte{}
	for _, r := range res.Recipes {
		for _, img := range r.Images {
			resolved := path.Clean(path.Join(htmlDir, img))
			f, ok := entries[resolved]
			if !ok {
				res.Warnings = append(res.Warnings, fmt.Sprintf("%s: image %q not found in zip", r.Title, img))
				continue
			}
			irc, err := f.Open()
			if err != nil {
				res.Warnings = append(res.Warnings, fmt.Sprintf("%s: image %q: %v", r.Title, img, err))
				continue
			}
			data, err := io.ReadAll(irc)
			irc.Close()
			if err != nil {
				res.Warnings = append(res.Warnings, fmt.Sprintf("%s: image %q: %v", r.Title, img, err))
				continue
			}
			images[img] = data
		}
	}
	return res, images, nil
}

// ParseHTML parses the recipes.html document itself.
func ParseHTML(r io.Reader) (*Result, error) {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}

	res := &Result{UnknownProps: map[string]int{}}
	details := doc.Find("div.recipe-details")
	if details.Length() == 0 {
		res.Warnings = append(res.Warnings, "no div.recipe-details found — export format differs from the assumed structure")
	}
	details.Each(func(i int, sel *goquery.Selection) {
		rec := parseRecipe(sel, res)
		if rec.Title == "" {
			rec.Title = fmt.Sprintf("untitled-%d", i+1)
			res.Warnings = append(res.Warnings, fmt.Sprintf("recipe #%d has no title", i+1))
		}
		res.Recipes = append(res.Recipes, rec)
	})
	return res, nil
}

var photoProp = regexp.MustCompile(`^photo\d+$`)

func parseRecipe(sel *goquery.Selection, res *Result) Recipe {
	var rec Recipe
	sel.Find("[itemprop]").Each(func(_ int, s *goquery.Selection) {
		prop, _ := s.Attr("itemprop")
		val := propValue(s)
		switch {
		case prop == "name":
			rec.Title = strings.TrimSpace(val)
		case prop == "recipeId":
			rec.RKID = strings.TrimSpace(val)
		case prop == "recipeIngredient" || prop == "recipeIngredients":
			rec.Ingredients = append(rec.Ingredients, splitLines(val)...)
		case prop == "recipeInstructions" || prop == "recipeDirections":
			rec.Steps = append(rec.Steps, splitSteps(val)...)
		case prop == "prepTime":
			rec.PrepTime = strings.TrimSpace(val)
		case prop == "performTime" || prop == "cookTime":
			rec.CookTime = strings.TrimSpace(val)
		case prop == "recipeYield":
			rec.Yield = strings.TrimSpace(val)
		case prop == "recipeCollection":
			if v := strings.TrimSpace(val); v != "" {
				rec.Collections = append(rec.Collections, v)
			}
		case prop == "recipeCategory":
			if v := strings.TrimSpace(val); v != "" {
				rec.Categories = append(rec.Categories, v)
			}
		case prop == "recipeCourse":
			if v := strings.TrimSpace(val); v != "" {
				rec.Courses = append(rec.Courses, v)
			}
		case prop == "recipeRating":
			rec.Rating = strings.TrimSpace(val)
		case prop == "recipeSource":
			// real exports wrap the source in <span><a href=...>; prefer the href
			if href, ok := s.Find("a[href]").First().Attr("href"); ok {
				rec.Source = strings.TrimSpace(href)
			} else {
				rec.Source = strings.TrimSpace(val)
			}
		case prop == "recipeNotes":
			rec.Notes = strings.TrimSpace(val)
		case photoProp.MatchString(prop):
			if src, ok := s.Attr("src"); ok && src != "" {
				rec.Images = append(rec.Images, src)
			}
		case strings.HasPrefix(prop, "recipeNut"):
			// nutrition — known but out of scope for konro
		case prop == "recipeIsFavourite" || prop == "recipeShareId":
			// known bookkeeping fields, irrelevant
		default:
			res.UnknownProps[prop]++
		}
	})
	if rec.Title == "" {
		rec.Title = strings.TrimSpace(sel.Find("h2").First().Text())
	}
	return rec
}

// propValue extracts the value of an itemprop-carrying element: meta uses the
// content attribute, img the src, anything else its block-aware text.
func propValue(s *goquery.Selection) string {
	switch goquery.NodeName(s) {
	case "meta":
		v, _ := s.Attr("content")
		return v
	case "img":
		v, _ := s.Attr("src")
		return v
	default:
		var b strings.Builder
		for _, n := range s.Nodes {
			blockText(n, &b)
		}
		return b.String()
	}
}

var blockTags = map[string]bool{
	"p": true, "div": true, "li": true, "br": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "tr": true,
}

// blockText walks the node tree collecting text, inserting newlines at block
// element boundaries. goquery's Text() would glue <p>a</p><p>b</p> into "ab",
// destroying the line structure that ingredients/steps depend on.
func blockText(n *html.Node, b *strings.Builder) {
	if n.Type == html.TextNode {
		b.WriteString(n.Data)
		return
	}
	if n.Type == html.ElementNode && blockTags[n.Data] {
		b.WriteString("\n")
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		blockText(c, b)
	}
	if n.Type == html.ElementNode && blockTags[n.Data] {
		b.WriteString("\n")
	}
}

func splitLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}

var stepNumber = regexp.MustCompile(`^\s*\d+\s*[.)．、]\s*`)

// splitSteps splits instruction text into steps (one line = one step) and
// strips leading manual numbering; the markdown ordered list re-numbers.
func splitSteps(s string) []string {
	var out []string
	for _, line := range splitLines(s) {
		out = append(out, stepNumber.ReplaceAllString(line, ""))
	}
	return out
}

// SortedUnknownProps returns unknown itemprops as "prop (count)" lines,
// most frequent first — the Phase 0 validation report.
func (r *Result) SortedUnknownProps() []string {
	type kv struct {
		k string
		n int
	}
	var props []kv
	for k, n := range r.UnknownProps {
		props = append(props, kv{k, n})
	}
	sort.Slice(props, func(i, j int) bool {
		if props[i].n != props[j].n {
			return props[i].n > props[j].n
		}
		return props[i].k < props[j].k
	})
	out := make([]string, len(props))
	for i, p := range props {
		out[i] = fmt.Sprintf("%s (%d)", p.k, p.n)
	}
	return out
}

// Package recipe loads konro recipe markdown files (the konro-import output
// format): YAML frontmatter + 材料 / 手順 / メモ sections. Files are the
// single source of truth — no index, parsed per request (a few hundred flat
// files; kensan proved this is fine).
package recipe

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type Meta struct {
	File     string   `json:"file"` // filename without .md — the recipe id
	Title    string   `json:"title"`
	Tags     []string `json:"tags"`
	Servings string   `json:"servings,omitempty"`
	PrepTime string   `json:"prepTime,omitempty"` // ISO8601
	CookTime string   `json:"cookTime,omitempty"`
	Source   string   `json:"source,omitempty"`
	Rating   int      `json:"rating,omitempty"`
	Images   []string `json:"images,omitempty"`
	RKID     string   `json:"rkId,omitempty"`
}

type Recipe struct {
	Meta
	Ingredients []string `json:"ingredients"`
	Steps       []string `json:"steps"`
	Notes       string   `json:"notes,omitempty"`
}

// List returns metadata for every .md file directly under dir, title-sorted.
// Unparseable files are skipped rather than failing the listing.
func List(dir string) ([]Meta, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var metas []Meta
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		r, err := Load(dir, strings.TrimSuffix(e.Name(), ".md"))
		if err != nil {
			continue
		}
		metas = append(metas, r.Meta)
	}
	sort.Slice(metas, func(i, j int) bool { return metas[i].Title < metas[j].Title })
	return metas, nil
}

// Load reads one recipe by id (filename without .md).
func Load(dir, name string) (*Recipe, error) {
	if strings.ContainsAny(name, "/\\") || strings.Contains(name, "..") {
		return nil, fmt.Errorf("invalid recipe name %q", name)
	}
	data, err := os.ReadFile(filepath.Join(dir, name+".md"))
	if err != nil {
		return nil, err
	}
	r := parse(string(data))
	r.File = name
	if r.Title == "" {
		r.Title = name
	}
	return r, nil
}

func parse(src string) *Recipe {
	r := &Recipe{}
	body := src
	if strings.HasPrefix(src, "---\n") {
		if end := strings.Index(src[4:], "\n---\n"); end >= 0 {
			parseFrontmatter(src[4:4+end], r)
			body = src[4+end+5:]
		}
	}
	parseBody(body, r)
	return r
}

func parseFrontmatter(fm string, r *Recipe) {
	for _, line := range strings.Split(fm, "\n") {
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		val = strings.TrimSpace(val)
		switch strings.TrimSpace(key) {
		case "title":
			r.Title = unquote(val)
		case "tags":
			r.Tags = parseList(val)
		case "servings":
			r.Servings = unquote(val)
		case "prep_time":
			r.PrepTime = val
		case "cook_time":
			r.CookTime = val
		case "source":
			r.Source = unquote(val)
		case "rating":
			r.Rating, _ = strconv.Atoi(val)
		case "images":
			r.Images = parseList(val)
		case "rk_id":
			r.RKID = val
		}
	}
}

var stepLine = regexp.MustCompile(`^\d+\.\s+`)

func parseBody(body string, r *Recipe) {
	section := ""
	var notes []string
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			section = strings.TrimSpace(strings.TrimPrefix(trimmed, "## "))
			continue
		}
		switch section {
		case "材料":
			if strings.HasPrefix(trimmed, "- ") {
				r.Ingredients = append(r.Ingredients, strings.TrimPrefix(trimmed, "- "))
			}
		case "手順":
			if stepLine.MatchString(trimmed) {
				r.Steps = append(r.Steps, stepLine.ReplaceAllString(trimmed, ""))
			}
		case "メモ":
			notes = append(notes, line)
		}
	}
	r.Notes = strings.TrimSpace(strings.Join(notes, "\n"))
}

func parseList(val string) []string {
	val = strings.TrimSpace(val)
	if !strings.HasPrefix(val, "[") || !strings.HasSuffix(val, "]") {
		return nil
	}
	var out []string
	for _, item := range strings.Split(val[1:len(val)-1], ",") {
		if item = unquote(strings.TrimSpace(item)); item != "" {
			out = append(out, item)
		}
	}
	return out
}

func unquote(s string) string {
	if len(s) >= 2 && strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"") {
		s = s[1 : len(s)-1]
		s = strings.ReplaceAll(s, "\\\"", "\"")
		s = strings.ReplaceAll(s, "\\\\", "\\")
	}
	return s
}

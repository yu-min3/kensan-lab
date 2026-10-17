// Package recipe loads konro recipe markdown files (the konro-import output
// format): YAML frontmatter + 材料 / 手順 / メモ sections. Files are the
// single source of truth — no index, parsed per request (a few hundred flat
// files; kensan proved this is fine).
package recipe

import (
	"encoding/json"
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
	Version     string   `json:"version"`
}

// List returns metadata for every .md file directly under dir, title-sorted.
// Unparseable files are skipped rather than failing the listing.
func List(dir string) ([]Meta, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	metas := []Meta{}
	if imported, err := os.ReadDir(filepath.Join(dir, "imported")); err == nil {
		for _, e := range imported {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
				r, err := Load(dir, "imported/"+strings.TrimSuffix(e.Name(), ".md"))
				if err == nil {
					metas = append(metas, r.Meta)
				}
			}
		}
	}
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
	if !validName(name) {
		return nil, fmt.Errorf("invalid recipe name %q", name)
	}
	data, err := readSource(dir, name)
	if err != nil {
		return nil, err
	}
	r := parse(string(data))
	r.File = name
	r.Version = version(data)
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
		if section == "メモ" {
			notes = append(notes, line)
			continue
		}
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
	var jsonList []string
	if json.Unmarshal([]byte(val), &jsonList) == nil {
		return jsonList
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
		if v, err := strconv.Unquote(s); err == nil {
			return v
		}
	}
	return s
}

func validName(name string) bool {
	if strings.HasPrefix(name, "imported/") {
		return importedID.MatchString(strings.TrimPrefix(name, "imported/"))
	}
	return name != "" && !strings.ContainsAny(name, "/\\") && !strings.Contains(name, "..")
}

var importedID = regexp.MustCompile(`^[0-9a-f]{32}$`)

// SaveImported writes an immutable URL/text recipe in its own namespace. ZIP
// imports only write root-level files and cannot collide with these IDs.
func SaveImported(dir, id string, r Recipe) error {
	if !importedID.MatchString(id) {
		return fmt.Errorf("invalid imported ID")
	}
	target := filepath.Join(dir, "imported")
	if err := os.MkdirAll(target, 0700); err != nil {
		return err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "---\ntitle: %s\nservings: %s\nsource: %s\ntags: []\n---\n\n## 材料\n", strconv.Quote(r.Title), strconv.Quote(r.Servings), strconv.Quote(r.Source))
	for _, v := range r.Ingredients {
		fmt.Fprintf(&b, "- %s\n", v)
	}
	b.WriteString("\n## 手順\n")
	for i, v := range r.Steps {
		fmt.Fprintf(&b, "%d. %s\n", i+1, v)
	}
	path := filepath.Join(target, id+".md")
	// A repeated confirm after a crash is safe; never replace a different recipe.
	if old, err := os.ReadFile(path); err == nil {
		if string(old) == b.String() {
			return nil
		}
		return fmt.Errorf("imported recipe already exists")
	}
	f, err := os.CreateTemp(target, ".recipe-")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(0600); err == nil {
		_, err = f.WriteString(b.String())
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	// Hard-link publication is atomic and refuses any existing destination.
	if err = os.Link(name, path); err != nil {
		return err
	}
	d, err := os.Open(target)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}

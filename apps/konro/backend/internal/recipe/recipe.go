// Package recipe loads konro recipe markdown files (the konro-import output
// format): YAML frontmatter + 材料 / 手順 / メモ sections. Files are the
// single source of truth — no index, parsed per request (a few hundred flat
// files; kensan proved this is fine).
package recipe

import (
	"fmt"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/provenance"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type Meta struct {
	File       string           `json:"file"` // filename without .md — the recipe id
	Title      string           `json:"title"`
	Tags       []string         `json:"tags"`
	Servings   string           `json:"servings,omitempty"`
	PrepTime   string           `json:"prepTime,omitempty"` // ISO8601
	CookTime   string           `json:"cookTime,omitempty"`
	Source     string           `json:"source,omitempty"`
	Rating     int              `json:"rating,omitempty"`
	Images     []string         `json:"images,omitempty"`
	RKID       string           `json:"rkId,omitempty"`
	SourceInfo *provenance.Info `json:"sourceInfo,omitempty"`
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
	if os.IsNotExist(err) {
		return []Meta{}, nil
	}
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
	d, err := DecodeDocument(data)
	if err != nil {
		return nil, err
	}
	r := &d.Recipe
	r.File = name
	r.Version = version(data)
	if r.Title == "" {
		r.Title = name
	}
	return r, nil
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

func validName(name string) bool {
	if strings.HasPrefix(name, "imported/") {
		return importedID.MatchString(strings.TrimPrefix(name, "imported/"))
	}
	return name != "" && !strings.ContainsAny(name, "/\\") && !strings.Contains(name, "..")
}

var importedID = regexp.MustCompile(`^[0-9a-f]{32}$`)

// SaveImported uses the same validation, codec and no-overwrite publication as manual creation.
func SaveImported(dir, id string, r Recipe) error {
	_, err := create(dir, id, r, false)
	return err
}

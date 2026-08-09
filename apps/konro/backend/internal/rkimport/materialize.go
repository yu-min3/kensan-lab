package rkimport

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
)

// WriteStats summarizes a Materialize run for the CLI report / API response.
type WriteStats struct {
	Recipes       int `json:"recipes"`
	Images        int `json:"images"`
	EmptySections int `json:"emptySections"`
}

// Materialize writes the parsed recipes as markdown files plus their images
// into outDir. Filenames derive from titles, so re-importing a newer export
// overwrites matching recipes in place (Recipe Keeper stays the master).
// Recipes with an empty ingredients or steps section are still written but
// counted and reported through res.Warnings.
func Materialize(res *Result, images map[string][]byte, outDir, date string) (WriteStats, error) {
	stats := WriteStats{Recipes: len(res.Recipes), Images: len(images)}
	if err := os.MkdirAll(filepath.Join(outDir, "images"), 0o755); err != nil {
		return stats, err
	}
	taken := map[string]bool{}
	for _, r := range res.Recipes {
		if len(r.Ingredients) == 0 || len(r.Steps) == 0 {
			stats.EmptySections++
			res.Warnings = append(res.Warnings, fmt.Sprintf("%s: ingredients=%d steps=%d", r.Title, len(r.Ingredients), len(r.Steps)))
		}
		name := Filename(r.Title, taken)
		md := Markdown(r, "images", date)
		if err := os.WriteFile(filepath.Join(outDir, name), []byte(md), 0o644); err != nil {
			return stats, err
		}
	}
	for ref, data := range images {
		if err := os.WriteFile(filepath.Join(outDir, "images", path.Base(ref)), data, 0o644); err != nil {
			return stats, err
		}
	}
	return stats, nil
}

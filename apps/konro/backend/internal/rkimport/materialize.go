package rkimport

import (
	"fmt"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/recipe"
	"os"
	"path"
	"path/filepath"
)

// WriteStats summarizes a Materialize run for the CLI report / API response.
type WriteStats struct {
	Recipes       int `json:"recipes"`
	Images        int `json:"images"`
	EmptySections int `json:"emptySections"`
	Skipped       int `json:"skipped"`
}

// Materialize writes the parsed recipes as markdown files plus their images
// into outDir. Filenames derive from titles, so re-importing a newer export
// updates matching untouched recipes; Konro-edited files stay authoritative.
// Recipes with an empty ingredients or steps section are still written but
// counted and reported through res.Warnings.
func Materialize(res *Result, images map[string][]byte, outDir, date string) (WriteStats, error) {
	stats := WriteStats{Images: len(images)}
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
		md, err := Markdown(r, "images", date)
		if err != nil {
			return stats, err
		}
		written, err := recipe.WriteLegacyImport(outDir, name, []byte(md))
		if err != nil {
			return stats, err
		}
		if written {
			stats.Recipes++
		} else {
			stats.Skipped++
			res.Warnings = append(res.Warnings, fmt.Sprintf("%s: Konroで編集済みのため保持しました", r.Title))
		}
	}
	for ref, data := range images {
		if err := os.WriteFile(filepath.Join(outDir, "images", path.Base(ref)), data, 0o644); err != nil {
			return stats, err
		}
	}
	return stats, nil
}

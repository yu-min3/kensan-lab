package rkimport

import (
	"fmt"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/recipe"
	"path"
	"strconv"
	"strings"
)

// Markdown renders a recipe as a konro recipe file: YAML frontmatter
// (type: recipe) + 材料 / 手順 / メモ sections. imageDir is where the caller
// will materialize image files ("images"); date fills created/updated.
func Markdown(r Recipe, imageDir, date string) (string, error) {
	tags := []string{}
	seen := map[string]bool{}
	for _, t := range append(append(append([]string{}, r.Collections...), r.Categories...), r.Courses...) {
		if !seen[t] {
			seen[t] = true
			tags = append(tags, t)
		}
	}
	images := []string{}
	for _, img := range r.Images {
		images = append(images, path.Join(imageDir, path.Base(img)))
	}
	prep, cook := r.PrepTime, r.CookTime
	if prep == "PT0S" {
		prep = ""
	}
	if cook == "PT0S" {
		cook = ""
	}
	rating, _ := strconv.Atoi(r.Rating)
	data, err := recipe.Encode(recipe.Recipe{Meta: recipe.Meta{Title: r.Title, RKID: r.RKID, Tags: tags, Servings: r.Yield, PrepTime: prep, CookTime: cook, Source: r.Source, Rating: rating, Images: images}, Ingredients: r.Ingredients, Steps: r.Steps, Notes: r.Notes}, date)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Filename derives a markdown filename from the title (Japanese kept as-is,
// path-hostile characters replaced), deduplicating via the taken set.
func Filename(title string, taken map[string]bool) string {
	name := strings.TrimSpace(title)
	for _, c := range []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|", "\n", "\t"} {
		name = strings.ReplaceAll(name, c, " ")
	}
	name = strings.Join(strings.Fields(name), " ")
	if name == "" {
		name = "untitled"
	}
	candidate := name + ".md"
	for i := 2; taken[candidate]; i++ {
		candidate = fmt.Sprintf("%s-%d.md", name, i)
	}
	taken[candidate] = true
	return candidate
}

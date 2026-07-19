package rkimport

import (
	"fmt"
	"path"
	"strings"
)

// Markdown renders a recipe as a konro recipe file: YAML frontmatter
// (type: recipe) + 材料 / 手順 / メモ sections. imageDir is where the caller
// will materialize image files ("images"); date fills created/updated.
func Markdown(r Recipe, imageDir, date string) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("type: recipe\n")
	fmt.Fprintf(&b, "title: %s\n", yamlString(r.Title))

	tags := append(append([]string{}, r.Categories...), r.Courses...)
	if len(tags) > 0 {
		quoted := make([]string, len(tags))
		for i, t := range tags {
			quoted[i] = yamlString(t)
		}
		fmt.Fprintf(&b, "tags: [%s]\n", strings.Join(quoted, ", "))
	}
	if r.Yield != "" {
		fmt.Fprintf(&b, "servings: %s\n", yamlString(r.Yield))
	}
	if r.PrepTime != "" {
		fmt.Fprintf(&b, "prep_time: %s\n", r.PrepTime)
	}
	if r.CookTime != "" {
		fmt.Fprintf(&b, "cook_time: %s\n", r.CookTime)
	}
	if r.Source != "" {
		fmt.Fprintf(&b, "source: %s\n", yamlString(r.Source))
	}
	if r.Rating != "" {
		fmt.Fprintf(&b, "rating: %s\n", r.Rating)
	}
	if len(r.Images) > 0 {
		refs := make([]string, len(r.Images))
		for i, img := range r.Images {
			refs[i] = yamlString(path.Join(imageDir, path.Base(img)))
		}
		fmt.Fprintf(&b, "images: [%s]\n", strings.Join(refs, ", "))
	}
	fmt.Fprintf(&b, "created: %s\nupdated: %s\n", date, date)
	b.WriteString("---\n\n## 材料\n\n")
	for _, ing := range r.Ingredients {
		fmt.Fprintf(&b, "- %s\n", ing)
	}
	b.WriteString("\n## 手順\n\n")
	for i, step := range r.Steps {
		fmt.Fprintf(&b, "%d. %s\n", i+1, step)
	}
	if r.Notes != "" {
		b.WriteString("\n## メモ\n\n")
		b.WriteString(r.Notes)
		b.WriteString("\n")
	}
	return b.String()
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

func yamlString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", " ")
	return "\"" + s + "\""
}

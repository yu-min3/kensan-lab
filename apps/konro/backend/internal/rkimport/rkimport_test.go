package rkimport

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func parseFixture(t *testing.T) *Result {
	t.Helper()
	f, err := os.Open("testdata/recipes.html")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	res, err := ParseHTML(f)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestParseHTML(t *testing.T) {
	res := parseFixture(t)
	if len(res.Recipes) != 2 {
		t.Fatalf("recipes = %d, want 2", len(res.Recipes))
	}

	r := res.Recipes[0]
	if r.Title != "鶏むね肉の南蛮漬け" {
		t.Errorf("title = %q", r.Title)
	}
	if len(r.Ingredients) != 4 || r.Ingredients[0] != "鶏むね肉 2枚" || r.Ingredients[2] != "■南蛮酢" {
		t.Errorf("ingredients = %#v", r.Ingredients)
	}
	// manual numbering must be stripped (ordered list re-numbers)
	if len(r.Steps) != 3 || r.Steps[0] != "鶏むね肉をそぎ切りにする。" {
		t.Errorf("steps = %#v", r.Steps)
	}
	if r.PrepTime != "PT15M" || r.CookTime != "PT20M" {
		t.Errorf("times = %q / %q", r.PrepTime, r.CookTime)
	}
	if r.Yield != "4人分" || r.Rating != "4" || r.Source != "https://example.com/nanban" {
		t.Errorf("yield/rating/source = %q / %q / %q", r.Yield, r.Rating, r.Source)
	}
	if len(r.Categories) != 1 || r.Categories[0] != "作り置き" || len(r.Courses) != 1 {
		t.Errorf("categories/courses = %#v / %#v", r.Categories, r.Courses)
	}
	if len(r.Images) != 1 || r.Images[0] != "images/nanban.jpg" {
		t.Errorf("images = %#v", r.Images)
	}
	if !strings.Contains(r.Notes, "冷蔵4日") {
		t.Errorf("notes = %q", r.Notes)
	}

	// second recipe exercises the singular/instructions aliases + cookTime
	r2 := res.Recipes[1]
	if len(r2.Ingredients) != 2 || len(r2.Steps) != 2 || r2.CookTime != "PT45M" {
		t.Errorf("recipe2 = %#v", r2)
	}

	// known-but-ignored props must not appear; the planted unknown must
	if _, ok := res.UnknownProps["recipeIsFavourite"]; ok {
		t.Error("recipeIsFavourite should be recognized")
	}
	if _, ok := res.UnknownProps["recipeNutTotalFat"]; ok {
		t.Error("recipeNut* should be recognized")
	}
	if res.UnknownProps["recipeFutureUnknownField"] != 1 {
		t.Errorf("unknown props = %#v", res.UnknownProps)
	}
}

func TestParseZip(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "export.zip")
	zf, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(zf)
	htmlSrc, err := os.ReadFile("testdata/recipes.html")
	if err != nil {
		t.Fatal(err)
	}
	// exports nest content under a folder — resolution must be relative to recipes.html
	w, _ := zw.Create("recipekeeperhtml/recipes.html")
	w.Write(htmlSrc)
	w, _ = zw.Create("recipekeeperhtml/images/nanban.jpg")
	w.Write([]byte("fake-jpeg"))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	zf.Close()

	res, images, err := ParseZip(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Recipes) != 2 {
		t.Fatalf("recipes = %d, want 2", len(res.Recipes))
	}
	if string(images["images/nanban.jpg"]) != "fake-jpeg" {
		t.Errorf("images = %v keys", len(images))
	}
	if len(res.Warnings) != 0 {
		t.Errorf("warnings = %#v", res.Warnings)
	}
}

func TestMarkdown(t *testing.T) {
	res := parseFixture(t)
	md := Markdown(res.Recipes[0], "images", "2026-07-19")
	for _, want := range []string{
		"type: recipe\n",
		`title: "鶏むね肉の南蛮漬け"`,
		`tags: ["作り置き", "主菜"]`,
		`servings: "4人分"`,
		"prep_time: PT15M",
		"cook_time: PT20M",
		"rating: 4",
		`images: ["images/nanban.jpg"]`,
		"## 材料\n\n- 鶏むね肉 2枚",
		"## 手順\n\n1. 鶏むね肉をそぎ切りにする。\n2. ",
		"## メモ\n\n冷蔵4日",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q\n---\n%s", want, md)
		}
	}
}

func TestFilename(t *testing.T) {
	taken := map[string]bool{}
	if got := Filename("鶏むね肉の南蛮漬け", taken); got != "鶏むね肉の南蛮漬け.md" {
		t.Errorf("got %q", got)
	}
	if got := Filename("鶏むね肉の南蛮漬け", taken); got != "鶏むね肉の南蛮漬け-2.md" {
		t.Errorf("dedupe got %q", got)
	}
	if got := Filename("a/b: c?", taken); got != "a b c.md" {
		t.Errorf("sanitize got %q", got)
	}
	if got := Filename("  ", taken); got != "untitled.md" {
		t.Errorf("empty got %q", got)
	}
}

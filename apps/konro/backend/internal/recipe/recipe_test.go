package recipe

import (
	"os"
	"path/filepath"
	"testing"
)

const sample = `---
type: recipe
title: "【作り置き】ゴーヤの甘酢漬け"
rk_id: 63593418-1ca8-469a-bcb5-c2fdd22f62d8
tags: ["野菜", "副菜", "和食"]
servings: "2～3人分"
cook_time: PT10M
source: "https://macaro-ni.jp/91055"
rating: 3
images: ["images/63593418_0.jpg"]
created: 2026-07-19
updated: 2026-07-19
---

## 材料

- ゴーヤ 1本
- 砂糖 80g

## 手順

1. ゴーヤは薄切りにします。
2. 冷蔵庫でひと晩漬け込みます。

## メモ

冷蔵3〜4日。
`

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ゴーヤの甘酢漬け.md"), []byte(sample), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := Load(dir, "ゴーヤの甘酢漬け")
	if err != nil {
		t.Fatal(err)
	}
	if r.File != "ゴーヤの甘酢漬け" || r.Title != "【作り置き】ゴーヤの甘酢漬け" {
		t.Errorf("meta = %#v", r.Meta)
	}
	if len(r.Tags) != 3 || r.Tags[0] != "野菜" {
		t.Errorf("tags = %#v", r.Tags)
	}
	if r.Servings != "2～3人分" || r.CookTime != "PT10M" || r.Rating != 3 {
		t.Errorf("meta = %#v", r.Meta)
	}
	if len(r.Ingredients) != 2 || r.Ingredients[0] != "ゴーヤ 1本" {
		t.Errorf("ingredients = %#v", r.Ingredients)
	}
	if len(r.Steps) != 2 || r.Steps[1] != "冷蔵庫でひと晩漬け込みます。" {
		t.Errorf("steps = %#v", r.Steps)
	}
	if r.Notes != "冷蔵3〜4日。" {
		t.Errorf("notes = %q", r.Notes)
	}
}

func TestLoadRejectsTraversal(t *testing.T) {
	if _, err := Load(t.TempDir(), "../etc/passwd"); err == nil {
		t.Error("path traversal not rejected")
	}
}

func TestList(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"b.md", "a.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(sample), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	metas, err := List(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(metas) != 2 {
		t.Fatalf("metas = %d", len(metas))
	}
}

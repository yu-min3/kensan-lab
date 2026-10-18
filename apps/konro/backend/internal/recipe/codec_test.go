package recipe

import (
	"reflect"
	"strings"
	"testing"
)

func TestCodecPreservesUnknownYAMLAndBody(t *testing.T) {
	src := "\ufeff---\r\n\"title\": '魚 & 野菜'\r\ntags:\r\n- 和食\r\n- '週末,作り置き'\r\nservings: >\r\n  2人分\r\ncustom: &info\r\n  title: keep-this\r\n  memo: |\r\n    line one\r\n    line two\r\ncopy: *info\r\n# keep this comment\r\nimages: [images/fish.jpg]\r\nrk_id: origin-id\r\ncreated: 2026-07-01\r\n---\r\n\r\n# 原文の補足\r\n\r\n## 材料\r\n- 魚 2切れ\r\n\r\n## 補足\r\nこの節を保持する\r\n\r\n## 手順\r\n1. 煮る\r\n\r\n## メモ\r\n## 手順\r\nここもメモ\r\n"
	d, err := DecodeDocument([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if d.Recipe.Title != "魚 & 野菜" || !reflect.DeepEqual(d.Recipe.Tags, []string{"和食", "週末,作り置き"}) {
		t.Fatalf("decode %+v", d.Recipe)
	}
	e := editable(&d.Recipe)
	e.Title = "変更した魚"
	b, err := d.render(e, "2026-10-17", true)
	if err != nil {
		t.Fatal(err)
	}
	for _, keep := range []string{"keep-this", "line one", "line two", "*info", "# keep this comment", "## 補足\nこの節を保持する", "# 原文の補足"} {
		if !strings.Contains(string(b), keep) {
			t.Fatalf("lost %q\n%s", keep, b)
		}
	}
	got, err := DecodeDocument(b)
	if err != nil {
		t.Fatal(err)
	}
	if got.Recipe.Title != e.Title || got.Recipe.RKID != "origin-id" || got.Recipe.Images[0] != "images/fish.jpg" || got.Recipe.Notes != "## 手順\nここもメモ" || !got.Edited() {
		t.Fatalf("roundtrip %+v", got.Recipe)
	}
}
func TestCodecCanonicalRoundTrip(t *testing.T) {
	r := Recipe{Meta: Meta{Title: "引用 \"魚\" & <野菜>", Tags: []string{"和食", "one,two"}, Servings: "2人分", CookTime: "PT1M30S", Images: []string{"images/a.jpg"}, RKID: "a-123", Rating: 3}, Ingredients: []string{"魚 2切れ", "塩 少々"}, Steps: []string{"煮る", "盛る"}, Notes: "メモ\n## 手順\n見出しもメモ"}
	b, err := Encode(r, "2026-10-17")
	if err != nil {
		t.Fatal(err)
	}
	d, err := DecodeDocument(b)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(normalizeEdit(editable(&r)), normalizeEdit(editable(&d.Recipe))) || !reflect.DeepEqual(r.Images, d.Recipe.Images) || r.RKID != d.Recipe.RKID {
		t.Fatalf("roundtrip differs: %+v", d.Recipe)
	}
}
func TestCodecRejectsAmbiguousYAML(t *testing.T) {
	for _, src := range []string{"---\ntitle: a\ntitle: b\n---\n", "---\n- a\n---\n", "---\ntitle: [\n---\n", "---\ntitle: a\n"} {
		if _, err := DecodeDocument([]byte(src)); err == nil {
			t.Fatalf("accepted %q", src)
		}
	}
}

func TestEditRetainsAliasToRemovedListItem(t *testing.T) {
	d, err := DecodeDocument([]byte("---\ntitle: 魚\ntags: [&tag 和食]\ncustom: *tag\n---\n\n## 材料\n- 魚\n\n## 手順\n1. 煮る\n"))
	if err != nil {
		t.Fatal(err)
	}
	e := editable(&d.Recipe)
	e.Tags = []string{"洋食"}
	b, err := d.render(e, "", true)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeDocument(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Recipe.Tags) != 1 || got.Recipe.Tags[0] != "洋食" {
		t.Fatal(got.Recipe.Tags)
	}
	var meta map[string]any
	if err := got.meta.Decode(&meta); err != nil {
		t.Fatal(err)
	}
	if meta["custom"] != "和食" {
		t.Fatalf("lost referenced value: %s", b)
	}
}
func TestEditMissingSectionsBeforeMemo(t *testing.T) {
	for _, body := range []string{"## メモ\n残すメモ\n", "## 材料\n- 魚\n\n## メモ\n残すメモ\n"} {
		d, err := DecodeDocument([]byte("---\ntitle: 魚\n---\n\n" + body))
		if err != nil {
			t.Fatal(err)
		}
		e := editable(&d.Recipe)
		e.Ingredients = []string{"魚"}
		e.Steps = []string{"煮る"}
		b, err := d.render(e, "", true)
		if err != nil {
			t.Fatal(err)
		}
		got, err := DecodeDocument(b)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got.Recipe.Steps, e.Steps) || !reflect.DeepEqual(got.Recipe.Ingredients, e.Ingredients) || got.Recipe.Notes != "残すメモ" {
			t.Fatalf("sections swallowed by memo: %s", b)
		}
	}
}

func TestChangingSourceDetachesSourceInfoAliases(t *testing.T) {
	raw := []byte("---\ntitle: 魚\nsource: https://example.com/old\nsource_info: &credit\n  creator:\n    name: 本人\ncredit_copy: *credit\n---\n\n## 材料\n- 魚\n\n## 手順\n1. 煮る\n")
	d, err := DecodeDocument(raw)
	if err != nil {
		t.Fatal(err)
	}
	e := editable(&d.Recipe)
	e.Source = "https://example.com/new"
	encoded, err := d.render(e, "2026-10-18", true)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := DecodeDocument(encoded)
	if err != nil || loaded.Recipe.SourceInfo != nil || !strings.Contains(string(encoded), "credit_copy:") {
		t.Fatal(string(encoded), err)
	}
}

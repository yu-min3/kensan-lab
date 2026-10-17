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

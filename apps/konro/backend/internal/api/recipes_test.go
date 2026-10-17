package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/recipe"
)

func TestRecipeEditorPreservesMetadataAndRejectsConflict(t *testing.T) {
	dir := t.TempDir()
	name := "魚"
	path := filepath.Join(dir, name+".md")
	original := "---\ntitle: 魚\nrk_id: source-123\nimages: [images/fish.jpg]\ncreated: 2026-07-01\ncustom_key: keep-me\ntags: []\n---\n\n## 材料\n- 魚 2切れ\n\n## 手順\n1. 煮る\n\n## メモ\nold note\n"
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	handler := New(dir, "")
	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest("GET", "/api/v1/recipes/魚", nil))
	var initial recipe.Recipe
	_ = json.Unmarshal(get.Body.Bytes(), &initial)
	if initial.Version == "" {
		t.Fatal("missing version")
	}
	payload := map[string]any{"version": initial.Version, "title": "魚 & 野菜", "servings": "2人分", "tags": []string{"和食", "週末,作り置き"}, "prepTime": "PT5M", "cookTime": "PT10M", "source": "本のレシピ", "rating": 5, "ingredients": []string{"魚 2切れ", "野菜"}, "steps": []string{"10分煮る"}, "notes": "メモ\n## 手順\nこの行もメモ"}
	encoded, _ := json.Marshal(payload)
	put := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("PUT", "/api/v1/recipes/魚", bytes.NewReader(encoded)))
		return w
	}
	if w := put(); w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	got, err := recipe.Load(dir, name)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "魚 & 野菜" || got.RKID != "source-123" || len(got.Images) != 1 || got.Images[0] != "images/fish.jpg" || got.Tags[1] != "週末,作り置き" || got.Notes != payload["notes"] || got.CookTime != "PT10M" {
		t.Fatalf("lost data %+v", got)
	}
	saved, _ := os.ReadFile(path)
	for _, keep := range []string{"created: 2026-07-01", "custom_key: keep-me", "konro_edited: true"} {
		if !strings.Contains(string(saved), keep) {
			t.Fatalf("lost %s", keep)
		}
	}
	if w := put(); w.Code != 409 {
		t.Fatalf("stale write accepted: %d", w.Code)
	}
	var statuses [2]int
	var wg sync.WaitGroup
	for i := range statuses {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			p := map[string]any{"version": got.Version, "title": fmt.Sprintf("別の編集%d", i)}
			body, _ := json.Marshal(p)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest("PUT", "/api/v1/recipes/魚", bytes.NewReader(body)))
			statuses[i] = w.Code
		}(i)
	}
	wg.Wait()
	if statuses[0]+statuses[1] != 609 {
		t.Fatalf("expected one winner: %v", statuses)
	}
}
func TestCreateRecipeIsIdempotentAndRejectsUnsafePaths(t *testing.T) {
	dir := t.TempDir()
	handler := New(dir, "")
	create := func(key string, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/api/v1/recipes", strings.NewReader(body))
		r.Header.Set("Idempotency-Key", key)
		handler.ServeHTTP(w, r)
		return w
	}
	id := strings.Repeat("a", 32)
	input := `{"title":"手入力","notes":"  メモ  ","tags":[],"ingredients":[],"steps":[]}`
	first := create(id, input)
	if first.Code != http.StatusCreated {
		t.Fatalf("%d %s", first.Code, first.Body.String())
	}
	if second := create(id, input); second.Code != http.StatusCreated {
		t.Fatalf("retry %d %s", second.Code, second.Body.String())
	}
	if w := create(id, `{"title":"上書き"}`); w.Code != 409 {
		t.Fatal("create overwrote a recipe")
	}
	if w := create("../outside", input); w.Code != 400 {
		t.Fatalf("unsafe id %d", w.Code)
	}
	if w := create("", `{"title":"x","images":["secret"]}`); w.Code != 400 {
		t.Fatal("unknown field accepted")
	}
	if w := create("", `{"title":"x"} {"title":"y"}`); w.Code != 400 {
		t.Fatal("multiple JSON accepted")
	}
	outside := filepath.Join(t.TempDir(), "outside.md")
	_ = os.WriteFile(outside, []byte("private"), 0600)
	_ = os.Symlink(outside, filepath.Join(dir, "link.md"))
	if _, err := recipe.Load(dir, "link"); err == nil {
		t.Fatal("symlink read")
	}
	if _, err := recipe.Update(dir, "link", "x", recipe.Editable{Title: "bad"}); err == nil {
		t.Fatal("symlink write")
	}
	raw, _ := os.ReadFile(outside)
	if string(raw) != "private" {
		t.Fatal("changed symlink target")
	}
}
func TestZIPReimportKeepsKonroEdits(t *testing.T) {
	dir := t.TempDir()
	handler := New(dir, "")
	post := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/import", bytes.NewReader(exportZip(t))))
		return w
	}
	if w := post(); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	name := "鶏むね肉の南蛮漬け"
	old, err := recipe.Load(dir, name)
	if err != nil {
		t.Fatal(err)
	}
	_, err = recipe.Update(dir, name, old.Version, recipe.Editable{Title: "Konroで書き換えたタイトル", Ingredients: []string{"魚"}, Steps: []string{"煮る"}})
	if err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(dir, name+".md"))
	w := post()
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var report importReport
	_ = json.Unmarshal(w.Body.Bytes(), &report)
	after, _ := os.ReadFile(filepath.Join(dir, name+".md"))
	if !bytes.Equal(before, after) || report.Skipped != 1 || report.Recipes != 1 {
		t.Fatalf("edited recipe overwritten: %+v", report)
	}
}

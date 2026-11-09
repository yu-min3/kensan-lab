package main

import (
	"bytes"
	"encoding/json"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/importer"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func call(t *testing.T, op string, payload any) map[string]json.RawMessage {
	t.Helper()
	raw, _ := json.Marshal(map[string]any{"protocol_version": "1", "request_id": "case-1", "operation": op, "domain_schema_version": domainVersion, "payload": payload})
	var out bytes.Buffer
	if e := run(bytes.NewReader(raw), &out); e != nil {
		t.Fatal(e)
	}
	var r map[string]json.RawMessage
	if e := json.Unmarshal(out.Bytes(), &r); e != nil {
		t.Fatal(e)
	}
	return r
}
func TestCapturedHTMLNeverFetchesSource(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1) }))
	defer server.Close()
	raw := `<html><body><h1>玉ねぎスープ</h1><p>玉ねぎ1/2個</p><script type="application/ld+json">{"@type":"Recipe","name":"玉ねぎスープ","recipeIngredient":["玉ねぎ1/2個"],"recipeInstructions":["煮る"]}</script></body></html>`
	r := call(t, "prepare", prepareInput{server.URL, "text/html", raw})
	if r["error"] != nil {
		t.Fatal(string(r["error"]))
	}
	var p prepared
	json.Unmarshal(r["result"], &p)
	if p.Content.Title != "玉ねぎスープ" || p.Content.Ingredients[0] != "玉ねぎ1/2個" || !strings.Contains(p.Text, "玉ねぎ1/2個") || hits.Load() != 0 {
		t.Fatalf("unexpected: %+v", p)
	}
}
func TestPreparedTextBoundsAreVisible(t *testing.T) {
	r := call(t, "prepare", prepareInput{"https://example.com/recipe", "text/html", "<body>" + strings.Repeat("x", importer.MaxText+10) + "</body>"})
	var p prepared
	json.Unmarshal(r["result"], &p)
	if !p.TextTruncated || p.TextBytes != importer.MaxText {
		t.Fatalf("truncation not disclosed: %+v", p)
	}
	r = call(t, "prepare", prepareInput{"", "text/plain", strings.Repeat("x", importer.MaxText+1)})
	if r["error"] == nil {
		t.Fatal("oversized text accepted")
	}
}
func TestGradesUseConfirmedExpectations(t *testing.T) {
	expected := importer.Content{Title: "スープ", Ingredients: []string{"玉ねぎ1/2個"}, Steps: []string{"煮る"}}
	output, _ := json.Marshal(expected)
	for _, tc := range []struct {
		name, status, annotation string
		expected                 *importer.Content
		output                   json.RawMessage
	}{
		{"confirmed", "passed", "confirmed", &expected, output},
		{"unreviewed", "needs_review", "unreviewed", &expected, output},
		{"missing expectation", "needs_review", "confirmed", nil, output},
		{"invented serving", "failed", "confirmed", &expected, json.RawMessage(`{"title":"スープ","servings":"2人分","ingredients":["玉ねぎ1/2個"],"steps":["煮る"]}`)},
		{"missing ingredients", "failed", "confirmed", &expected, json.RawMessage(`{"title":"スープ","servings":"","ingredients":[],"steps":["煮る"]}`)},
		{"unknown field", "failed", "confirmed", &expected, json.RawMessage(`{"title":"スープ","servings":"","ingredients":["玉ねぎ1/2個"],"steps":["煮る"],"shell":"id"}`)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := call(t, "grade", gradeInput{Output: tc.output, Expected: tc.expected, AnnotationStatus: tc.annotation})
			var g graded
			json.Unmarshal(r["result"], &g)
			if g.Status != tc.status {
				t.Fatalf("want %s got %+v (%s)", tc.status, g, r["error"])
			}
		})
	}
}
func TestProtocolRejectsInjectionAndTrailingInput(t *testing.T) {
	for _, raw := range []string{`{"protocol_version":"1","request_id":"ok","operation":"prepare","domain_schema_version":"konro.recipe-import.v1","payload":{},"shell":"id"}`, `{} {}`, `{"request_id":"../../oops"}`} {
		if run(strings.NewReader(raw), &bytes.Buffer{}) == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	r := call(t, "prepare", map[string]any{"raw": "x", "mime_type": "text/plain", "command": "id"})
	if r["error"] == nil {
		t.Fatal("unknown payload accepted")
	}
}

func TestConstraintGradingDetectsCorruptionAndPreservesUnknown(t *testing.T) {
	expected := importer.Content{Title: "スープ", Ingredients: []string{"玉ねぎ1/2個", "じゃがいも2個"}, Steps: []string{"切る", "煮る"}}
	output := func(c importer.Content) json.RawMessage { b, _ := json.Marshal(c); return b }
	for _, tc := range []struct {
		name, status, annotation string
		recipe                   importer.Content
		constraints              map[string]any
	}{
		{"synthetic oracle", "passed", "synthetic", expected, map[string]any{"expected": expected}},
		{"reviewed oracle", "passed", "reviewed", expected, map[string]any{"expected": expected}},
		{"unreviewed oracle", "needs_review", "unreviewed", expected, map[string]any{"expected": expected}},
		// Both outputs contain the same global numeric tokens, attached to different ingredients.
		{"swapped quantities", "failed", "reviewed", importer.Content{Title: "スープ", Ingredients: []string{"玉ねぎ2個", "じゃがいも1/2個"}, Steps: expected.Steps}, map[string]any{"expected": expected}},
		{"invented ingredient", "failed", "synthetic", importer.Content{Title: "スープ", Ingredients: []string{"玉ねぎ1/2個", "じゃがいも2個", "塩5g"}, Steps: expected.Steps}, map[string]any{"expected": expected}},
		{"missing step", "failed", "synthetic", importer.Content{Title: "スープ", Ingredients: expected.Ingredients, Steps: []string{"切る"}}, map[string]any{"expected": expected}},
		{"invented servings", "failed", "reviewed", importer.Content{Title: "スープ", Servings: "2人分", Ingredients: expected.Ingredients, Steps: expected.Steps}, map[string]any{"preserve_unknown_servings": true}},
		{"missing required line", "failed", "synthetic", expected, map[string]any{"required_ingredients": []string{"塩5g"}}},
		{"partial meaning unknown", "needs_review", "reviewed", expected, map[string]any{"required_ingredients": []string{"玉ねぎ1/2個"}}},
		{"unknown constraint", "needs_review", "reviewed", expected, map[string]any{"semantic_safety": true, "expected": expected}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			constraints, _ := json.Marshal(tc.constraints)
			response := call(t, "grade", gradeInput{Output: output(tc.recipe), AnnotationStatus: tc.annotation, ExpectedConstraints: constraints})
			if response["error"] != nil {
				t.Fatal(string(response["error"]))
			}
			var g graded
			json.Unmarshal(response["result"], &g)
			if g.Status != tc.status || g.AnnotationStatus != tc.annotation {
				t.Fatalf("expected %s/%s got %+v", tc.status, tc.annotation, g)
			}
		})
	}
}

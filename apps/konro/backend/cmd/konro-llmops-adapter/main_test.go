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
			r := call(t, "grade", gradeInput{tc.output, tc.expected, tc.annotation})
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

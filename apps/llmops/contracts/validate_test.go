package contracts

import (
	"encoding/json"
	"os"
	"testing"
)

func TestFixtures(t *testing.T) {
	b, e := os.ReadFile("fixtures/manifest.json")
	if e != nil {
		t.Fatal(e)
	}
	var cases []struct {
		File, Schema string
		Valid        bool
	}
	if e = json.Unmarshal(b, &cases); e != nil {
		t.Fatal(e)
	}
	for _, tc := range cases {
		t.Run(tc.File, func(t *testing.T) {
			b, e := os.ReadFile("fixtures/" + tc.File)
			if e != nil {
				t.Fatal(e)
			}
			var v any
			if e = json.Unmarshal(b, &v); e != nil {
				t.Fatal(e)
			}
			e = Validate(tc.Schema, v)
			if (e == nil) != tc.Valid {
				t.Fatalf("valid=%v: %v", tc.Valid, e)
			}
		})
	}
}
func TestUnknown(t *testing.T) {
	if Validate("../../arbitrary", nil) == nil {
		t.Fatal("unknown accepted")
	}
}
func TestRequiredAndTimestamp(t *testing.T) {
	b, e := os.ReadFile("fixtures/capture.valid.json")
	if e != nil {
		t.Fatal(e)
	}
	var v map[string]any
	if e = json.Unmarshal(b, &v); e != nil {
		t.Fatal(e)
	}
	v["captured_at"] = "yesterday"
	if Validate("capture", v) == nil {
		t.Fatal("invalid date accepted")
	}
	v["captured_at"] = "2026-01-01T00:00:00Z"
	delete(v, "object")
	if Validate("capture", v) == nil {
		t.Fatal("missing object accepted")
	}
}
func TestOpenAPIReferencesAndSecurity(t *testing.T) {
	b, e := os.ReadFile("openapi.json")
	if e != nil {
		t.Fatal(e)
	}
	var d map[string]any
	if e = json.Unmarshal(b, &d); e != nil {
		t.Fatal(e)
	}
	if len(d["paths"].(map[string]any)) != 7 {
		t.Fatal("expected 7 paths")
	}
	var walk func(any)
	walk = func(v any) {
		switch v := v.(type) {
		case map[string]any:
			if r, ok := v["$ref"].(string); ok {
				if len(r) < 2 || r[:2] != "./" {
					t.Fatalf("bad ref %s", r)
				}
				if _, e := os.Stat(r); e != nil {
					t.Fatal(e)
				}
			}
			if _, ok := v["operationId"]; ok {
				if _, ok := v["security"]; !ok {
					t.Fatal("missing security")
				}
			}
			for _, child := range v {
				walk(child)
			}
		case []any:
			for _, child := range v {
				walk(child)
			}
		}
	}
	walk(d)
}

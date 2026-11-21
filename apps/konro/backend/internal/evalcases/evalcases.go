// Package evalcases holds Konro's versioned evaluation cases. The file in this
// package is the source of truth; Langfuse datasets are synchronized from it.
package evalcases

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/telemetry"
)

//go:embed cases.json
var raw []byte

type Expect struct {
	TitleContains         string   `json:"title_contains,omitempty"`
	Servings              *string  `json:"servings,omitempty"`
	IngredientsContain    []string `json:"ingredients_contain,omitempty"`
	StepsContain          []string `json:"steps_contain,omitempty"`
	Forbid                []string `json:"forbid,omitempty"`
	StepsMin              *int     `json:"steps_min,omitempty"`
	StepsMax              *int     `json:"steps_max,omitempty"`
	AssessmentContainsAny []string `json:"assessment_contains_any,omitempty"`
}

type Case struct {
	ID     string   `json:"id"`
	Split  string   `json:"split"`
	Mode   string   `json:"mode"`
	Tags   []string `json:"tags"`
	Text   string   `json:"text"`
	Expect Expect   `json:"expect"`
}

type Set struct {
	Version int    `json:"version"`
	Note    string `json:"note"`
	Cases   []Case `json:"cases"`
}

// Load parses and validates the embedded case set.
func Load() (Set, error) { return Parse(raw) }

// Hash identifies the exact case file content.
func Hash() string { return telemetry.Version(string(raw)) }

func Parse(b []byte) (Set, error) {
	var s Set
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(&s); err != nil || d.Decode(new(any)) != io.EOF {
		return Set{}, fmt.Errorf("invalid case file: %v", err)
	}
	if s.Version != 1 || len(s.Cases) == 0 {
		return Set{}, errors.New("unsupported case file version")
	}
	seen := map[string]bool{}
	for _, c := range s.Cases {
		if c.ID == "" || seen[c.ID] {
			return Set{}, fmt.Errorf("duplicate or empty case id %q", c.ID)
		}
		seen[c.ID] = true
		if c.Split != "dev" && c.Split != "holdout" {
			return Set{}, fmt.Errorf("%s: split must be dev or holdout", c.ID)
		}
		if c.Mode != "steps" && c.Mode != "faithful" {
			return Set{}, fmt.Errorf("%s: unknown mode", c.ID)
		}
		e := c.Expect
		if c.Text == "" || e.TitleContains == "" {
			return Set{}, fmt.Errorf("%s: text and title expectation required", c.ID)
		}
		if e.StepsMin != nil && e.StepsMax != nil && *e.StepsMin > *e.StepsMax {
			return Set{}, fmt.Errorf("%s: steps_min > steps_max", c.ID)
		}
		for _, list := range [][]string{e.IngredientsContain, e.StepsContain, e.Forbid, e.AssessmentContainsAny} {
			for _, v := range list {
				if v == "" {
					return Set{}, fmt.Errorf("%s: empty expectation", c.ID)
				}
			}
		}
	}
	return s, nil
}

// Select returns cases in the given split ("" for all), preserving file order.
func (s Set) Select(split string) []Case {
	var out []Case
	for _, c := range s.Cases {
		if split == "" || c.Split == split {
			out = append(out, c)
		}
	}
	return out
}

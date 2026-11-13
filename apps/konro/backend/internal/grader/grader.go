// Package grader scores one runtime output for an evaluation case.
//
// It first applies the same acceptance gate production uses
// (importer.Interpret), then the case's expectations. Checks are exact
// substring/count tests after NFKC and whitespace normalization; they detect
// regressions, not semantic equivalence. Executions that did not succeed are
// not_evaluated, never silently scored as failures or passes.
package grader

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/evalcases"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/importer"
	"golang.org/x/text/unicode/norm"
)

// Version changes whenever scoring rules change, so scores from different
// rules are never compared as if they were the same metric.
const Version = "konro-import-grader/1"

const (
	Passed       = "passed"
	Failed       = "failed"
	NotEvaluated = "not_evaluated"
)

type Check struct {
	Name string `json:"name"`
	OK   bool   `json:"ok"`
}

type Result struct {
	CaseID    string   `json:"case_id"`
	Status    string   `json:"status"`
	Score     float64  `json:"score"` // passed checks / total checks; 0 when the gate rejects
	Gate      string   `json:"gate"`  // "" accepted, or the production rejection reason
	Checks    []Check  `json:"checks"`
	Reasons   []string `json:"reasons"`
	JobState  string   `json:"job_state"`
	JobReason string   `json:"job_reason,omitempty"`
	Version   string   `json:"grader_version"`
}

// Prepare reproduces what production gives the model for pasted text.
func Prepare(c evalcases.Case) (importer.Source, importer.Content) {
	s := importer.Source{Raw: c.Text, Text: c.Text}
	content, _ := importer.Extract(s)
	return s, content
}

func Grade(c evalcases.Case, jobState, jobReason string, output json.RawMessage) Result {
	r := Result{CaseID: c.ID, JobState: jobState, JobReason: jobReason, Version: Version, Checks: []Check{}, Reasons: []string{}}
	if jobState != "succeeded" {
		r.Status = NotEvaluated
		r.Reasons = append(r.Reasons, "job_"+jobState)
		return r
	}
	source, draft := Prepare(c)
	in := importer.Interpret(source.Text, draft, output)
	r.Gate = in.Reason
	if in.Reason != "" {
		r.Status = Failed
		r.Reasons = append(r.Reasons, "gate_"+in.Reason)
		return r
	}
	got := in.Content
	e := c.Expect
	add := func(name string, ok bool, reason string) {
		r.Checks = append(r.Checks, Check{name, ok})
		if !ok {
			r.Reasons = append(r.Reasons, reason)
		}
	}
	add("title_contains:"+e.TitleContains, strings.Contains(normalize(got.Title), normalize(e.TitleContains)), "title_mismatch")
	if e.Servings != nil {
		add(fmt.Sprintf("servings:%q", *e.Servings), normalize(got.Servings) == normalize(*e.Servings), "servings_mismatch")
	}
	for _, v := range e.IngredientsContain {
		add("ingredient:"+v, anyContains(got.Ingredients, v), "ingredient_missing")
	}
	for _, v := range e.StepsContain {
		add("step:"+v, anyContains(got.Steps, v), "step_detail_missing")
	}
	all := append(append([]string{got.Title, got.Servings}, got.Ingredients...), got.Steps...)
	for _, v := range e.Forbid {
		add("forbid:"+v, !anyContains(all, v), "forbidden_content")
	}
	if e.StepsMin != nil {
		add(fmt.Sprintf("steps>=%d", *e.StepsMin), len(got.Steps) >= *e.StepsMin, "too_few_steps")
	}
	if e.StepsMax != nil {
		add(fmt.Sprintf("steps<=%d", *e.StepsMax), len(got.Steps) <= *e.StepsMax, "invented_or_extra_steps")
	}
	if len(e.AssessmentContainsAny) > 0 {
		add("assessment_explains", anyContains([]string{in.Assessment}, e.AssessmentContainsAny...), "assessment_missing_reason")
	}
	passed := 0
	for _, ch := range r.Checks {
		if ch.OK {
			passed++
		}
	}
	r.Score = float64(passed) / float64(len(r.Checks))
	r.Status = Passed
	if passed != len(r.Checks) {
		r.Status = Failed
	}
	return r
}

func anyContains(lines []string, needles ...string) bool {
	for _, line := range lines {
		n := normalize(line)
		for _, needle := range needles {
			if strings.Contains(n, normalize(needle)) {
				return true
			}
		}
	}
	return false
}

func normalize(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, norm.NFKC.String(s))
}

// Summary aggregates results without dropping unevaluated cases.
type Summary struct {
	Total        int     `json:"total"`
	Passed       int     `json:"passed"`
	Failed       int     `json:"failed"`
	NotEvaluated int     `json:"not_evaluated"`
	MeanScore    float64 `json:"mean_score"` // over evaluated cases only
}

func Summarize(results []Result) Summary {
	s := Summary{Total: len(results)}
	sum := 0.0
	for _, r := range results {
		switch r.Status {
		case Passed:
			s.Passed++
		case Failed:
			s.Failed++
		default:
			s.NotEvaluated++
		}
		if r.Status != NotEvaluated {
			sum += r.Score
		}
	}
	if evaluated := s.Passed + s.Failed; evaluated > 0 {
		s.MeanScore = sum / float64(evaluated)
	}
	return s
}

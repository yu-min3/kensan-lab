// konro-llmops-adapter transforms captured inputs and grades annotated outputs.
// It has no network, storage, runtime client or formal-save operations.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/importer"
)

const domainVersion = "konro.recipe-import.v1"
const maxRequest = 8 << 20

var requestID = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,127}$`)

type request struct {
	ProtocolVersion string          `json:"protocol_version"`
	RequestID       string          `json:"request_id"`
	Operation       string          `json:"operation"`
	DomainVersion   string          `json:"domain_schema_version"`
	Payload         json.RawMessage `json:"payload"`
}
type problem struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
type response struct {
	ProtocolVersion string   `json:"protocol_version"`
	RequestID       string   `json:"request_id"`
	Result          any      `json:"result,omitempty"`
	Error           *problem `json:"error,omitempty"`
}

func strict(raw []byte, dst any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e := d.Decode(dst); e != nil {
		return e
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("one JSON value required")
	}
	return nil
}
func main() {
	if run(os.Stdin, os.Stdout) != nil {
		os.Exit(1)
	}
}
func run(in io.Reader, out io.Writer) error {
	raw, e := io.ReadAll(io.LimitReader(in, maxRequest+1))
	var r request
	if e != nil || len(raw) > maxRequest || !utf8.Valid(raw) || strict(raw, &r) != nil || !requestID.MatchString(r.RequestID) {
		return errors.New("invalid request envelope")
	}
	res := response{ProtocolVersion: "1", RequestID: r.RequestID}
	if r.ProtocolVersion != "1" || r.DomainVersion != domainVersion {
		res.Error = &problem{"unsupported_version", "unsupported adapter version"}
	} else {
		switch r.Operation {
		case "prepare":
			res.Result, e = prepare(r.Payload)
		case "grade":
			res.Result, e = grade(r.Payload)
		default:
			e = errors.New("unsupported operation")
		}
		if e != nil {
			res.Result = nil
			res.Error = &problem{"invalid_input", e.Error()}
		}
	}
	return json.NewEncoder(out).Encode(res)
}

type prepareInput struct {
	SourceURI string `json:"source_uri"`
	MIMEType  string `json:"mime_type"`
	Raw       string `json:"raw"`
}
type prepared struct {
	Text          string           `json:"text"`
	TextBytes     int              `json:"text_bytes"`
	TextTruncated bool             `json:"text_truncated"`
	Content       importer.Content `json:"content"`
	Warnings      []string         `json:"warnings"`
	OutputSchema  json.RawMessage  `json:"output_schema"`
}

func prepare(raw []byte) (any, error) {
	var p prepareInput
	if strict(raw, &p) != nil || len(p.Raw) == 0 || len(p.Raw) > importer.MaxSource || !utf8.ValidString(p.Raw) {
		return nil, errors.New("invalid captured source")
	}
	var s importer.Source
	switch p.MIMEType {
	case "text/html":
		var e error
		s, e = importer.SourceFromHTML(p.SourceURI, p.Raw)
		if e != nil {
			return nil, e
		}
	case "text/plain", "text/markdown":
		if len(p.Raw) > importer.MaxText {
			return nil, errors.New("plain text exceeds prepared input limit")
		}
		s = importer.Source{Raw: p.Raw, Text: p.Raw}
	default:
		return nil, errors.New("unsupported source MIME type")
	}
	c, w := importer.Extract(s)
	return prepared{s.Text, len(s.Text), s.TextTruncated, c, w, json.RawMessage(importer.Schema)}, nil
}

type gradeInput struct {
	Output              json.RawMessage   `json:"output"`
	Expected            *importer.Content `json:"expected,omitempty"`
	AnnotationStatus    string            `json:"annotation_status"`
	ExpectedConstraints json.RawMessage   `json:"expected_constraints,omitempty"`
}
type graded struct {
	Status           string          `json:"status"`
	Checks           map[string]bool `json:"checks"`
	Reasons          []string        `json:"reasons"`
	AnnotationStatus string          `json:"annotation_status"`
	Oracle           string          `json:"oracle"`
}

func grade(raw []byte) (any, error) {
	var p gradeInput
	if strict(raw, &p) != nil || (p.AnnotationStatus != "confirmed" && p.AnnotationStatus != "reviewed" && p.AnnotationStatus != "synthetic" && p.AnnotationStatus != "unreviewed") {
		return nil, errors.New("invalid grading input")
	}
	c, _, err := importer.DecodeAgentOutput(p.Output)
	g := graded{Status: "needs_review", Checks: map[string]bool{}, Reasons: []string{}, AnnotationStatus: p.AnnotationStatus, Oracle: "none"}
	g.Checks["valid_recipe"] = err == nil && importer.Validate(c) == nil
	if !g.Checks["valid_recipe"] {
		g.Status = "failed"
		g.Reasons = append(g.Reasons, "invalid_recipe")
		return g, nil
	}
	if p.AnnotationStatus == "unreviewed" {
		g.Reasons = append(g.Reasons, "annotation_not_confirmed")
		return g, nil
	}
	var constraints struct {
		Expected                *importer.Content `json:"expected,omitempty"`
		RequiredIngredients     []string          `json:"required_ingredients,omitempty"`
		RequiredSteps           []string          `json:"required_steps,omitempty"`
		ForbiddenTerms          []string          `json:"forbidden_terms,omitempty"`
		PreserveUnknownServings bool              `json:"preserve_unknown_servings,omitempty"`
	}
	if len(p.ExpectedConstraints) > 0 {
		if strict(p.ExpectedConstraints, &constraints) != nil {
			g.Checks["constraints_supported"] = false
			g.Reasons = append(g.Reasons, "unsupported_constraints")
			return g, nil
		}
		if constraints.Expected != nil {
			if p.Expected != nil && !reflect.DeepEqual(*p.Expected, *constraints.Expected) {
				return nil, errors.New("conflicting expected records")
			}
			p.Expected = constraints.Expected
		}
	}
	// Partial constraints are exact line/substring checks, never a semantic
	// guarantee based on a global set of numeric tokens.
	g.Oracle = "partial_constraints"
	contains := func(lines []string, term string) bool {
		for _, line := range lines {
			if line == term {
				return true
			}
		}
		return false
	}
	for _, line := range constraints.RequiredIngredients {
		ok := contains(c.Ingredients, line)
		g.Checks["required_ingredient:"+line] = ok
		if !ok {
			g.Status = "failed"
			g.Reasons = append(g.Reasons, "required_ingredient_missing")
		}
	}
	for _, line := range constraints.RequiredSteps {
		ok := contains(c.Steps, line)
		g.Checks["required_step:"+line] = ok
		if !ok {
			g.Status = "failed"
			g.Reasons = append(g.Reasons, "required_step_missing")
		}
	}
	for _, term := range constraints.ForbiddenTerms {
		if term == "" {
			return nil, errors.New("empty forbidden term")
		}
		ok := !strings.Contains(c.Title+"\n"+c.Servings+"\n"+strings.Join(c.Ingredients, "\n")+"\n"+strings.Join(c.Steps, "\n"), term)
		g.Checks["forbidden_term_absent:"+term] = ok
		if !ok {
			g.Status = "failed"
			g.Reasons = append(g.Reasons, "forbidden_term_present")
		}
	}
	if constraints.PreserveUnknownServings {
		ok := c.Servings == ""
		g.Checks["unknown_servings_preserved"] = ok
		if !ok {
			g.Status = "failed"
			g.Reasons = append(g.Reasons, "invented_servings")
		}
	}
	if p.Expected == nil {
		if g.Status != "failed" {
			g.Reasons = append(g.Reasons, "complete_expectation_missing")
		}
		sort.Strings(g.Reasons)
		return g, nil
	}

	if importer.Validate(*p.Expected) != nil {
		return nil, errors.New("invalid confirmed expected recipe")
	}
	g.Oracle = "exact_record"
	if g.Status != "failed" {
		g.Status = "passed"
	}
	for name, pair := range map[string][2]any{"title": {c.Title, p.Expected.Title}, "servings": {c.Servings, p.Expected.Servings}, "ingredients": {c.Ingredients, p.Expected.Ingredients}, "steps": {c.Steps, p.Expected.Steps}} {
		g.Checks[name] = reflect.DeepEqual(pair[0], pair[1])
		if !g.Checks[name] {
			g.Status = "failed"
			g.Reasons = append(g.Reasons, fmt.Sprintf("%s_mismatch", name))
		}
	}
	sort.Strings(g.Reasons)
	return g, nil
}

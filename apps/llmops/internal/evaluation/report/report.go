// Package report renders an explicitly paired, immutable experiment comparison.
package report

import (
	"bytes"
	"encoding/json"
	"errors"
	"html/template"
	"io"
	"regexp"
	"unicode/utf8"

	"github.com/yu-min3/kensan-lab/apps/llmops/internal/evaluation/graders"
)

const MaxManifestBytes = 16 << 20

var identifier = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,127}$`)

type Snapshot struct {
	Table string `json:"table"`
	ID    string `json:"snapshot_id"`
}
type Dataset struct {
	AppID     string     `json:"app_id"`
	TaskID    string     `json:"task_id"`
	ReleaseID string     `json:"release_id"`
	Snapshots []Snapshot `json:"snapshots"`
}
type Variant struct {
	RunID            string `json:"run_id"`
	DatasetReleaseID string `json:"dataset_release_id"`
	PromptVersion    string `json:"prompt_version"`
	CodeVersion      string `json:"code_version"`
	EvaluatorVersion string `json:"evaluator_version"`
	ExecutionSource  string `json:"execution_source"`
}
type Source struct {
	ObjectID         string `json:"object_id"`
	SHA256           string `json:"sha256"`
	URI              string `json:"uri"`
	AnnotationStatus string `json:"annotation_status"`
	Input            string `json:"input"`
}
type Usage struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
}
type Attempt struct {
	CaseID     string             `json:"case_id"`
	RunID      string             `json:"run_id"`
	ID         string             `json:"attempt_id"`
	State      string             `json:"state"`
	Reason     string             `json:"reason"`
	Output     string             `json:"output"`
	Model      *string            `json:"model"`
	Usage      *Usage             `json:"usage"`
	Evaluation graders.Evaluation `json:"evaluation"`
}
type Case struct {
	ID        string   `json:"case_id"`
	Source    Source   `json:"source"`
	Current   *Attempt `json:"current"`
	Candidate *Attempt `json:"candidate"`
}
type Comparison struct {
	SchemaVersion string  `json:"schema_version"`
	ID            string  `json:"comparison_id"`
	Mode          string  `json:"mode"`
	Dataset       Dataset `json:"dataset"`
	Current       Variant `json:"current"`
	Candidate     Variant `json:"candidate"`
	Cases         []Case  `json:"cases"`
}

func validateAttempt(a *Attempt, c Case, v Variant, mode string) error {
	if a == nil {
		return nil
	}
	if a.CaseID != c.ID || a.RunID != v.RunID || !identifier.MatchString(a.ID) {
		return errors.New("attempt identity mismatch")
	}
	switch a.State {
	case "queued", "running", "waiting", "succeeded", "failed", "cancelled":
	default:
		return errors.New("unknown execution state")
	}
	g := a.Evaluation
	if g.CaseID != c.ID || g.AttemptID != a.ID || g.Mode != mode || g.EvaluatorVersion != v.EvaluatorVersion || g.ExecutionState != a.State || g.AnnotationStatus != c.Source.AnnotationStatus {
		return errors.New("evaluation provenance mismatch")
	}
	switch g.Status {
	case "passed", "failed", "needs_review":
		if !g.Evaluated || a.State != "succeeded" {
			return errors.New("grading state mismatch")
		}
	case "not_evaluated":
		if g.Evaluated {
			return errors.New("grading state mismatch")
		}
	default:
		return errors.New("unknown evaluation status")
	}
	if c.Source.AnnotationStatus == "unreviewed" && g.Status == "passed" {
		return errors.New("unreviewed pass is invalid")
	}
	if a.Usage != nil && (a.Usage.InputTokens < 0 || a.Usage.OutputTokens < 0) {
		return errors.New("invalid token usage")
	}
	return nil
}
func (c Comparison) Validate() error {
	if c.SchemaVersion != "1" || !identifier.MatchString(c.ID) || !identifier.MatchString(c.Dataset.AppID) || !identifier.MatchString(c.Dataset.TaskID) || !identifier.MatchString(c.Dataset.ReleaseID) || (c.Mode != "prepared" && c.Mode != "raw") || len(c.Dataset.Snapshots) == 0 {
		return errors.New("invalid comparison manifest")
	}
	for _, v := range []Variant{c.Current, c.Candidate} {
		if !identifier.MatchString(v.RunID) || v.DatasetReleaseID != c.Dataset.ReleaseID || v.PromptVersion == "" || v.CodeVersion == "" || v.EvaluatorVersion == "" || (v.ExecutionSource != "fixture" && v.ExecutionSource != "runtime") {
			return errors.New("invalid variant")
		}
	}
	if c.Current.RunID == c.Candidate.RunID || c.Current.EvaluatorVersion != c.Candidate.EvaluatorVersion {
		return errors.New("comparison variants are not comparable")
	}
	tables := map[string]bool{}
	for _, s := range c.Dataset.Snapshots {
		if s.Table == "" || s.ID == "" || tables[s.Table] {
			return errors.New("invalid snapshot reference")
		}
		for _, ch := range s.ID {
			if ch < '0' || ch > '9' {
				return errors.New("snapshot must be fixed numeric ID")
			}
		}
		tables[s.Table] = true
	}
	cases := map[string]bool{}
	for _, item := range c.Cases {
		if !identifier.MatchString(item.ID) || cases[item.ID] || !identifier.MatchString(item.Source.ObjectID) || len(item.Source.SHA256) != 64 {
			return errors.New("invalid case/source identity")
		}
		cases[item.ID] = true
		for _, ch := range item.Source.SHA256 {
			if !(ch >= '0' && ch <= '9' || ch >= 'a' && ch <= 'f') {
				return errors.New("invalid source hash")
			}
		}
		switch item.Source.AnnotationStatus {
		case "synthetic", "reviewed", "unreviewed":
		default:
			return errors.New("invalid annotation status")
		}
		if err := validateAttempt(item.Current, item, c.Current, c.Mode); err != nil {
			return err
		}
		if err := validateAttempt(item.Candidate, item, c.Candidate, c.Mode); err != nil {
			return err
		}
	}
	return nil
}

// Decode rejects unknown fields and trailing JSON. Artifact resolution and
// authorization happen before constructing this manifest, outside the renderer.
func Decode(r io.Reader) (Comparison, error) {
	raw, err := io.ReadAll(io.LimitReader(r, MaxManifestBytes+1))
	if err != nil || len(raw) > MaxManifestBytes || !utf8.Valid(raw) {
		return Comparison{}, errors.New("comparison manifest too large")
	}
	if err := uniqueJSON(json.NewDecoder(bytes.NewReader(raw)), 0); err != nil {
		return Comparison{}, errors.New("ambiguous comparison JSON")
	}
	var c Comparison
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&c) != nil || d.Decode(new(any)) != io.EOF {
		return Comparison{}, errors.New("invalid comparison JSON")
	}
	if err = c.Validate(); err != nil {
		return Comparison{}, err
	}
	return c, nil
}
func status(a *Attempt) string {
	if a == nil {
		return "missing"
	}
	return a.Evaluation.Status
}
func change(c Case) string {
	a, b := status(c.Current), status(c.Candidate)
	if a == "passed" && b == "failed" {
		return "回帰"
	}
	if a == "failed" && b == "passed" {
		return "改善"
	}
	if a == "passed" && b == "passed" {
		return "両方合格"
	}
	if a == "failed" && b == "failed" {
		return "両方不合格"
	}
	return "要確認 / 比較未完了"
}
func label(v string) string {
	switch v {
	case "passed":
		return "合格"
	case "failed":
		return "不合格"
	case "needs_review":
		return "人の確認待ち"
	case "not_evaluated":
		return "採点対象外"
	case "missing":
		return "結果未記録"
	case "synthetic":
		return "合成fixture注釈"
	case "reviewed":
		return "確認済み注釈"
	case "unreviewed":
		return "未確認注釈"
	case "fixture":
		return "fixture実行"
	case "runtime":
		return "runtime経由"
	}
	return v
}

type totals struct{ Total, Evaluated, Passed, Failed, NeedsReview, Missing int }

func count(cases []Case, candidate bool) totals {
	s := totals{Total: len(cases)}
	for _, c := range cases {
		a := c.Current
		if candidate {
			a = c.Candidate
		}
		if a == nil {
			s.Missing++
			continue
		}
		if a.Evaluation.Evaluated {
			s.Evaluated++
		}
		switch a.Evaluation.Status {
		case "passed":
			s.Passed++
		case "failed":
			s.Failed++
		case "needs_review":
			s.NeedsReview++
		}
	}
	return s
}

// Write only uses html/template autoescaping. It does not fetch URIs, resolve
// paths, load remote resources, or execute model-generated HTML/JavaScript.
func Write(w io.Writer, c Comparison) error {
	if err := c.Validate(); err != nil {
		return err
	}
	t, err := template.New("comparison").Funcs(template.FuncMap{"label": label, "change": change, "changeClass": func(c Case) string {
		if change(c) == "回帰" {
			return "failed"
		}
		if change(c) == "改善" {
			return "passed"
		}
		return "needs_review"
	}, "status": status, "model": func(a *Attempt) string {
		if a == nil || a.Model == nil || *a.Model == "" {
			return "未取得"
		}
		return *a.Model
	}, "countCurrent": func() totals { return count(c.Cases, false) }, "countCandidate": func() totals { return count(c.Cases, true) }}).Parse(page)
	if err != nil {
		return err
	}
	return t.Execute(w, c)
}

func uniqueJSON(d *json.Decoder, depth int) error {
	if depth > 64 {
		return errors.New("JSON nesting exceeded")
	}
	token, err := d.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok || seen[name] {
				return errors.New("duplicate JSON key")
			}
			seen[name] = true
			if err = uniqueJSON(d, depth+1); err != nil {
				return err
			}
		}
		end, err := d.Token()
		if err != nil || end != json.Delim('}') {
			return errors.New("invalid object")
		}
	case '[':
		for d.More() {
			if err = uniqueJSON(d, depth+1); err != nil {
				return err
			}
		}
		end, err := d.Token()
		if err != nil || end != json.Delim(']') {
			return errors.New("invalid array")
		}
	default:
		return errors.New("invalid JSON delimiter")
	}
	return nil
}

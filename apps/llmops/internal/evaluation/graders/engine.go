// Package graders runs versioned schema and domain checks without discarding
// executions that could not be graded.
package graders

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"regexp"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/yu-min3/kensan-lab/apps/llmops/contracts"
)

const MaxEnvelopeBytes = 2 << 20

// Invoke exchanges one complete versioned adapter envelope, without shell text.
type Invoke func(context.Context, json.RawMessage) (json.RawMessage, error)
type Engine struct {
	Version string
	Invoke  Invoke
}
type Case struct {
	CaseID              string          `json:"case_id"`
	AttemptID           string          `json:"attempt_id"`
	RequestID           string          `json:"request_id"`
	Mode                string          `json:"mode"` // prepared or raw; provenance supplied by runner
	DomainSchemaVersion string          `json:"domain_schema_version"`
	AnnotationStatus    string          `json:"annotation_status"`
	ExpectedConstraints json.RawMessage `json:"expected_constraints"`
	OutputSchema        json.RawMessage `json:"output_schema"`
	Output              json.RawMessage `json:"output"`
	State               string          `json:"state"`
	Reason              string          `json:"reason,omitempty"`
}
type Evaluation struct {
	CaseID           string          `json:"case_id"`
	AttemptID        string          `json:"attempt_id"`
	Mode             string          `json:"mode"`
	EvaluatorVersion string          `json:"evaluator_version"`
	AnnotationStatus string          `json:"annotation_status"`
	ExecutionState   string          `json:"execution_state"`
	ExecutionReason  string          `json:"execution_reason,omitempty"`
	Status           string          `json:"status"` // passed, failed, needs_review, not_evaluated
	Evaluated        bool            `json:"evaluated"`
	Checks           map[string]bool `json:"checks"`
	Reasons          []string        `json:"reasons"`
}
type result struct {
	Status           string          `json:"status"`
	Checks           map[string]bool `json:"checks"`
	Reasons          []string        `json:"reasons"`
	AnnotationStatus string          `json:"annotation_status,omitempty"`
	Oracle           string          `json:"oracle,omitempty"`
}
type envelope struct {
	ProtocolVersion string          `json:"protocol_version"`
	RequestID       string          `json:"request_id"`
	Result          json.RawMessage `json:"result"`
	Error           *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}
type offline struct{}

func (offline) Load(string) (any, error) {
	return nil, errors.New("external schema references disabled")
}

var idPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,127}$`)

func object(raw []byte) (map[string]any, error) {
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil || m == nil {
		return nil, errors.New("object required")
	}
	return m, nil
}
func schema(raw json.RawMessage) (*jsonschema.Schema, error) {
	if len(raw) == 0 || len(raw) > 32768 {
		return nil, errors.New("invalid output schema")
	}
	v, err := object(raw)
	if err != nil {
		return nil, err
	}
	c := jsonschema.NewCompiler()
	c.UseLoader(offline{})
	c.AssertFormat()
	if err = c.AddResource("urn:evaluation:output", v); err != nil {
		return nil, err
	}
	return c.Compile("urn:evaluation:output")
}

// Evaluate always returns execution metadata, including on grading dependency
// errors. Caller must persist that result and the error instead of dropping it.
func (e Engine) Evaluate(ctx context.Context, c Case) (Evaluation, error) {
	out := Evaluation{CaseID: c.CaseID, AttemptID: c.AttemptID, Mode: c.Mode, EvaluatorVersion: e.Version, AnnotationStatus: c.AnnotationStatus, ExecutionState: c.State, ExecutionReason: c.Reason, Status: "not_evaluated", Checks: map[string]bool{}, Reasons: []string{}}
	if e.Version == "" || !idPattern.MatchString(c.CaseID) || !idPattern.MatchString(c.AttemptID) || !idPattern.MatchString(c.RequestID) || c.DomainSchemaVersion == "" || (c.Mode != "prepared" && c.Mode != "raw") || (c.AnnotationStatus != "reviewed" && c.AnnotationStatus != "synthetic" && c.AnnotationStatus != "unreviewed") {
		out.Reasons = append(out.Reasons, "invalid_case")
		return out, errors.New("invalid grading case")
	}
	switch c.State {
	case "queued", "running", "waiting", "failed", "cancelled":
		out.Reasons = append(out.Reasons, "execution_not_succeeded")
		return out, nil
	case "succeeded":
	default:
		out.Reasons = append(out.Reasons, "unknown_execution_state")
		return out, errors.New("unknown execution state")
	}
	compiled, err := schema(c.OutputSchema)
	if err != nil {
		out.Reasons = append(out.Reasons, "invalid_evaluator_schema")
		return out, errors.New("invalid evaluator schema")
	}
	var value any
	if len(c.Output) > 1<<20 || json.Unmarshal(c.Output, &value) != nil || compiled.Validate(value) != nil {
		out.Evaluated = true
		out.Status = "failed"
		out.Checks["schema"] = false
		out.Reasons = append(out.Reasons, "schema_mismatch")
		return out, nil
	}
	out.Checks["schema"] = true
	constraints := c.ExpectedConstraints
	if len(constraints) == 0 {
		constraints = json.RawMessage(`{}`)
	}
	if _, err = object(constraints); err != nil {
		out.Reasons = append(out.Reasons, "invalid_constraints")
		return out, errors.New("invalid expected constraints")
	}
	payload := map[string]any{"output": c.Output, "expected_constraints": constraints, "annotation_status": c.AnnotationStatus}
	request, _ := json.Marshal(map[string]any{"protocol_version": "1", "request_id": c.RequestID, "operation": "grade", "domain_schema_version": c.DomainSchemaVersion, "payload": payload})
	if len(request) > MaxEnvelopeBytes || e.Invoke == nil {
		out.Reasons = append(out.Reasons, "adapter_unavailable")
		return out, errors.New("grading adapter unavailable")
	}
	response, err := e.Invoke(ctx, request)
	if err != nil {
		out.Reasons = append(out.Reasons, "adapter_unavailable")
		return out, errors.New("grading adapter unavailable")
	}
	if len(response) > MaxEnvelopeBytes {
		out.Reasons = append(out.Reasons, "invalid_adapter_response")
		return out, errors.New("invalid adapter response")
	}
	var decoded any
	if json.Unmarshal(response, &decoded) != nil || contracts.Validate("adapter-response", decoded) != nil {
		out.Reasons = append(out.Reasons, "invalid_adapter_response")
		return out, errors.New("invalid adapter response")
	}
	var env envelope
	if json.Unmarshal(response, &env) != nil || env.RequestID != c.RequestID || env.ProtocolVersion != "1" || env.Error != nil {
		out.Reasons = append(out.Reasons, "adapter_rejected")
		return out, errors.New("grading adapter rejected request")
	}
	var r result
	d := json.NewDecoder(bytes.NewReader(env.Result))
	d.DisallowUnknownFields()
	if d.Decode(&r) != nil || d.Decode(new(any)) != io.EOF || r.Checks == nil || r.Reasons == nil || (r.Status != "passed" && r.Status != "failed" && r.Status != "needs_review") {
		out.Reasons = append(out.Reasons, "invalid_adapter_result")
		return out, errors.New("invalid adapter result")
	}
	if _, exists := r.Checks["schema"]; exists {
		out.Reasons = append(out.Reasons, "reserved_adapter_check")
		return out, errors.New("reserved schema check")
	}
	if r.AnnotationStatus != "" && r.AnnotationStatus != c.AnnotationStatus {
		out.Reasons = append(out.Reasons, "annotation_status_mismatch")
		return out, errors.New("adapter annotation mismatch")
	}
	for name, ok := range r.Checks {
		out.Checks[name] = ok
	}
	out.Reasons = append(out.Reasons, r.Reasons...)
	out.Status = r.Status
	out.Evaluated = true
	if len(r.Checks) == 0 && out.Status == "passed" {
		out.Status = "needs_review"
		out.Reasons = append(out.Reasons, "domain_checks_missing")
	}
	if c.AnnotationStatus == "unreviewed" && out.Status == "passed" {
		out.Status = "needs_review"
		out.Reasons = append(out.Reasons, "annotation_not_reviewed")
	}
	if out.Status == "passed" {
		for _, ok := range out.Checks {
			if !ok {
				out.Status = "failed"
				out.Reasons = append(out.Reasons, "contradictory_adapter_pass")
				break
			}
		}
	}
	return out, nil
}

// Summary preserves the full execution denominator. NeedsReview is counted as
// evaluated but never as passed. Execution failures remain ungraded entries.
type Summary struct {
	Total           int            `json:"total"`
	Evaluated       int            `json:"evaluated"`
	Passed          int            `json:"passed"`
	Failed          int            `json:"failed"`
	NeedsReview     int            `json:"needs_review"`
	NotEvaluated    int            `json:"not_evaluated"`
	ExecutionStates map[string]int `json:"execution_states"`
}

func Summarize(rows []Evaluation) Summary {
	s := Summary{ExecutionStates: map[string]int{}}
	for _, r := range rows {
		s.Total++
		s.ExecutionStates[r.ExecutionState]++
		if r.Evaluated {
			s.Evaluated++
		}
		switch r.Status {
		case "passed":
			s.Passed++
		case "failed":
			s.Failed++
		case "needs_review":
			s.NeedsReview++
		default:
			s.NotEvaluated++
		}
	}
	return s
}

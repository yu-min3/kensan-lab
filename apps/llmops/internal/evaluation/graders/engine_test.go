package graders

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func sample() Case {
	return Case{CaseID: "case-1", AttemptID: "attempt-1", RequestID: "request-1", Mode: "prepared", DomainSchemaVersion: "sample-v1", AnnotationStatus: "synthetic", ExpectedConstraints: json.RawMessage(`{"required":"sample"}`), OutputSchema: json.RawMessage(`{"type":"object","required":["value"],"additionalProperties":false,"properties":{"value":{"type":"string"}}}`), Output: json.RawMessage(`{"value":"sample"}`), State: "succeeded"}
}
func invokeResult(status string, checks map[string]bool) Invoke {
	return func(_ context.Context, raw json.RawMessage) (json.RawMessage, error) {
		var request map[string]json.RawMessage
		json.Unmarshal(raw, &request)
		var id string
		json.Unmarshal(request["request_id"], &id)
		return json.Marshal(map[string]any{"protocol_version": "1", "request_id": id, "result": map[string]any{"status": status, "checks": checks, "reasons": []string{}}})
	}
}
func TestExecutionDenominatorAndProvenance(t *testing.T) {
	calls := 0
	invoke := invokeResult("passed", map[string]bool{"fidelity": true})
	engine := Engine{Version: "grader-v1", Invoke: func(ctx context.Context, b json.RawMessage) (json.RawMessage, error) { calls++; return invoke(ctx, b) }}
	rows := []Evaluation{}
	for _, state := range []string{"succeeded", "waiting", "failed", "cancelled", "queued", "running"} {
		c := sample()
		c.State = state
		c.Reason = "observed-state"
		c.Mode = "raw"
		v, e := engine.Evaluate(context.Background(), c)
		if e != nil {
			t.Fatal(e)
		}
		if v.Mode != "raw" || v.AnnotationStatus != "synthetic" || v.ExecutionReason != "observed-state" {
			t.Fatal("provenance lost")
		}
		rows = append(rows, v)
	}
	summary := Summarize(rows)
	if calls != 1 || summary.Total != 6 || summary.Evaluated != 1 || summary.Passed != 1 || summary.NotEvaluated != 5 || summary.ExecutionStates["failed"] != 1 {
		t.Fatalf("summary %+v calls %d", summary, calls)
	}
}
func TestUnreviewedAndUnknownMeaningCannotPass(t *testing.T) {
	c := sample()
	c.AnnotationStatus = "unreviewed"
	engine := Engine{Version: "v1", Invoke: invokeResult("passed", map[string]bool{"fidelity": true})}
	v, e := engine.Evaluate(context.Background(), c)
	if e != nil || v.Status != "needs_review" || v.AnnotationStatus != "unreviewed" {
		t.Fatalf("unreviewed %+v %v", v, e)
	}
	engine.Invoke = invokeResult("needs_review", map[string]bool{"partial": true})
	c.AnnotationStatus = "reviewed"
	v, e = engine.Evaluate(context.Background(), c)
	if e != nil || v.Status != "needs_review" {
		t.Fatal(e)
	}
	engine.Invoke = invokeResult("passed", map[string]bool{})
	v, e = engine.Evaluate(context.Background(), c)
	if e != nil || v.Status != "needs_review" {
		t.Fatal("empty checks passed")
	}
}
func TestSchemaFailureStopsAdapter(t *testing.T) {
	calls := 0
	engine := Engine{Version: "v1", Invoke: func(context.Context, json.RawMessage) (json.RawMessage, error) {
		calls++
		return nil, errors.New("must not invoke")
	}}
	for _, output := range []string{`{"value":42}`, `{"other":"x"}`, `{"value":"x","shell":"id"}`, `not JSON`} {
		c := sample()
		c.Output = json.RawMessage(output)
		v, e := engine.Evaluate(context.Background(), c)
		if e != nil || v.Status != "failed" || !v.Evaluated || v.Checks["schema"] {
			t.Fatalf("schema %+v %v", v, e)
		}
	}
	if calls != 0 {
		t.Fatal("adapter called for schema-invalid output")
	}
}
func TestMalformedAdapterRetainsEntry(t *testing.T) {
	for _, reply := range []string{`not JSON`, `{"protocol_version":"1","request_id":"other","result":{}}`, `{"protocol_version":"1","request_id":"request-1","error":{"code":"invalid_input","message":"bad"}}`, `{"protocol_version":"1","request_id":"request-1","result":{"status":"passed","checks":{},"reasons":[]},"command":"sh"}`} {
		engine := Engine{Version: "v1", Invoke: func(context.Context, json.RawMessage) (json.RawMessage, error) { return json.RawMessage(reply), nil }}
		v, e := engine.Evaluate(context.Background(), sample())
		if e == nil || v.Status != "not_evaluated" || v.CaseID != "case-1" || v.ExecutionState != "succeeded" {
			t.Fatalf("failed dependency dropped entry %+v %v", v, e)
		}
	}
}
func TestContradictoryAndReservedChecks(t *testing.T) {
	engine := Engine{Version: "v1", Invoke: invokeResult("passed", map[string]bool{"fidelity": false})}
	v, e := engine.Evaluate(context.Background(), sample())
	if e != nil || v.Status != "failed" {
		t.Fatalf("false check %+v %v", v, e)
	}
	engine.Invoke = invokeResult("passed", map[string]bool{"schema": true})
	v, e = engine.Evaluate(context.Background(), sample())
	if e == nil || v.Status != "not_evaluated" {
		t.Fatal("adapter overwrote schema check")
	}
}
func TestInvalidSchemaIsEvaluatorError(t *testing.T) {
	c := sample()
	c.OutputSchema = json.RawMessage(`{"$ref":"https://untrusted.example/remote"}`)
	engine := Engine{Version: "v1", Invoke: invokeResult("passed", map[string]bool{"fidelity": true})}
	v, e := engine.Evaluate(context.Background(), c)
	if e == nil || v.Evaluated || v.Status != "not_evaluated" {
		t.Fatalf("invalid schema incorrectly charged model %+v %v", v, e)
	}
}

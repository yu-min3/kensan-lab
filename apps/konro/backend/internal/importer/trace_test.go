package importer

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/agentclient"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

type traceAgent struct {
	fakeAgent
	parents []string
}

func (a *traceAgent) Create(ctx context.Context, r agentclient.Request) (agentclient.Job, error) {
	a.parents = append(a.parents, telemetry.Traceparent(ctx))
	return a.fakeAgent.Create(ctx, r)
}

// One import is one trace, even though submit and apply happen in separate
// requests and the service is reopened from file state in between.
func TestImportIsOneTraceAcrossRequests(t *testing.T) {
	rec := tracetest.NewSpanRecorder()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec)))
	otel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() { otel.SetTracerProvider(sdktrace.NewTracerProvider()) })
	t.Setenv("KONRO_TRACE_CONTENT", "true")

	ctx := context.Background()
	dir := t.TempDir()
	a := &traceAgent{fakeAgent: fakeAgent{result: agentclient.Job{ID: "generic-job", State: "succeeded", Output: json.RawMessage(`{"title":"煮魚","servings":"2人分","ingredients":["魚 2切れ"],"steps":["10分煮る"],"assessment":"読めた"}`)}}}
	d, err := New(dir, a).Create(ctx, "", "煮魚 2人分 魚2切れ 10分煮る")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = New(dir, a).Start(ctx, d.ID, d.Revision, "steps"); err != nil {
		t.Fatal(err)
	}
	s := New(dir, a)
	if _, err = s.Get(ctx, d.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Get(ctx, d.ID); err != nil { // a second poll must not duplicate the apply span
		t.Fatal(err)
	}

	byName := map[string][]sdktrace.ReadOnlySpan{}
	for _, sp := range rec.Ended() {
		byName[sp.Name()] = append(byName[sp.Name()], sp)
	}
	root := byName["konro.import"]
	if len(root) != 1 || len(byName["konro.import.submit"]) != 1 || len(byName["konro.import.apply"]) != 1 || len(byName["konro.import.extract"]) != 1 {
		t.Fatalf("unexpected spans: %v", byName)
	}
	traceID := root[0].SpanContext().TraceID()
	for name, spans := range byName {
		for _, sp := range spans {
			if sp.SpanContext().TraceID() != traceID {
				t.Fatalf("%s is in another trace", name)
			}
		}
	}
	submit := byName["konro.import.submit"][0]
	if len(a.parents) != 1 || a.parents[0] == "" || a.parents[0][36:52] != submit.SpanContext().SpanID().String() {
		t.Fatalf("runtime job not parented to the submit span: %v", a.parents)
	}
	attrs := map[string]string{}
	for _, kv := range byName["konro.import.apply"][0].Attributes() {
		attrs[string(kv.Key)] = kv.Value.Emit()
	}
	if attrs["konro.outcome"] != "applied" || attrs["konro.prompt.version"] != PromptVersion("steps") || attrs["langfuse.observation.output"] == "" {
		t.Fatalf("apply attributes: %v", attrs)
	}
}

func TestContentNotCapturedByDefault(t *testing.T) {
	rec := tracetest.NewSpanRecorder()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec)))
	t.Cleanup(func() { otel.SetTracerProvider(sdktrace.NewTracerProvider()) })
	t.Setenv("KONRO_TRACE_CONTENT", "")
	a := &fakeAgent{result: agentclient.Job{ID: "generic-job", State: "succeeded", Output: json.RawMessage(`{"title":"x","servings":"","ingredients":["a"],"steps":["b"]}`)}}
	s := New(t.TempDir(), a)
	d, _ := s.Create(context.Background(), "", "x a b")
	_, _ = s.Start(context.Background(), d.ID, d.Revision, "steps")
	for _, sp := range rec.Ended() {
		for _, kv := range sp.Attributes() {
			if kv.Key == telemetry.KeyInput || kv.Key == telemetry.KeyOutput {
				t.Fatalf("%s carried content", sp.Name())
			}
		}
	}
}

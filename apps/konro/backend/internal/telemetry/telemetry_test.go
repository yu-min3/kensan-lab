package telemetry

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestSetupWithoutEndpointIsNoop(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "")
	shutdown, err := Setup(context.Background())
	if err != nil || shutdown(context.Background()) != nil {
		t.Fatal(err)
	}
	if Traceparent(context.Background()) != "" {
		t.Fatal("traceparent without span")
	}
}

func TestResumeKeepsTraceAcrossRequests(t *testing.T) {
	rec := tracetest.NewSpanRecorder()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec)))
	otel.SetTextMapPropagator(propagator)
	ctx, root := Tracer().Start(context.Background(), "root")
	stored := Traceparent(ctx)
	root.End()
	_, later := Tracer().Start(Resume(context.Background(), stored), "later")
	later.End()
	spans := rec.Ended()
	if len(spans) != 2 || spans[1].Parent().SpanID() != spans[0].SpanContext().SpanID() || spans[1].SpanContext().TraceID() != spans[0].SpanContext().TraceID() {
		t.Fatal("resumed span not parented to stored context")
	}
	if Resume(context.Background(), "garbage") == nil {
		t.Fatal("nil context")
	}
}

func TestVersionAndContentBounds(t *testing.T) {
	if Version("a", "b") == Version("ab") || len(Version("x")) != 12 {
		t.Fatal("version")
	}
	if len(Content(string(make([]byte, 40<<10)))) != 32<<10 {
		t.Fatal("content not bounded")
	}
}

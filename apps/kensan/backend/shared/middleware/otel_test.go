package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func setupOTelTracer(t *testing.T) *tracetest.InMemoryExporter {
	t.Helper()
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	return exporter
}

func TestOTelTrace_CreatesSpan(t *testing.T) {
	exporter := setupOTelTracer(t)

	handler := OTelTrace("test-service")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	spans := exporter.GetSpans()
	require.NotEmpty(t, spans, "expected at least one span to be created")
}

func TestOTelTrace_SpanNameIsMethod(t *testing.T) {
	exporter := setupOTelTracer(t)

	handler := OTelTrace("test-service")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	spans := exporter.GetSpans()
	require.NotEmpty(t, spans)
	// The SpanNameFormatter should set the name to just the HTTP method.
	assert.Equal(t, "POST", spans[0].Name)
}

func TestOTelTrace_PropagatesContext(t *testing.T) {
	setupOTelTracer(t)

	var spanCtx trace.SpanContext
	handler := OTelTrace("test-service")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		spanCtx = trace.SpanFromContext(r.Context()).SpanContext()
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.True(t, spanCtx.IsValid(), "expected valid span context propagated to handler")
	assert.True(t, spanCtx.HasTraceID(), "expected trace ID in context")
	assert.True(t, spanCtx.HasSpanID(), "expected span ID in context")
}

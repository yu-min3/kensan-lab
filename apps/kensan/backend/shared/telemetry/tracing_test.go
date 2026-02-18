package telemetry

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// setupTestTracer configures an in-memory TracerProvider and returns the exporter.
func setupTestTracer(t *testing.T) *tracetest.InMemoryExporter {
	t.Helper()
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	return exporter
}

func TestServiceTracer_NamingConvention(t *testing.T) {
	exporter := setupTestTracer(t)

	tracer := ServiceTracer("task")
	_, span := tracer.Start(context.Background(), "test-op")
	span.End()

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	assert.Equal(t, "task.service", spans[0].InstrumentationLibrary.Name)
}

func TestStartSpan_CreatesSpan(t *testing.T) {
	exporter := setupTestTracer(t)
	tracer := ServiceTracer("test")

	ctx, end := StartSpan(context.Background(), tracer, "CreateTask")
	end(nil)

	// Context should carry the span.
	assert.NotNil(t, ctx)

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	assert.Equal(t, "CreateTask", spans[0].Name)
}

func TestStartSpan_RecordsError(t *testing.T) {
	exporter := setupTestTracer(t)
	tracer := ServiceTracer("test")

	testErr := errors.New("something failed")
	_, end := StartSpan(context.Background(), tracer, "FailOp")
	end(testErr)

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	assert.Equal(t, codes.Error, spans[0].Status.Code)
	assert.Equal(t, "something failed", spans[0].Status.Description)

	// Should have a recorded error event.
	require.NotEmpty(t, spans[0].Events)
	assert.Equal(t, "exception", spans[0].Events[0].Name)
}

func TestStartSpan_Success(t *testing.T) {
	exporter := setupTestTracer(t)
	tracer := ServiceTracer("test")

	_, end := StartSpan(context.Background(), tracer, "SuccessOp")
	end(nil)

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	// When no error, span status should be Unset (the default — not explicitly set to OK).
	assert.Equal(t, codes.Unset, spans[0].Status.Code)
}

func TestStartSpan_WithAttributes(t *testing.T) {
	exporter := setupTestTracer(t)
	tracer := ServiceTracer("test")

	_, end := StartSpan(context.Background(), tracer, "AttrOp",
		attribute.String("task.id", "abc-123"),
		attribute.Int("task.priority", 1),
	)
	end(nil)

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)

	attrMap := make(map[string]interface{})
	for _, a := range spans[0].Attributes {
		attrMap[string(a.Key)] = a.Value.AsInterface()
	}
	assert.Equal(t, "abc-123", attrMap["task.id"])
	assert.Equal(t, int64(1), attrMap["task.priority"])
}

package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

// setupMetricsTest configures an in-memory meter provider with a manual reader,
// and an in-memory tracer provider. It overrides the package-level histogram.
func setupMetricsTest(t *testing.T) (*sdkmetric.ManualReader, *tracetest.InMemoryExporter) {
	t.Helper()

	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(mp)

	// Re-initialize the package-level histogram with the new meter provider.
	meter := mp.Meter("kensan/http")
	var err error
	httpServerRequestDuration, err = meter.Float64Histogram(
		"http.server.request.duration",
	)
	require.NoError(t, err)

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	otel.SetTracerProvider(tp)

	t.Cleanup(func() {
		_ = mp.Shutdown(context.Background())
		_ = tp.Shutdown(context.Background())
	})

	return reader, exporter
}

// withChiRouteContext injects a chi.RouteContext with the given pattern into the request.
func withChiRouteContext(r *http.Request, pattern string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.RoutePatterns = []string{pattern}
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func TestMetrics_RecordsDuration(t *testing.T) {
	reader, _ := setupMetricsTest(t)

	handler := Metrics(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
	req = withChiRouteContext(req, "/api/v1/tasks")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var rm metricdata.ResourceMetrics
	err := reader.Collect(context.Background(), &rm)
	require.NoError(t, err)
	require.NotEmpty(t, rm.ScopeMetrics)
	require.NotEmpty(t, rm.ScopeMetrics[0].Metrics)

	m := rm.ScopeMetrics[0].Metrics[0]
	assert.Equal(t, "http.server.request.duration", m.Name)

	hist, ok := m.Data.(metricdata.Histogram[float64])
	require.True(t, ok)
	require.NotEmpty(t, hist.DataPoints)
	assert.Greater(t, hist.DataPoints[0].Count, uint64(0))
}

func TestMetrics_CorrectAttributes(t *testing.T) {
	reader, _ := setupMetricsTest(t)

	handler := Metrics(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks", nil)
	req = withChiRouteContext(req, "/api/v1/tasks")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var rm metricdata.ResourceMetrics
	err := reader.Collect(context.Background(), &rm)
	require.NoError(t, err)

	hist := rm.ScopeMetrics[0].Metrics[0].Data.(metricdata.Histogram[float64])
	dp := hist.DataPoints[0]

	attrMap := make(map[string]interface{})
	for _, kv := range dp.Attributes.ToSlice() {
		attrMap[string(kv.Key)] = kv.Value.AsInterface()
	}

	assert.Equal(t, "POST", attrMap["http.request.method"])
	assert.Equal(t, "/api/v1/tasks", attrMap["http.route"])
	assert.Equal(t, int64(201), attrMap["http.response.status_code"])
}

func TestMetrics_UpdatesSpanName(t *testing.T) {
	_, exporter := setupMetricsTest(t)

	// Create a span in the request context so Metrics can update its name.
	tracer := otel.Tracer("test")

	handler := Metrics(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
	req = withChiRouteContext(req, "/api/v1/tasks")

	// Wrap request with an active span.
	ctx, span := tracer.Start(req.Context(), "GET")
	defer span.End()
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// End the span so it gets exported.
	span.End()

	spans := exporter.GetSpans()
	require.NotEmpty(t, spans)

	// Find the span that was updated by Metrics middleware.
	var found bool
	for _, s := range spans {
		if s.Name == "GET /api/v1/tasks" {
			found = true
			// Verify http.route attribute was set.
			for _, a := range s.Attributes {
				if string(a.Key) == "http.route" {
					assert.Equal(t, "/api/v1/tasks", a.Value.AsInterface())
				}
			}
			break
		}
	}
	assert.True(t, found, "expected span name to be updated to 'GET /api/v1/tasks'")
}

func TestMetricsResponseWriter_WriteHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &metricsResponseWriter{ResponseWriter: rec, statusCode: http.StatusOK}

	w.WriteHeader(http.StatusNotFound)

	assert.Equal(t, http.StatusNotFound, w.statusCode)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestMetricsResponseWriter_Unwrap(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &metricsResponseWriter{ResponseWriter: rec}

	assert.Equal(t, rec, w.Unwrap())
}

func TestMetrics_UnknownRoute(t *testing.T) {
	reader, _ := setupMetricsTest(t)

	handler := Metrics(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/unknown", nil)
	// Inject a chi route context with empty pattern.
	rctx := chi.NewRouteContext()
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var rm metricdata.ResourceMetrics
	err := reader.Collect(context.Background(), &rm)
	require.NoError(t, err)

	hist := rm.ScopeMetrics[0].Metrics[0].Data.(metricdata.Histogram[float64])
	dp := hist.DataPoints[0]

	for _, kv := range dp.Attributes.ToSlice() {
		if string(kv.Key) == "http.route" {
			assert.Equal(t, "unknown", kv.Value.AsInterface())
		}
	}
}

// Ensure metricsResponseWriter correctly captures span context.
func TestMetrics_SpanNotValid_NoUpdate(t *testing.T) {
	_, exporter := setupMetricsTest(t)

	handler := Metrics(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify no panic when span context is not valid.
		span := trace.SpanFromContext(r.Context())
		assert.False(t, span.SpanContext().IsValid())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req = withChiRouteContext(req, "/test")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// No spans should be updated since there's no valid span.
	spans := exporter.GetSpans()
	assert.Empty(t, spans)
}

// Suppress unused import lint.
var _ = attribute.String

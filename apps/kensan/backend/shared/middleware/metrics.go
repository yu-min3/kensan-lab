package middleware

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// OTel HTTP Server Semantic Conventions
// https://opentelemetry.io/docs/specs/semconv/http/http-metrics/

var httpServerRequestDuration metric.Float64Histogram

func init() {
	meter := otel.Meter("kensan/http")

	httpServerRequestDuration, _ = meter.Float64Histogram(
		"http.server.request.duration",
		metric.WithDescription("Duration of HTTP server requests"),
		metric.WithUnit("s"),
	)
}

// Metrics records HTTP server request duration following OTel Semantic Conventions.
// Rate and error rate are derived from the histogram using attribute filters.
func Metrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		wrapped := &metricsResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(wrapped, r)

		routePattern := chi.RouteContext(r.Context()).RoutePattern()
		if routePattern == "" {
			routePattern = "unknown"
		}

		duration := time.Since(start).Seconds()
		attrs := metric.WithAttributes(
			attribute.String("http.request.method", r.Method),
			attribute.String("http.route", routePattern),
			attribute.Int("http.response.status_code", wrapped.statusCode),
		)
		httpServerRequestDuration.Record(r.Context(), duration, attrs)

		// Update trace span with routed name and http.route attribute
		span := trace.SpanFromContext(r.Context())
		if span.SpanContext().IsValid() {
			span.SetName(r.Method + " " + routePattern)
			span.SetAttributes(semconv.HTTPRoute(routePattern))
		}
	})
}

type metricsResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *metricsResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

// Unwrap returns the underlying ResponseWriter (for http.ResponseController).
func (w *metricsResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

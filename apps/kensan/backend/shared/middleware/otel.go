package middleware

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
)

// OTelTrace returns middleware that instruments HTTP requests with OpenTelemetry spans.
// Metrics recording is disabled here (delegated to the Metrics middleware).
// Span name is set to METHOD only; the Metrics middleware updates it with the route pattern after routing.
func OTelTrace(serviceName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return otelhttp.NewHandler(next, serviceName,
			otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
				return r.Method
			}),
			otelhttp.WithMeterProvider(metricnoop.NewMeterProvider()),
		)
	}
}

package telemetry

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// InstrumentedTransport wraps an http.RoundTripper with OTel trace propagation.
// If base is nil, http.DefaultTransport is used.
func InstrumentedTransport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return otelhttp.NewTransport(base)
}

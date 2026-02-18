package telemetry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestInitialize_Disabled(t *testing.T) {
	cfg := Config{
		ServiceName: "test-service",
		Enabled:     false,
	}

	p, err := Initialize(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, p)

	// All providers should be nil (no-op).
	assert.Nil(t, p.tracerProvider)
	assert.Nil(t, p.meterProvider)
	assert.Nil(t, p.loggerProvider)
	assert.Nil(t, p.LoggerProvider())
}

func TestInitialize_DefaultValues(t *testing.T) {
	// We can't easily test that default values are applied to the resource
	// without an enabled config (which requires a collector). Instead, verify
	// the logic branch: empty ServiceVersion/Environment should not panic
	// when Enabled=false and providers are nil.
	cfg := Config{
		ServiceName:    "test-service",
		ServiceVersion: "",
		Environment:    "",
		Enabled:        false,
	}

	p, err := Initialize(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, p)
}

func TestProvider_Shutdown_NilProviders(t *testing.T) {
	p := &Provider{}
	err := p.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestProvider_Shutdown_Success(t *testing.T) {
	// Build real (in-memory) providers to test shutdown path.
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))

	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	lp := sdklog.NewLoggerProvider()

	p := &Provider{
		tracerProvider: tp,
		meterProvider:  mp,
		loggerProvider: lp,
	}

	err := p.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestProvider_LoggerProvider(t *testing.T) {
	lp := sdklog.NewLoggerProvider()
	p := &Provider{loggerProvider: lp}

	assert.Equal(t, lp, p.LoggerProvider())
}

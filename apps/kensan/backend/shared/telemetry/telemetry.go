// Package telemetry provides OpenTelemetry initialization for Kensan services.
package telemetry

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Config holds OpenTelemetry configuration.
type Config struct {
	ServiceName    string
	ServiceVersion string
	Environment    string // e.g. "development", "production"
	CollectorURL   string // host:port for OTLP HTTP endpoint
	Enabled        bool
}

// Provider wraps OTel providers for graceful shutdown.
type Provider struct {
	tracerProvider *sdktrace.TracerProvider
	meterProvider  *sdkmetric.MeterProvider
	loggerProvider *sdklog.LoggerProvider
}

// Initialize sets up OpenTelemetry TracerProvider and MeterProvider.
// If cfg.Enabled is false, global providers remain as no-op defaults.
func Initialize(ctx context.Context, cfg Config) (*Provider, error) {
	if !cfg.Enabled {
		return &Provider{}, nil
	}

	serviceVersion := cfg.ServiceVersion
	if serviceVersion == "" {
		serviceVersion = "dev"
	}
	environment := cfg.Environment
	if environment == "" {
		environment = "development"
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(cfg.ServiceName),
			semconv.ServiceVersionKey.String(serviceVersion),
			attribute.String("deployment.environment", environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("telemetry: create resource: %w", err)
	}

	// Trace exporter (OTLP HTTP)
	traceExporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(cfg.CollectorURL),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("telemetry: create trace exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	// Metric exporter (OTLP HTTP)
	metricExporter, err := otlpmetrichttp.New(ctx,
		otlpmetrichttp.WithEndpoint(cfg.CollectorURL),
		otlpmetrichttp.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("telemetry: create metric exporter: %w", err)
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(mp)

	// Log exporter (OTLP HTTP)
	logExporter, err := otlploghttp.New(ctx,
		otlploghttp.WithEndpoint(cfg.CollectorURL),
		otlploghttp.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("telemetry: create log exporter: %w", err)
	}

	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter)),
		sdklog.WithResource(res),
	)

	// Propagator: W3C TraceContext + Baggage
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return &Provider{
		tracerProvider: tp,
		meterProvider:  mp,
		loggerProvider: lp,
	}, nil
}

// LoggerProvider returns the OTel LoggerProvider for use with otelslog bridge.
// Returns nil if OTel is not enabled.
func (p *Provider) LoggerProvider() *sdklog.LoggerProvider {
	return p.loggerProvider
}

// Shutdown gracefully shuts down OTel providers.
func (p *Provider) Shutdown(ctx context.Context) error {
	if p.tracerProvider == nil && p.meterProvider == nil && p.loggerProvider == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if p.tracerProvider != nil {
		if err := p.tracerProvider.Shutdown(ctx); err != nil {
			return fmt.Errorf("telemetry: shutdown tracer: %w", err)
		}
	}
	if p.meterProvider != nil {
		if err := p.meterProvider.Shutdown(ctx); err != nil {
			return fmt.Errorf("telemetry: shutdown meter: %w", err)
		}
	}
	if p.loggerProvider != nil {
		if err := p.loggerProvider.Shutdown(ctx); err != nil {
			return fmt.Errorf("telemetry: shutdown logger: %w", err)
		}
	}
	return nil
}

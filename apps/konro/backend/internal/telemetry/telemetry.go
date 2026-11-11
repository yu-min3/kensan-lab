// Package telemetry wires OpenTelemetry tracing for Konro.
//
// Tracing is optional: without OTEL_EXPORTER_OTLP_ENDPOINT (or the traces
// variant) every call here is a no-op and Konro behaves as before. Exporting is
// asynchronous, so an unavailable collector never fails a user request.
//
// LLM work in Konro is asynchronous (submit now, poll later), so the trace
// context of the operation that started it is persisted with the draft or
// proposal and restored with Resume when the result is applied.
package telemetry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"runtime/debug"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

const App = "konro"

// Attribute keys. Langfuse maps langfuse.* keys; the rest are Konro's own and
// are stable across GenAI semantic-convention renames.
const (
	KeyApp           = attribute.Key("app")
	KeyTask          = attribute.Key("task")
	KeyPromptVersion = attribute.Key("konro.prompt.version")
	KeyCodeVersion   = attribute.Key("konro.code.version")
	KeyJobID         = attribute.Key("agent_runtime.job.id")
	KeyJobState      = attribute.Key("agent_runtime.job.state")
	KeyJobReason     = attribute.Key("agent_runtime.job.reason")
	KeyOutcome       = attribute.Key("konro.outcome")
	KeyInput         = attribute.Key("langfuse.observation.input")
	KeyOutput        = attribute.Key("langfuse.observation.output")
	KeyTraceName     = attribute.Key("langfuse.trace.name")
	KeyPromptName    = attribute.Key("langfuse.observation.prompt.name")
	KeyRelease       = attribute.Key("langfuse.release")
)

var propagator = propagation.TraceContext{}

// Tracer is safe to use before or without Setup (no-op until configured).
func Tracer() trace.Tracer { return otel.Tracer("github.com/yu-min3/kensan-lab/apps/konro") }

// Setup installs an OTLP/gRPC exporter when the standard OTEL endpoint
// variables are present. The returned shutdown flushes pending spans.
func Setup(ctx context.Context) (func(context.Context) error, error) {
	otel.SetTextMapPropagator(propagator)
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" && os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT") == "" {
		return func(context.Context) error { return nil }, nil
	}
	exporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		return nil, err
	}
	res, err := resource.Merge(resource.Default(), resource.NewSchemaless(
		attribute.String("service.name", App),
		attribute.String("service.version", CodeVersion()),
		KeyApp.String(App),
	))
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exporter), sdktrace.WithResource(res))
	otel.SetTracerProvider(tp)
	return tp.Shutdown, nil
}

// CaptureContent reports whether LLM input/output may be attached to spans.
// The collector removes these attributes on the operations path regardless.
func CaptureContent() bool { return os.Getenv("KONRO_TRACE_CONTENT") == "true" }

// Traceparent serializes the current span context ("" when not recording).
func Traceparent(ctx context.Context) string {
	carrier := propagation.MapCarrier{}
	propagator.Inject(ctx, carrier)
	return carrier.Get("traceparent")
}

// Resume returns ctx parented to a stored traceparent, or ctx unchanged.
func Resume(ctx context.Context, traceparent string) context.Context {
	if traceparent == "" {
		return ctx
	}
	return propagator.Extract(ctx, propagation.MapCarrier{"traceparent": traceparent})
}

// Version is a short content hash, used for prompt and schema versions.
func Version(parts ...string) string {
	h := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(h[:6])
}

// CodeVersion is the VCS revision embedded at build time, or KONRO_VERSION.
func CodeVersion() string {
	if v := os.Getenv("KONRO_VERSION"); v != "" {
		return v
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" && len(s.Value) >= 12 {
				return s.Value[:12]
			}
		}
	}
	return "dev"
}

// Content truncates text attached to spans to a bounded size.
func Content(s string) string {
	const max = 32 << 10
	if len(s) <= max {
		return s
	}
	return s[:max]
}

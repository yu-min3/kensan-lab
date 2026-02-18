package telemetry

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// ServiceTracer returns a tracer scoped to a service's business logic layer.
func ServiceTracer(serviceName string) trace.Tracer {
	return otel.Tracer(serviceName + ".service")
}

// StartSpan starts a new span for a service operation.
// Returns the enriched context and a function to end the span.
// Usage:
//
//	ctx, end := telemetry.StartSpan(ctx, tracer, "CreateTask")
//	defer end(err)
func StartSpan(ctx context.Context, tracer trace.Tracer, operation string, attrs ...attribute.KeyValue) (context.Context, func(error)) {
	ctx, span := tracer.Start(ctx, operation, trace.WithAttributes(attrs...))
	return ctx, func(err error) {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}
}

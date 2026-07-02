// Package telemetry は OpenTelemetry のセットアップを提供する。
// OTEL_EXPORTER_OTLP_ENDPOINT が未設定なら no-op（ローカル開発の既定）。
package telemetry

import (
	"context"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Setup は trace provider を初期化する。shutdown 関数と有効フラグを返す。
func Setup(ctx context.Context, service string) (func(context.Context) error, bool, error) {
	noop := func(context.Context) error { return nil }
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" {
		return noop, false, nil
	}
	exp, err := otlptracehttp.New(ctx)
	if err != nil {
		return noop, false, err
	}
	// NewSchemaless で Merge する: Default() は SDK 同梱の semconv schema を持ち、
	// バージョン固定の semconv.SchemaURL と衝突して Merge がエラーになるため
	// （OTEL_SERVICE_NAME 環境変数があれば Default() 側がそれを優先する）
	res, err := sdkresource.Merge(
		sdkresource.Default(),
		sdkresource.NewSchemaless(semconv.ServiceName(service)),
	)
	if err != nil {
		return noop, false, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))
	return tp.Shutdown, true, nil
}

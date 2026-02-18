// Package logging provides common logging setup for all services.
package logging

import (
	"context"
	"log/slog"
	"os"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	sdklog "go.opentelemetry.io/otel/sdk/log"
)

// Setup configures the global slog logger based on the environment.
// In production, it outputs JSON logs to stdout.
// In non-production, it outputs human-readable text logs to stderr.
func Setup(env string) {
	var handler slog.Handler
	if env == "production" {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{})
	} else {
		handler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{})
	}
	slog.SetDefault(slog.New(handler))
}

// SetupWithOTel configures the global slog logger with both stdout/stderr output
// and OTel LoggerProvider via otelslog bridge (fan-out handler).
func SetupWithOTel(env string, provider *sdklog.LoggerProvider) {
	var stdHandler slog.Handler
	if env == "production" {
		stdHandler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{})
	} else {
		stdHandler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{})
	}

	otelHandler := otelslog.NewHandler("", otelslog.WithLoggerProvider(provider))
	slog.SetDefault(slog.New(&fanOutHandler{handlers: []slog.Handler{stdHandler, otelHandler}}))
}

// fanOutHandler sends log records to multiple slog handlers.
type fanOutHandler struct {
	handlers []slog.Handler
}

func (h *fanOutHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h *fanOutHandler) Handle(ctx context.Context, record slog.Record) error {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, record.Level) {
			if err := handler.Handle(ctx, record); err != nil {
				return err
			}
		}
	}
	return nil
}

func (h *fanOutHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		handlers[i] = handler.WithAttrs(attrs)
	}
	return &fanOutHandler{handlers: handlers}
}

func (h *fanOutHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		handlers[i] = handler.WithGroup(name)
	}
	return &fanOutHandler{handlers: handlers}
}

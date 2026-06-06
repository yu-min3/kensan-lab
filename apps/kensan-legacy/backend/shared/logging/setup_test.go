package logging

import (
	"context"
	"log/slog"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdklog "go.opentelemetry.io/otel/sdk/log"
)

// mockHandler is a test slog.Handler that records calls.
type mockHandler struct {
	mu        sync.Mutex
	enabled   bool
	records   []slog.Record
	attrCalls int
	groupName string
}

func newMockHandler(enabled bool) *mockHandler {
	return &mockHandler{enabled: enabled}
}

func (h *mockHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return h.enabled
}

func (h *mockHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r)
	return nil
}

func (h *mockHandler) WithAttrs(_ []slog.Attr) slog.Handler {
	return &mockHandler{enabled: h.enabled, attrCalls: h.attrCalls + 1}
}

func (h *mockHandler) WithGroup(name string) slog.Handler {
	return &mockHandler{enabled: h.enabled, groupName: name}
}

func TestSetup_Production(t *testing.T) {
	Setup("production")

	handler := slog.Default().Handler()
	_, ok := handler.(*slog.JSONHandler)
	assert.True(t, ok, "expected JSONHandler in production")
}

func TestSetup_Development(t *testing.T) {
	Setup("development")

	handler := slog.Default().Handler()
	_, ok := handler.(*slog.TextHandler)
	assert.True(t, ok, "expected TextHandler in development")
}

func TestSetupWithOTel_FanOut(t *testing.T) {
	lp := sdklog.NewLoggerProvider()
	t.Cleanup(func() { _ = lp.Shutdown(context.Background()) })

	SetupWithOTel("development", lp)

	handler := slog.Default().Handler()
	fan, ok := handler.(*fanOutHandler)
	require.True(t, ok, "expected fanOutHandler")
	assert.Len(t, fan.handlers, 2)
}

func TestFanOutHandler_Enabled(t *testing.T) {
	tests := []struct {
		name     string
		handlers []slog.Handler
		want     bool
	}{
		{
			name:     "all disabled",
			handlers: []slog.Handler{newMockHandler(false), newMockHandler(false)},
			want:     false,
		},
		{
			name:     "one enabled",
			handlers: []slog.Handler{newMockHandler(false), newMockHandler(true)},
			want:     true,
		},
		{
			name:     "all enabled",
			handlers: []slog.Handler{newMockHandler(true), newMockHandler(true)},
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &fanOutHandler{handlers: tt.handlers}
			got := h.Enabled(context.Background(), slog.LevelInfo)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFanOutHandler_Handle(t *testing.T) {
	h1 := newMockHandler(true)
	h2 := newMockHandler(true)
	h3 := newMockHandler(false) // disabled — should not receive the record.

	fan := &fanOutHandler{handlers: []slog.Handler{h1, h2, h3}}

	record := slog.Record{}
	record.Level = slog.LevelInfo
	record.Message = "test message"

	err := fan.Handle(context.Background(), record)
	require.NoError(t, err)

	assert.Len(t, h1.records, 1)
	assert.Equal(t, "test message", h1.records[0].Message)
	assert.Len(t, h2.records, 1)
	assert.Equal(t, "test message", h2.records[0].Message)
	assert.Empty(t, h3.records, "disabled handler should not receive records")
}

func TestFanOutHandler_WithAttrs(t *testing.T) {
	h1 := newMockHandler(true)
	h2 := newMockHandler(true)
	fan := &fanOutHandler{handlers: []slog.Handler{h1, h2}}

	result := fan.WithAttrs([]slog.Attr{slog.String("key", "val")})
	newFan, ok := result.(*fanOutHandler)
	require.True(t, ok)
	assert.Len(t, newFan.handlers, 2)
	// Each inner handler should be a new mockHandler with incremented attrCalls.
	for _, inner := range newFan.handlers {
		m := inner.(*mockHandler)
		assert.Equal(t, 1, m.attrCalls)
	}
}

func TestFanOutHandler_WithGroup(t *testing.T) {
	h1 := newMockHandler(true)
	h2 := newMockHandler(true)
	fan := &fanOutHandler{handlers: []slog.Handler{h1, h2}}

	result := fan.WithGroup("mygroup")
	newFan, ok := result.(*fanOutHandler)
	require.True(t, ok)
	assert.Len(t, newFan.handlers, 2)
	for _, inner := range newFan.handlers {
		m := inner.(*mockHandler)
		assert.Equal(t, "mygroup", m.groupName)
	}
}

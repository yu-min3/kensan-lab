package planner

import (
	"context"
	"testing"

	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/mealplan"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestMenuProposalIsOneTrace(t *testing.T) {
	rec := tracetest.NewSpanRecorder()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec)))
	otel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() { otel.SetTracerProvider(sdktrace.NewTracerProvider()) })
	root := fixture(t)
	agent := &fakeAgent{output: `{"interpretation":"x","menus":[{"picks":[{"file":"1"},{"file":"2"}]}]}`}
	s := New(root, agent, mealplan.New(root))
	p, err := s.Create(context.Background(), Conditions{Count: 2, Side: 2}, "ai")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = New(root, agent, mealplan.New(root)).Get(context.Background(), p.ID); err != nil {
		t.Fatal(err)
	}
	names := map[string]int{}
	var trace string
	for _, sp := range rec.Ended() {
		names[sp.Name()]++
		if trace == "" {
			trace = sp.SpanContext().TraceID().String()
		}
		if sp.SpanContext().TraceID().String() != trace {
			t.Fatalf("%s in another trace", sp.Name())
		}
	}
	if names["konro.menu"] != 1 || names["konro.menu.submit"] != 1 || names["konro.menu.apply"] != 1 {
		t.Fatalf("spans %v", names)
	}
}

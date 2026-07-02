package telemetry

import (
	"net/http"
	"testing"
)

func TestInstrumentedTransport_NilBase(t *testing.T) {
	rt := InstrumentedTransport(nil)
	if rt == nil {
		t.Fatal("expected non-nil RoundTripper")
	}
}

func TestInstrumentedTransport_CustomBase(t *testing.T) {
	custom := &http.Transport{}
	rt := InstrumentedTransport(custom)
	if rt == nil {
		t.Fatal("expected non-nil RoundTripper")
	}
}

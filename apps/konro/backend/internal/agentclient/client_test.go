package agentclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestTransportRejectsRedirectAndInvalidResponses(t *testing.T) {
	for _, mode := range []string{"redirect", "invalid", "unauthorized"} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch mode {
			case "redirect":
				http.Redirect(w, r, "https://other.example/", 302)
			case "invalid":
				w.Write([]byte(`{"id":"x","state":"unknown"}`))
			default:
				w.WriteHeader(401)
			}
		}))
		c, e := New(srv.URL, func(context.Context) (string, error) { return "private", nil })
		if e != nil {
			t.Fatal(e)
		}
		if _, e = c.Get(context.Background(), "job"); e == nil {
			t.Error("accepted", mode)
		}
		srv.Close()
	}
	for _, raw := range []string{"http://runtime.local", "file:///tmp/socket", "https://user:pw@example.org", "https://runtime.example/path"} {
		if _, e := New(raw, func(context.Context) (string, error) { return "a", nil }); e == nil {
			t.Fatal("accepted unsafe configuration", raw)
		}
	}
}

func TestProjectedCredentialAllowsOnlyPrivateGroup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secret")
	if e := os.WriteFile(path, []byte("fixture"), 0440); e != nil {
		t.Fatal(e)
	}
	link := filepath.Join(dir, "projected")
	if e := os.Symlink(path, link); e != nil {
		t.Fatal(e)
	}
	if got, e := privateFile(link); e != nil || got != "fixture" {
		t.Fatal(got, e)
	}
	if e := os.Chmod(path, 0444); e != nil {
		t.Fatal(e)
	}
	if _, e := privateFile(link); e == nil {
		t.Fatal("world readable accepted")
	}
}

func TestTraceparentSentOnlyWhenEnabled(t *testing.T) {
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		v, _ := body["traceparent"].(string)
		got = append(got, v)
		w.WriteHeader(202)
		_, _ = w.Write([]byte(`{"id":"j","state":"queued"}`))
	}))
	defer srv.Close()
	otel.SetTextMapPropagator(propagation.TraceContext{})
	otel.SetTracerProvider(sdktrace.NewTracerProvider())
	ctx, span := otel.Tracer("t").Start(context.Background(), "s")
	defer span.End()
	for _, enabled := range []string{"", "true"} {
		t.Setenv("KONRO_AGENT_TRACE", enabled)
		c, err := New("http://127.0.0.1"+srv.URL[len("http://127.0.0.1"):], func(context.Context) (string, error) { return "t", nil })
		if err != nil {
			t.Fatal(err)
		}
		if _, err = c.Create(ctx, Request{Profile: "text", Traceparent: "00-ffffffffffffffffffffffffffffffff-ffffffffffffffff-01"}); err != nil {
			t.Fatal(err)
		}
	}
	if got[0] != "" || len(got[1]) != 55 || got[1][:3] != "00-" {
		t.Fatalf("traceparent gating: %q", got)
	}
}

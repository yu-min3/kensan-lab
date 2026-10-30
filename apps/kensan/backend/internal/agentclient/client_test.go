package agentclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
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

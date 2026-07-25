package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStars(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Error("authorization header missing")
		}
		_, _ = w.Write([]byte(`{"stargazers_count":7}`))
	}))
	defer server.Close()
	client := server.Client()
	client.Transport = rewriteTransport{base: server.URL, next: client.Transport}
	value, err := NewWithHTTPClient(client, "token").Stars(context.Background(), "owner/repo")
	if err != nil || value != 7 {
		t.Fatalf("value=%v err=%v", value, err)
	}
}

type rewriteTransport struct {
	base string
	next http.RoundTripper
}

func (t rewriteTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r.URL.Scheme = "http"
	r.URL.Host = strings.TrimPrefix(t.base, "http://")
	return t.next.RoundTrip(r)
}

package web

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/core"
)

func TestPrivateWebLoginCSRFAndTask(t *testing.T) {
	dir := t.TempDir()
	store, err := core.Open(filepath.Join(dir, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.SeedKnowledge(); err != nil {
		t.Fatal(err)
	}
	tokenFile := filepath.Join(dir, "admin-token")
	if err := os.WriteFile(tokenFile, []byte(strings.Repeat("t", 64)), 0600); err != nil {
		t.Fatal(err)
	}
	css := filepath.Join(dir, "tokens.css")
	if err := os.WriteFile(css, []byte(":root{--background:40 27% 94%}"), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := New(store, tokenFile, css, false)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(s.Handler())
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	resp, err := client.Get(server.URL + "/login")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("login page: %d", resp.StatusCode)
	}
	resp.Body.Close()
	resp, err = client.Get(server.URL + "/static/app.css")
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("public stylesheet unavailable: %v %v", resp, err)
	}
	resp.Body.Close()
	resp, err = client.PostForm(server.URL+"/login", url.Values{"token": {strings.Repeat("t", 64)}})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("login failed: %d %s", resp.StatusCode, b)
	}
	csrfMatch := regexp.MustCompile(`name="csrf" value="([a-f0-9]+)"`).FindSubmatch(b)
	if len(csrfMatch) != 2 {
		t.Fatal("CSRF token missing from authenticated page")
	}
	form := url.Values{"mission": {"golden-path"}, "team": {"platform"}, "kind": {"change"}, "title": {"契約を確認する"}, "contract": {"draft-v1"}}
	resp, err = client.PostForm(server.URL+"/api/tasks", form)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if len(store.Snapshot().Tasks) != 0 {
		t.Fatal("task accepted without CSRF")
	}
	form.Set("csrf", string(csrfMatch[1]))
	resp, err = client.PostForm(server.URL+"/api/tasks", form)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if len(store.Snapshot().Tasks) != 1 {
		t.Fatal("valid task not persisted")
	}
	if len(store.Snapshot().Agents) != 4 {
		t.Fatal("change task did not receive four independent stage agents")
	}
	resp, err = client.Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	b, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(b), "契約を確認する") || !strings.Contains(string(b), "Platform") {
		t.Fatal("task missing from UI")
	}
}

func TestNonLoopbackBindRejected(t *testing.T) {
	for _, addr := range []string{"0.0.0.0:8787", "[::]:8787", "192.168.0.113:8787", "localhost:8787"} {
		if err := LoopbackOnly(addr); err == nil {
			t.Fatalf("accepted public or ambiguous bind %s", addr)
		}
	}
	for _, addr := range []string{"127.0.0.1:8787", "[::1]:8787"} {
		if err := LoopbackOnly(addr); err != nil {
			t.Fatalf("rejected loopback bind %s: %v", addr, err)
		}
	}
}

package runtime

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yu-min3/kensan-lab/apps/llmops/internal/jobs"
)

var testOwner = jobs.Owner{App: "example", Role: "app", Subject: "example-app"}

func request() Request {
	return Request{Profile: "text", Prompt: "Extract supplied text", Input: json.RawMessage(`{"text":"sample"}`), OutputSchema: json.RawMessage(`{"type":"object"}`), IdempotencyKey: "stable-key"}
}
func secret(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "secret")
	if e := os.WriteFile(path, []byte("sample-test-secret"), 0600); e != nil {
		t.Fatal(e)
	}
	return path
}
func config(t *testing.T, base, token string) Config {
	return Config{Owner: testOwner, TargetID: "test-runtime", BaseURL: base, TokenURL: token, ClientID: "runtime-example-online", ClientSecretFile: secret(t)}
}
func tokenResponse(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"access_token":"test-token","token_type":"Bearer","expires_in":60}`))
}
func jobResponse(w http.ResponseWriter, state string) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"id":"job-1","state":"` + state + `"}`))
}
func TestReplayAndWaitingFailedRemainDistinct(t *testing.T) {
	var creates, tokenCalls atomic.Int32
	var mu sync.Mutex
	var bodies [][]byte
	token := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenCalls.Add(1)
		id, pw, ok := r.BasicAuth()
		if !ok || id != "runtime-example-online" || pw != "sample-test-secret" {
			t.Error("bad service credentials")
		}
		r.ParseForm()
		if r.Form.Get("grant_type") != "client_credentials" {
			t.Error("bad grant")
		}
		tokenResponse(w)
	}))
	defer token.Close()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("missing token")
		}
		if r.URL.Path == "/v1/jobs" {
			creates.Add(1)
			var req json.RawMessage
			json.NewDecoder(r.Body).Decode(&req)
			mu.Lock()
			bodies = append(bodies, req)
			mu.Unlock()
			jobResponse(w, "queued")
			return
		}
		switch r.URL.Path {
		case "/v1/jobs/job-1":
			w.Write([]byte(`{"id":"job-1","state":"waiting","reason":"auth_required"}`))
		case "/v1/jobs/job-1/retry":
			jobResponse(w, "queued")
		case "/v1/jobs/job-1/cancel":
			jobResponse(w, "cancelled")
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()
	c, e := New(config(t, upstream.URL, token.URL))
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 2; i++ {
		j, e := c.Create(context.Background(), testOwner, request())
		if e != nil || j.ID != "job-1" {
			t.Fatalf("Create %v %v", j, e)
		}
	}
	if creates.Load() != 2 || tokenCalls.Load() != 1 || string(bodies[0]) != string(bodies[1]) {
		t.Fatal("replay changed request or repeated token fetch")
	}
	j, e := c.Get(context.Background(), testOwner, "job-1")
	if e != nil || j.State != "waiting" || j.Reason != "auth_required" || j.Model != nil || j.Usage != nil {
		t.Fatalf("waiting %v %v", j, e)
	}
	b, _ := json.Marshal(j)
	if !strings.Contains(string(b), `"model":null`) || !strings.Contains(string(b), `"usage":null`) {
		t.Fatal("unknown fabricated")
	}
	for _, action := range []string{"retry", "cancel"} {
		if _, e = c.Action(context.Background(), testOwner, "job-1", action); e != nil {
			t.Fatal(e)
		}
	}
	if _, e = c.Action(context.Background(), testOwner, "job-1", "rerun-shell"); e == nil {
		t.Fatal("unknown action")
	}
}
func TestOwnerAndInputRejectedBeforeNetwork(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); tokenResponse(w) }))
	defer srv.Close()
	c, e := New(config(t, srv.URL, srv.URL))
	if e != nil {
		t.Fatal(e)
	}
	for _, o := range []jobs.Owner{{App: "other", Role: "app", Subject: testOwner.Subject}, {App: testOwner.App, Role: "eval", Subject: testOwner.Subject}, {App: testOwner.App, Role: "app", Subject: "other"}} {
		if _, e = c.Create(context.Background(), o, request()); !errors.Is(e, ErrOwner) {
			t.Fatal(e)
		}
		if _, e = c.Get(context.Background(), o, "job-1"); !errors.Is(e, ErrOwner) {
			t.Fatal(e)
		}
		if _, e = c.Action(context.Background(), o, "job-1", "cancel"); !errors.Is(e, ErrOwner) {
			t.Fatal(e)
		}
	}
	for _, change := range []func(*Request){func(r *Request) { r.Profile = "shell" }, func(r *Request) { r.Prompt = strings.Repeat("あ", 11000) }, func(r *Request) { r.Input = json.RawMessage(`not json`) }, func(r *Request) { r.OutputSchema = json.RawMessage(`null`) }, func(r *Request) { r.IdempotencyKey = "" }, func(r *Request) { r.Input = json.RawMessage(`"` + strings.Repeat("x", MaxRequestBytes) + `"`) }} {
		r := request()
		change(&r)
		if _, e = c.Create(context.Background(), testOwner, r); e == nil {
			t.Fatal("invalid request accepted")
		}
	}
	if _, e = c.Get(context.Background(), testOwner, "../secret"); e == nil {
		t.Fatal("path accepted")
	}
	if calls.Load() != 0 {
		t.Fatal("rejected call sent to network")
	}
}
func TestEndpointAndPrivateSecretValidation(t *testing.T) {
	for _, raw := range []string{"http://localhost:9000", "http://runtime.example", "https://user:pass@runtime.example", "https://runtime.example/base", "https://runtime.example?token=secret", "https://runtime.example#fragment"} {
		cfg := config(t, raw, "https://identity.example/token")
		if _, e := New(cfg); e == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	cfg := config(t, "https://runtime.example", "http://identity.example/token")
	if _, e := New(cfg); e == nil {
		t.Fatal("plaintext remote token endpoint")
	}
	cfg = config(t, "https://runtime.example", "https://identity.example/token")
	os.Chmod(cfg.ClientSecretFile, 0644)
	if _, e := New(cfg); e == nil {
		t.Fatal("world readable secret")
	}
}
func TestHTTPAndProtocolFaults(t *testing.T) {
	token := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { tokenResponse(w) }))
	defer token.Close()
	for _, tc := range []struct {
		name       string
		status     int
		body, kind string
	}{
		{"quota", 429, "private upstream text", "admission_limited"}, {"auth", 401, "private upstream text", "authentication_rejected"}, {"owner", 404, "private upstream text", "not_found"}, {"conflict", 409, "private upstream text", "conflict"},
		{"bad-state", 200, `{"id":"job-1","state":"imaginary"}`, "invalid_response"}, {"wrong-id", 200, `{"id":"different","state":"queued"}`, "invalid_response"}, {"huge", 200, strings.Repeat("x", MaxResponseBytes+1), "invalid_response"}, {"trailing", 200, `{"id":"job-1","state":"queued"} {}`, "invalid_response"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status); w.Write([]byte(tc.body)) }))
			defer srv.Close()
			c, e := New(config(t, srv.URL, token.URL))
			if e != nil {
				t.Fatal(e)
			}
			_, e = c.Get(context.Background(), testOwner, "job-1")
			var re *Error
			if !errors.As(e, &re) || re.Kind != tc.kind || strings.Contains(e.Error(), "private") {
				t.Fatalf("fault %v", e)
			}
		})
	}
}
func TestTokenRefreshReadsRotatedSecret(t *testing.T) {
	var calls atomic.Int32
	var cfg Config
	token := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		_, pw, _ := r.BasicAuth()
		if n == 2 && pw != "rotated-test-secret" {
			t.Error("secret not rotated")
		}
		tokenResponse(w)
	}))
	defer token.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { jobResponse(w, "queued") }))
	defer srv.Close()
	cfg = config(t, srv.URL, token.URL)
	c, e := New(cfg)
	if e != nil {
		t.Fatal(e)
	}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	c.now = func() time.Time { return now }
	if _, e = c.Get(context.Background(), testOwner, "job-1"); e != nil {
		t.Fatal(e)
	}
	os.WriteFile(cfg.ClientSecretFile, []byte("rotated-test-secret"), 0600)
	now = now.Add(time.Minute)
	if _, e = c.Get(context.Background(), testOwner, "job-1"); e != nil || calls.Load() != 2 {
		t.Fatalf("refresh %d %v", calls.Load(), e)
	}
}
func TestRedirectTimeoutAndCA(t *testing.T) {
	var leaked atomic.Int32
	dest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { leaked.Add(1) }))
	defer dest.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, dest.URL, 307) }))
	defer redirect.Close()
	c, e := New(config(t, redirect.URL, redirect.URL))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = c.Get(context.Background(), testOwner, "job-1"); e == nil || leaked.Load() != 0 {
		t.Fatal("token redirect followed")
	}
	token := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { tokenResponse(w) }))
	defer token.Close()
	c, e = New(config(t, redirect.URL, token.URL))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = c.Get(context.Background(), testOwner, "job-1"); e == nil || leaked.Load() != 0 {
		t.Fatal("runtime redirect followed")
	}
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer slow.Close()
	cfg := config(t, slow.URL, token.URL)
	cfg.Timeout = 20 * time.Millisecond
	c, e = New(cfg)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = c.Get(context.Background(), testOwner, "job-1"); e == nil {
		t.Fatal("timeout ignored")
	}
	tlsSrv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			tokenResponse(w)
		} else {
			jobResponse(w, "queued")
		}
	}))
	defer tlsSrv.Close()
	cfg = config(t, tlsSrv.URL, tlsSrv.URL+"/token")
	c, e = New(cfg)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = c.Get(context.Background(), testOwner, "job-1"); e == nil {
		t.Fatal("untrusted CA accepted")
	}
	cfg.CAFile = filepath.Join(t.TempDir(), "ca.pem")
	os.WriteFile(cfg.CAFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: tlsSrv.Certificate().Raw}), 0600)
	c, e = New(cfg)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = c.Get(context.Background(), testOwner, "job-1"); e != nil {
		t.Fatalf("trusted CA not applied to token and runtime: %v", e)
	}
}

func TestLostCreateResponseReplaysIdenticalKey(t *testing.T) {
	token := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { tokenResponse(w) }))
	defer token.Close()
	var count atomic.Int32
	var first string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var got Request
		if e := json.NewDecoder(r.Body).Decode(&got); e != nil {
			t.Error(e)
		}
		if count.Add(1) == 1 {
			first = got.IdempotencyKey
			conn, _, e := w.(http.Hijacker).Hijack()
			if e != nil {
				t.Error(e)
				return
			}
			conn.Close()
			return
		}
		if got.IdempotencyKey != first {
			t.Error("new key after lost response")
		}
		jobResponse(w, "queued")
	}))
	defer srv.Close()
	c, e := New(config(t, srv.URL, token.URL))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = c.Create(context.Background(), testOwner, request()); e == nil {
		t.Fatal("lost response reported as success")
	}
	j, e := c.Create(context.Background(), testOwner, request())
	if e != nil || j.ID != "job-1" || count.Load() != 2 {
		t.Fatalf("replay %v %v %d", j, e, count.Load())
	}
}

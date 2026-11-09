package api

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yu-min3/kensan-lab/apps/llmops/contracts"
	"github.com/yu-min3/kensan-lab/apps/llmops/internal/identity"
	"github.com/yu-min3/kensan-lab/apps/llmops/internal/jobs"
	"github.com/yu-min3/kensan-lab/apps/llmops/internal/objects"
	"github.com/yu-min3/kensan-lab/apps/llmops/internal/policy"
	runtimeclient "github.com/yu-min3/kensan-lab/apps/llmops/internal/runtime"
)

type jobHub struct {
	rejectStatus int
	mu           sync.Mutex
	byKey        map[string]runtimeclient.Job
	creates      int
	calls        int
	lost         bool
	order        []string
}
type jobGateway struct {
	hub    *jobHub
	owner  jobs.Owner
	client string
}

func (g *jobGateway) ClientID() string { return g.client }
func (g *jobGateway) TargetID() string { return "fixture-runtime" }
func (g *jobGateway) Create(_ context.Context, o jobs.Owner, r runtimeclient.Request) (runtimeclient.Job, error) {
	if o != g.owner {
		return runtimeclient.Job{}, errorsOwner()
	}
	g.hub.mu.Lock()
	defer g.hub.mu.Unlock()
	g.hub.calls++
	if g.hub.rejectStatus != 0 {
		return runtimeclient.Job{}, &runtimeclient.Error{Status: g.hub.rejectStatus, Kind: "fixture_rejected"}
	}
	key := o.App + ":" + o.Role + ":" + o.Subject + ":" + r.IdempotencyKey
	if j, ok := g.hub.byKey[key]; ok {
		return j, nil
	}
	g.hub.creates++
	g.hub.order = append(g.hub.order, r.Prompt)
	state := "succeeded"
	if strings.HasPrefix(r.Prompt, "wait") {
		state = "waiting"
	}
	if strings.HasPrefix(r.Prompt, "fail") {
		state = "failed"
	}
	j := runtimeclient.Job{ID: fmt.Sprintf("runtime-%d", g.hub.creates), State: state, Output: json.RawMessage(`{"result":"fixture"}`)}
	g.hub.byKey[key] = j
	if g.hub.lost {
		g.hub.lost = false
		return runtimeclient.Job{}, &runtimeclient.Error{Kind: "transport_unavailable"}
	}
	return j, nil
}
func errorsOwner() error { return fmt.Errorf("owner mismatch") }
func (g *jobGateway) Get(_ context.Context, o jobs.Owner, id string) (runtimeclient.Job, error) {
	g.hub.mu.Lock()
	defer g.hub.mu.Unlock()
	if o != g.owner {
		return runtimeclient.Job{}, errorsOwner()
	}
	for _, j := range g.hub.byKey {
		if j.ID == id {
			return j, nil
		}
	}
	return runtimeclient.Job{}, &runtimeclient.Error{Status: 404}
}
func (g *jobGateway) Action(ctx context.Context, o jobs.Owner, id, action string) (runtimeclient.Job, error) {
	g.hub.mu.Lock()
	defer g.hub.mu.Unlock()
	if o != g.owner {
		return runtimeclient.Job{}, errorsOwner()
	}
	for key, j := range g.hub.byKey {
		if j.ID == id {
			if action == "cancel" {
				j.State = "cancelled"
			} else if j.State == "waiting" {
				j.State = "queued"
			} else {
				return j, &runtimeclient.Error{Status: 409}
			}
			g.hub.byKey[key] = j
			return j, nil
		}
	}
	return runtimeclient.Job{}, &runtimeclient.Error{Status: 404}
}

type jobFixture struct {
	api     *JobAPI
	handler http.Handler
	store   *jobs.Store
	dir     string
	hub     *jobHub
	tokens  map[string]string
	objects map[string]*objects.Store
	auth    Verifier
}

func newJobFixture(t *testing.T) *jobFixture {
	t.Helper()
	key, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		t.Fatal(e)
	}
	issuer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]string{"kty": "RSA", "kid": "independent-key", "alg": "RS256", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())}}})
	}))
	t.Cleanup(issuer.Close)
	mapping := `{"version":"v1","clients":{"example-app":{"app_id":"example","role":"app","audience":"llmops-platform","runtime_client":"runtime-example-online"},"example-eval":{"app_id":"example","role":"eval","audience":"llmops-platform","runtime_client":"runtime-example-eval"},"other-app":{"app_id":"other","role":"app","audience":"llmops-platform","runtime_client":"runtime-other-online"}}}`
	auth, e := identity.New(identity.Config{Issuer: issuer.URL, Audience: "llmops-platform", JWKSURL: issuer.URL, MappingJSON: []byte(mapping), HTTPClient: issuer.Client()})
	if e != nil {
		t.Fatal(e)
	}
	tokens := map[string]string{}
	for _, id := range []string{"example-app", "example-eval", "other-app"} {
		header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","kid":"independent-key"}`))
		payload, _ := json.Marshal(map[string]any{"iss": issuer.URL, "aud": "llmops-platform", "azp": id, "sub": id + "-subject", "exp": time.Now().Add(time.Hour).Unix()})
		data := header + "." + base64.RawURLEncoding.EncodeToString(payload)
		digest := sha256.Sum256([]byte(data))
		sig, e := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
		if e != nil {
			t.Fatal(e)
		}
		tokens[id] = data + "." + base64.RawURLEncoding.EncodeToString(sig)
	}
	f := &jobFixture{dir: filepath.Join(t.TempDir(), "state"), hub: &jobHub{byKey: map[string]runtimeclient.Job{}}, tokens: tokens, objects: map[string]*objects.Store{}, auth: auth}
	for _, app := range []string{"example", "other"} {
		b := &testBackend{app: app, items: map[string]objects.Blob{}}
		f.objects[app], e = objects.NewStore(app, b)
		if e != nil {
			t.Fatal(e)
		}
	}
	f.open(t)
	t.Cleanup(func() { f.store.Close() })
	return f
}
func (f *jobFixture) open(t *testing.T) {
	var e error
	f.store, e = jobs.Open(f.dir)
	if e != nil {
		t.Fatal(e)
	}
	resolver := &StorageResolver{Stores: f.objects, Jobs: f.store}
	engine, e := policy.New(policy.Config{Resolver: resolver, CursorKey: bytes.Repeat([]byte("c"), 32)})
	if e != nil {
		t.Fatal(e)
	}
	capture, e := NewCaptureFeedback(CaptureFeedbackConfig{Auth: f.auth, Policy: engine, Stores: f.objects})
	if e != nil {
		t.Fatal(e)
	}
	routes := map[string]RuntimeRoute{}
	for _, r := range []struct{ client, app, role string }{{"runtime-example-online", "example", "app"}, {"runtime-example-eval", "example", "eval"}, {"runtime-other-online", "other", "app"}} {
		r := r
		routes[r.client] = RuntimeRoute{AppID: r.app, Role: r.role, TargetID: "fixture-runtime", Factory: func(o jobs.Owner) (RuntimeGateway, error) { return &jobGateway{f.hub, o, r.client}, nil }}
	}
	f.api, e = NewJobs(JobConfig{Authenticate: capture.Authenticated, Policy: engine, Store: f.store, Objects: f.objects, Routes: routes, DailyLimits: map[string]map[string]int{"example": {"online": 30, "eval": 10}, "other": {"online": 10}}})
	if e != nil {
		t.Fatal(e)
	}
	resolver.Next = f.api
	mux := http.NewServeMux()
	capture.Register(mux)
	f.api.Register(mux)
	f.handler = mux
}
func (f *jobFixture) request(t *testing.T, client, method, path, key string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var raw []byte
	if body != nil {
		raw, _ = json.Marshal(body)
	}
	r := httptest.NewRequest(method, path, bytes.NewReader(raw))
	r.Header.Set("Authorization", "Bearer "+f.tokens[client])
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Idempotency-Key", key)
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	return w
}
func jobBody(kind, prompt string) map[string]any {
	return map[string]any{"profile": "text", "kind": kind, "prompt": prompt, "input": map[string]any{"text": "captured"}, "output_schema": map[string]any{"type": "object"}}
}
func jobID(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	if w.Code != 202 {
		t.Fatalf("create %d %s", w.Code, w.Body)
	}
	var body map[string]any
	json.Unmarshal(w.Body.Bytes(), &body)
	if e := contracts.Validate("job", body); e != nil {
		t.Fatal(e)
	}
	return body["id"].(string)
}
func TestJobsHTTPIdentityScopeAndArtifacts(t *testing.T) {
	f := newJobFixture(t)
	id := jobID(t, f.request(t, "example-app", "POST", "/v1/jobs", "key-one", jobBody("online", "online-one")))
	if e := f.api.DispatchOnce(context.Background()); e != nil {
		t.Fatal(e)
	}
	got := f.request(t, "example-app", "GET", "/v1/jobs/"+id, "", nil)
	if got.Code != 200 || !strings.Contains(got.Body.String(), "fixture") {
		t.Fatal("own result", got.Code, got.Body.String())
	}
	for _, client := range []string{"other-app", "example-eval"} {
		for _, suffix := range []string{"", "/cancel", "/retry"} {
			method := "POST"
			if suffix == "" {
				method = "GET"
			}
			if w := f.request(t, client, method, "/v1/jobs/"+id+suffix, "", map[string]any{}); w.Code != 404 {
				t.Fatalf("foreign %s %s status %d", client, suffix, w.Code)
			}
		}
		if w := f.request(t, client, "GET", "/v1/artifacts/job_"+id, "", nil); w.Code != 404 {
			t.Fatal("foreign artifact", w.Code)
		}
		if w := f.request(t, client, "GET", "/v1/jobs", "", nil); w.Code != 200 || strings.Contains(w.Body.String(), id) {
			t.Fatal("listing leaked", w.Code, w.Body.String())
		}
	}
	w := f.request(t, "example-app", "GET", "/v1/artifacts/job_"+id, "", nil)
	if w.Code != 200 || w.Header().Get("Content-Disposition") != "attachment" || !strings.Contains(w.Header().Get("Content-Security-Policy"), "sandbox") {
		t.Fatal("artifact is not a safe authorized download", w.Code)
	}
	for _, override := range []string{"owner", "runtime_client", "credentials"} {
		body := jobBody("online", "x")
		body[override] = "admin"
		if w := f.request(t, "example-app", "POST", "/v1/jobs", "bad", body); w.Code != 400 {
			t.Fatal("override accepted", override, w.Code)
		}
	}
	body := jobBody("online", "x")
	body["app_id"] = "other"
	if w := f.request(t, "example-app", "POST", "/v1/jobs", "bad", body); w.Code != 403 {
		t.Fatal("app override", w.Code)
	}
	if w := f.request(t, "example-eval", "POST", "/v1/jobs", "bad", jobBody("online", "x")); w.Code != 403 {
		t.Fatal("eval invoked online", w.Code)
	}
	if w := f.request(t, "example-app", "POST", "/v1/jobs", "", jobBody("online", "x")); w.Code != 400 {
		t.Fatal("missing idempotency", w.Code)
	}
	if w := f.request(t, "unknown", "GET", "/v1/jobs", "", nil); w.Code != 401 {
		t.Fatal("invalid bearer accepted", w.Code)
	}
}
func TestDispatchPriorityResponseLossAndRestart(t *testing.T) {
	f := newJobFixture(t)
	evalID := jobID(t, f.request(t, "example-eval", "POST", "/v1/jobs", "eval-key", jobBody("eval", "eval-first")))
	onlineID := jobID(t, f.request(t, "example-app", "POST", "/v1/jobs", "online-key", jobBody("online", "online-second")))
	f.hub.lost = true
	if e := f.api.DispatchOnce(context.Background()); e == nil {
		t.Fatal("lost response not simulated")
	}
	if f.hub.order[0] != "online-second" {
		t.Fatal("online not prioritized")
	}
	f.store.Close()
	f.open(t)
	if e := f.api.DispatchOnce(context.Background()); e != nil {
		t.Fatal(e)
	}
	if f.hub.creates != 1 || f.hub.calls != 2 {
		t.Fatal("response-loss replay duplicated generation", f.hub.creates, f.hub.calls)
	}
	if e := f.api.DispatchOnce(context.Background()); e != nil {
		t.Fatal(e)
	}
	if f.hub.creates != 2 {
		t.Fatal("eval not dispatched")
	}
	for _, tc := range []struct{ client, id string }{{"example-app", onlineID}, {"example-eval", evalID}} {
		w := f.request(t, tc.client, "GET", "/v1/jobs/"+tc.id, "", nil)
		if w.Code != 200 || !strings.Contains(w.Body.String(), "succeeded") {
			t.Fatal("restart correlation lost", w.Code, w.Body.String())
		}
	}
	replay := jobID(t, f.request(t, "example-app", "POST", "/v1/jobs", "online-key", jobBody("online", "online-second")))
	if replay != onlineID {
		t.Fatal("upper replay changed job")
	}
	if w := f.request(t, "example-app", "POST", "/v1/jobs", "online-key", jobBody("online", "changed")); w.Code != 409 {
		t.Fatal("content conflict accepted", w.Code)
	}
}
func TestActionsBudgetAndCursor(t *testing.T) {
	f := newJobFixture(t)
	canceled := jobID(t, f.request(t, "example-app", "POST", "/v1/jobs", "cancel-key", jobBody("online", "x")))
	if w := f.request(t, "example-app", "POST", "/v1/jobs/"+canceled+"/cancel", "", map[string]any{}); w.Code != 200 {
		t.Fatal(w.Code)
	}
	waiting := jobID(t, f.request(t, "example-app", "POST", "/v1/jobs", "wait-key", jobBody("online", "wait-one")))
	f.api.DispatchOnce(context.Background())
	if w := f.request(t, "example-app", "POST", "/v1/jobs/"+waiting+"/retry", "", map[string]any{}); w.Code != 200 {
		t.Fatal("waiting resume", w.Code, w.Body.String())
	}
	failed := jobID(t, f.request(t, "example-app", "POST", "/v1/jobs", "failed-key", jobBody("online", "fail-one")))
	f.api.DispatchOnce(context.Background())
	next := jobID(t, f.request(t, "example-app", "POST", "/v1/jobs/"+failed+"/retry", "rerun-key", map[string]any{}))
	if next == failed {
		t.Fatal("failed rerun kept same attempt")
	}
	replay := jobID(t, f.request(t, "example-app", "POST", "/v1/jobs/"+failed+"/retry", "rerun-key", map[string]any{}))
	if replay != next {
		t.Fatal("rerun replay duplicated attempt")
	}
	f.api.limits["example"]["online"] = 0
	if w := f.request(t, "example-app", "POST", "/v1/jobs", "budget", jobBody("online", "x")); w.Code != 429 {
		t.Fatal("budget not enforced", w.Code)
	}
	listing := f.request(t, "example-app", "GET", "/v1/jobs?limit=1", "", nil)
	var page struct {
		Next string `json:"next_cursor"`
	}
	json.Unmarshal(listing.Body.Bytes(), &page)
	if page.Next == "" {
		t.Fatal("no pagination cursor")
	}
	if w := f.request(t, "other-app", "GET", "/v1/jobs?cursor="+page.Next, "", nil); w.Code != 404 {
		t.Fatal("foreign cursor accepted", w.Code)
	}
}

func TestDefinitiveRejectionsDoNotGenerateOrAutomaticallyRetry(t *testing.T) {
	for _, tc := range []struct {
		status        int
		state, reason string
	}{{400, "failed", "runtime_request_rejected"}, {401, "waiting", "runtime_auth_required"}, {403, "waiting", "runtime_auth_required"}, {429, "waiting", "runtime_quota_wait"}} {
		t.Run(fmt.Sprint(tc.status), func(t *testing.T) {
			f := newJobFixture(t)
			f.hub.rejectStatus = tc.status
			id := jobID(t, f.request(t, "example-app", "POST", "/v1/jobs", "rejected", jobBody("online", "x")))
			if e := f.api.DispatchOnce(context.Background()); e == nil {
				t.Fatal("rejection missing")
			}
			for i := 0; i < 3; i++ {
				f.api.DispatchOnce(context.Background())
			}
			if f.hub.calls != 1 || f.hub.creates != 0 {
				t.Fatal("definitive rejection was replayed automatically")
			}
			w := f.request(t, "example-app", "GET", "/v1/jobs/"+id, "", nil)
			var result map[string]any
			json.Unmarshal(w.Body.Bytes(), &result)
			if w.Code != 200 || result["state"] != tc.state || result["reason"] != tc.reason {
				t.Fatal("rejection not visible", w.Code, w.Body.String())
			}
			if tc.state == "waiting" {
				f.hub.rejectStatus = 0
				resume := f.request(t, "example-app", "POST", "/v1/jobs/"+id+"/retry", "", map[string]any{})
				if resume.Code != 200 {
					t.Fatal("not resumable", resume.Code)
				}
				if e := f.api.DispatchOnce(context.Background()); e != nil {
					t.Fatal(e)
				}
				if f.hub.creates != 1 {
					t.Fatal("safe resume failed")
				}
			}
		})
	}
}
func TestHTTPByteLimitsAndSafeUncertainState(t *testing.T) {
	f := newJobFixture(t)
	for _, body := range []map[string]any{jobBody("online", strings.Repeat("あ", 12000)), func() map[string]any {
		b := jobBody("online", "x")
		b["output_schema"] = map[string]any{"description": strings.Repeat("x", 33000)}
		return b
	}(), jobBody("online", strings.Repeat("x", 600000))} {
		if w := f.request(t, "example-app", "POST", "/v1/jobs", "oversized", body); w.Code != 413 && w.Code != 400 {
			t.Fatal("oversized input accepted", w.Code)
		}
	}
	f.hub.rejectStatus = 503
	id := jobID(t, f.request(t, "example-app", "POST", "/v1/jobs", "uncertain", jobBody("online", "x")))
	f.api.DispatchOnce(context.Background())
	w := f.request(t, "example-app", "GET", "/v1/jobs/"+id, "", nil)
	var result map[string]any
	json.Unmarshal(w.Body.Bytes(), &result)
	if result["state"] != "waiting" || result["reason"] != "dispatch_reconciliation_required" {
		t.Fatal("uncertain dispatch hidden", w.Body.String())
	}
	r, e := f.store.Get(jobs.Owner{App: "example", Role: "app", Subject: "example-app-subject"}, id)
	if e != nil || r.Phase != "uncertain" {
		t.Fatal("uncertain create became definitive rejection", r, e)
	}
}

func TestResultStorageBoundsAndImmutableTerminalResult(t *testing.T) {
	f := newJobFixture(t)
	id := jobID(t, f.request(t, "example-app", "POST", "/v1/jobs", "result-key", jobBody("online", "x")))
	f.api.DispatchOnce(context.Background())
	var upstreamKey string
	for key, j := range f.hub.byKey {
		upstreamKey = key
		j.Output = json.RawMessage(`{"text":"` + strings.Repeat("x", 750<<10) + `"}`)
		f.hub.byKey[key] = j
	}
	if w := f.request(t, "example-app", "GET", "/v1/jobs/"+id, "", nil); w.Code != 200 {
		t.Fatal("valid large runtime result not stored", w.Code)
	}
	j := f.hub.byKey[upstreamKey]
	j.Output = json.RawMessage(`{"changed":true}`)
	f.hub.byKey[upstreamKey] = j
	w := f.request(t, "example-app", "GET", "/v1/jobs/"+id, "", nil)
	if w.Code != 200 || strings.Contains(w.Body.String(), "changed") {
		t.Fatal("immutable terminal result changed", w.Code)
	}
	second := jobID(t, f.request(t, "example-app", "POST", "/v1/jobs", "too-large-result", jobBody("online", "y")))
	f.api.DispatchOnce(context.Background())
	for key, j := range f.hub.byKey {
		if j.ID != "runtime-1" {
			j.Output = json.RawMessage(`{"text":"` + strings.Repeat("x", 3<<20) + `"}`)
			f.hub.byKey[key] = j
		}
	}
	if w := f.request(t, "example-app", "GET", "/v1/jobs/"+second, "", nil); w.Code != 503 {
		t.Fatal("oversized upstream result silently accepted", w.Code)
	}
}

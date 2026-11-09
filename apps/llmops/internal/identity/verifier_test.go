package identity

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

const mapping = `{"version":"policy-v1","clients":{"konro-app":{"app_id":"konro","role":"app","audience":"llmops-platform","runtime_client":"runtime-konro-online"}}}`

// The producer uses crypto/rsa and JSON directly, independently of the JWT
// library under test. Expected identities are literal assertions, not derived
// from the verifier's mapping or response.
func sign(t *testing.T, key *rsa.PrivateKey, kid string, claims map[string]any, alg string) string {
	t.Helper()
	header, _ := json.Marshal(map[string]any{"alg": alg, "kid": kid, "typ": "JWT"})
	payload, _ := json.Marshal(claims)
	data := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	digest := sha256.Sum256([]byte(data))
	sig, e := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if e != nil {
		t.Fatal(e)
	}
	return data + "." + base64.RawURLEncoding.EncodeToString(sig)
}
func jwk(key *rsa.PrivateKey, kid string) map[string]string {
	return map[string]string{"kty": "RSA", "kid": kid, "alg": "RS256", "use": "sig", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())}
}

type fixture struct {
	t      *testing.T
	now    time.Time
	key    *rsa.PrivateKey
	mu     sync.Mutex
	keys   any
	status int
	calls  int
	server *httptest.Server
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	key, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		t.Fatal(e)
	}
	f := &fixture{t: t, now: time.Unix(1800000000, 0), key: key, status: 200, keys: []any{jwk(key, "key-a")}}
	f.server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.calls++
		w.WriteHeader(f.status)
		json.NewEncoder(w).Encode(map[string]any{"keys": f.keys})
	}))
	t.Cleanup(f.server.Close)
	return f
}
func (f *fixture) config() Config {
	return Config{Issuer: f.server.URL + "/realm", JWKSURL: f.server.URL + "/jwks", Audience: "llmops-platform", MappingJSON: []byte(mapping), HTTPClient: f.server.Client(), Now: func() time.Time { return f.now }, CacheTTL: time.Minute}
}
func (f *fixture) claims() map[string]any {
	return map[string]any{"iss": f.server.URL + "/realm", "aud": []string{"unrelated", "llmops-platform"}, "exp": f.now.Add(10 * time.Minute).Unix(), "nbf": f.now.Add(-time.Minute).Unix(), "sub": "service-account-uuid", "azp": "konro-app"}
}
func TestSignatureAndClaims(t *testing.T) {
	f := newFixture(t)
	for _, tc := range []struct {
		name   string
		mutate func(map[string]any)
		alg    string
		valid  bool
	}{
		{"valid audience array", func(map[string]any) {}, "RS256", true},
		{"audience string", func(c map[string]any) { c["aud"] = "llmops-platform" }, "RS256", true},
		{"issuer", func(c map[string]any) { c["iss"] = "https://other.example/realm" }, "RS256", false},
		{"audience", func(c map[string]any) { c["aud"] = "polaris" }, "RS256", false},
		{"expired", func(c map[string]any) { c["exp"] = f.now.Unix() }, "RS256", false},
		{"missing exp", func(c map[string]any) { delete(c, "exp") }, "RS256", false},
		{"future nbf", func(c map[string]any) { c["nbf"] = f.now.Add(time.Second).Unix() }, "RS256", false},
		{"missing azp", func(c map[string]any) { delete(c, "azp") }, "RS256", false},
		{"unregistered client", func(c map[string]any) { c["azp"] = "evil-client" }, "RS256", false},
		{"client mismatch", func(c map[string]any) { c["client_id"] = "evil-client" }, "RS256", false},
		{"empty subject", func(c map[string]any) { c["sub"] = "" }, "RS256", false},
		{"algorithm confusion", func(map[string]any) {}, "HS256", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v, e := New(f.config())
			if e != nil {
				t.Fatal(e)
			}
			c := f.claims()
			tc.mutate(c)
			p, e := v.Verify(context.Background(), sign(t, f.key, "key-a", c, tc.alg))
			if (e == nil) != tc.valid {
				t.Fatalf("valid=%v principal=%+v error=%v", tc.valid, p, e)
			}
			if tc.valid && (p.AppID != "konro" || p.Role != "app" || p.ClientID != "konro-app" || p.PolicyVersion != "policy-v1") {
				t.Fatalf("unexpected principal %+v", p)
			}
		})
	}
	wrong, _ := rsa.GenerateKey(rand.Reader, 2048)
	v, _ := New(f.config())
	if _, e := v.Verify(context.Background(), sign(t, wrong, "key-a", f.claims(), "RS256")); e == nil {
		t.Fatal("wrong signing key accepted")
	}
}
func TestKeyRotationCacheAndFailure(t *testing.T) {
	f := newFixture(t)
	v, _ := New(f.config())
	ctx := context.Background()
	first := sign(t, f.key, "key-a", f.claims(), "RS256")
	if _, e := v.Verify(ctx, first); e != nil {
		t.Fatal(e)
	}
	if _, e := v.Verify(ctx, first); e != nil || f.calls != 1 {
		t.Fatalf("cache not used calls=%d err=%v", f.calls, e)
	}
	second, _ := rsa.GenerateKey(rand.Reader, 2048)
	f.keys = []any{jwk(second, "key-b")}
	f.now = f.now.Add(time.Second)
	token := sign(t, second, "key-b", f.claims(), "RS256")
	if _, e := v.Verify(ctx, token); e != nil || f.calls != 2 {
		t.Fatalf("rotation failed %d %v", f.calls, e)
	}
	if _, e := v.Verify(ctx, first); e == nil || f.calls != 2 {
		t.Fatal("removed key or refresh cooldown bypassed")
	}
	f.now = f.now.Add(time.Second)
	unknown := sign(t, second, "missing", f.claims(), "RS256")
	for i := 0; i < 5; i++ {
		if _, e := v.Verify(ctx, unknown); e == nil {
			t.Fatal("unknown key accepted")
		}
	}
	if f.calls != 3 {
		t.Fatalf("unknown kid repeatedly fetched: %d", f.calls)
	}
	f.status = 503
	f.now = f.now.Add(time.Minute)
	if _, e := v.Verify(ctx, token); e == nil {
		t.Fatal("stale keys used after expiry failure")
	}
	if _, e := v.Verify(ctx, token); e == nil {
		t.Fatal("failed refresh cache accepted")
	}
	f.status = 200
	f.now = f.now.Add(time.Second)
	if _, e := v.Verify(ctx, token); e != nil {
		t.Fatal("recovery failed", e)
	}
}
func TestEndpointAndMappingConfiguration(t *testing.T) {
	f := newFixture(t)
	for _, endpoint := range []string{"http://evil.example/jwks", "http://localhost/jwks", "https://user:secret@example.com/jwks", "https://example.com/jwks?token=secret"} {
		c := f.config()
		c.JWKSURL = endpoint
		c.AllowLoopbackHTTP = true
		if _, e := New(c); e == nil {
			t.Fatal("unsafe endpoint accepted", endpoint)
		}
	}
	c := f.config()
	c.MappingJSON = []byte(strings.Replace(mapping, "llmops-platform", "polaris", 1))
	if _, e := New(c); e == nil {
		t.Fatal("mismatched mapping audience accepted")
	}
	c = f.config()
	c.MappingJSON = []byte(`{"version":"v1","clients":{},"admin":true}`)
	if _, e := New(c); e == nil {
		t.Fatal("unknown mapping field accepted")
	}
	c = f.config()
	c.JWKSURL = "http://127.0.0.1:12345/jwks"
	if _, e := New(c); e == nil {
		t.Fatal("HTTP unexpectedly enabled")
	}
	c.AllowLoopbackHTTP = true
	if _, e := New(c); e != nil {
		t.Fatal("explicit loopback dev rejected", e)
	}
}
func TestMalformedAndRedirectedJWKSFailClosed(t *testing.T) {
	f := newFixture(t)
	for _, keys := range []any{[]any{jwk(f.key, "key-a"), jwk(f.key, "key-a")}, []any{map[string]string{"kty": "RSA", "kid": "key-a", "n": "AQ", "e": "AQAB"}}, []any{}} {
		f.keys = keys
		v, _ := New(f.config())
		if _, e := v.Verify(context.Background(), sign(t, f.key, "key-a", f.claims(), "RS256")); e == nil {
			t.Fatal("bad JWKS accepted")
		}
	}
	redirect := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, f.server.URL+"/jwks", 302) }))
	defer redirect.Close()
	c := f.config()
	c.JWKSURL = redirect.URL
	c.HTTPClient = redirect.Client()
	v, _ := New(c)
	if _, e := v.Verify(context.Background(), sign(t, f.key, "key-a", f.claims(), "RS256")); e == nil {
		t.Fatal("redirect followed")
	}
}

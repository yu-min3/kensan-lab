// Package identity authenticates service clients; resource authorization belongs
// to policy. JWTs and provider credentials are never logged by this package.
package identity

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/yu-min3/kensan-lab/apps/llmops/contracts"
)

var ErrUnauthorized = errors.New("authentication failed")

type Principal struct {
	ClientID      string `json:"client_id"`
	AppID         string `json:"app_id"`
	Role          string `json:"role"`
	Subject       string `json:"subject"`
	RuntimeClient string `json:"runtime_client,omitempty"`
	PolicyVersion string `json:"policy_version"`
}
type Client struct {
	AppID         string `json:"app_id"`
	Role          string `json:"role"`
	Audience      string `json:"audience"`
	RuntimeClient string `json:"runtime_client,omitempty"`
}
type Mapping struct {
	Version string            `json:"version"`
	Clients map[string]Client `json:"clients"`
}
type Config struct {
	Issuer      string
	Audience    string
	JWKSURL     string
	MappingJSON []byte
	CacheTTL    time.Duration
	// RefreshInterval limits refresh attempts caused by unknown key IDs.
	RefreshInterval time.Duration
	HTTPClient      *http.Client
	Now             func() time.Time
	// Development only. HTTP is accepted only for literal loopback addresses.
	AllowLoopbackHTTP bool
}
type Verifier struct {
	cfg         Config
	mapping     Mapping
	client      *http.Client
	mu          sync.Mutex
	keys        map[string]*rsa.PublicKey
	fetchedAt   time.Time
	attemptedAt time.Time
}

func safeURL(raw string, allowHTTP bool) bool {
	u, e := url.Parse(raw)
	if e != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	if u.Scheme == "https" {
		return true
	}
	ip := net.ParseIP(u.Hostname())
	return allowHTTP && u.Scheme == "http" && ip != nil && ip.IsLoopback()
}
func New(c Config) (*Verifier, error) {
	if c.Issuer == "" || c.Audience == "" || !safeURL(c.Issuer, c.AllowLoopbackHTTP) || !safeURL(c.JWKSURL, c.AllowLoopbackHTTP) {
		return nil, errors.New("invalid identity endpoint configuration")
	}
	var decoded any
	if json.Unmarshal(c.MappingJSON, &decoded) != nil || contracts.Validate("client-mapping", decoded) != nil {
		return nil, errors.New("invalid client mapping")
	}
	var m Mapping
	if json.Unmarshal(c.MappingJSON, &m) != nil || len(m.Clients) == 0 {
		return nil, errors.New("empty client mapping")
	}
	for _, client := range m.Clients {
		if client.Audience != c.Audience {
			return nil, errors.New("client mapping audience mismatch")
		}
	}
	if c.CacheTTL == 0 {
		c.CacheTTL = 5 * time.Minute
	}
	if c.RefreshInterval == 0 {
		c.RefreshInterval = time.Second
	}
	if c.CacheTTL < time.Second || c.CacheTTL > time.Hour || c.RefreshInterval < 0 || c.RefreshInterval > c.CacheTTL {
		return nil, errors.New("invalid JWKS cache timing")
	}
	if c.Now == nil {
		c.Now = time.Now
	}
	hc := http.Client{Timeout: 5 * time.Second}
	if c.HTTPClient != nil {
		hc = *c.HTTPClient
		if hc.Timeout == 0 {
			hc.Timeout = 5 * time.Second
		}
	}
	hc.CheckRedirect = func(*http.Request, []*http.Request) error { return errors.New("JWKS redirect refused") }
	return &Verifier{cfg: c, mapping: m, client: &hc, keys: map[string]*rsa.PublicKey{}}, nil
}

type claims struct {
	jwt.RegisteredClaims
	AuthorizedParty string `json:"azp"`
	ClientID        string `json:"client_id,omitempty"`
}

func (v *Verifier) Verify(ctx context.Context, raw string) (Principal, error) {
	if len(raw) == 0 || len(raw) > 16<<10 {
		return Principal{}, ErrUnauthorized
	}
	c := claims{}
	token, e := jwt.ParseWithClaims(raw, &c, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodRS256 {
			return nil, ErrUnauthorized
		}
		kid, ok := t.Header["kid"].(string)
		if !ok || kid == "" || len(kid) > 256 {
			return nil, ErrUnauthorized
		}
		// jku/x5u are never used: the JWKS endpoint is operator configuration.
		return v.key(ctx, kid)
	}, jwt.WithValidMethods([]string{"RS256"}), jwt.WithIssuer(v.cfg.Issuer), jwt.WithAudience(v.cfg.Audience), jwt.WithExpirationRequired(), jwt.WithTimeFunc(v.cfg.Now), jwt.WithStrictDecoding())
	if e != nil || !token.Valid || c.AuthorizedParty == "" || c.Subject == "" || (c.ClientID != "" && c.ClientID != c.AuthorizedParty) {
		return Principal{}, ErrUnauthorized
	}
	registered, ok := v.mapping.Clients[c.AuthorizedParty]
	if !ok {
		return Principal{}, ErrUnauthorized
	}
	return Principal{c.AuthorizedParty, registered.AppID, registered.Role, c.Subject, registered.RuntimeClient, v.mapping.Version}, nil
}
func (v *Verifier) key(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	now := v.cfg.Now()
	key, found := v.keys[kid]
	if found && now.Before(v.fetchedAt.Add(v.cfg.CacheTTL)) {
		return key, nil
	}
	if !v.attemptedAt.IsZero() && now.Before(v.attemptedAt.Add(v.cfg.RefreshInterval)) {
		return nil, ErrUnauthorized
	}
	v.attemptedAt = now
	keys, e := v.fetch(ctx)
	if e != nil {
		v.keys = map[string]*rsa.PublicKey{}
		v.fetchedAt = time.Time{}
		return nil, ErrUnauthorized
	}
	v.keys = keys
	v.fetchedAt = now
	key, found = v.keys[kid]
	if !found {
		return nil, ErrUnauthorized
	}
	return key, nil
}
func (v *Verifier) fetch(ctx context.Context) (map[string]*rsa.PublicKey, error) {
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, v.cfg.JWKSURL, nil)
	if e != nil {
		return nil, e
	}
	res, e := v.client.Do(req)
	if e != nil {
		return nil, e
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, ErrUnauthorized
	}
	raw, e := io.ReadAll(io.LimitReader(res.Body, (1<<20)+1))
	if e != nil || len(raw) > 1<<20 {
		return nil, ErrUnauthorized
	}
	var set struct {
		Keys []struct {
			Kty string `json:"kty"`
			Kid string `json:"kid"`
			Alg string `json:"alg"`
			Use string `json:"use"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if json.Unmarshal(raw, &set) != nil || len(set.Keys) > 100 {
		return nil, ErrUnauthorized
	}
	keys := map[string]*rsa.PublicKey{}
	for _, j := range set.Keys {
		if j.Kty != "RSA" || (j.Alg != "" && j.Alg != "RS256") || (j.Use != "" && j.Use != "sig") {
			continue
		}
		if j.Kid == "" || len(j.Kid) > 256 {
			return nil, ErrUnauthorized
		}
		if _, ok := keys[j.Kid]; ok {
			return nil, ErrUnauthorized
		}
		n, e1 := base64.RawURLEncoding.DecodeString(j.N)
		eb, e2 := base64.RawURLEncoding.DecodeString(j.E)
		modulus := new(big.Int).SetBytes(n)
		exponent := new(big.Int).SetBytes(eb)
		if e1 != nil || e2 != nil || modulus.BitLen() < 2048 || modulus.BitLen() > 8192 || !exponent.IsInt64() {
			return nil, ErrUnauthorized
		}
		ei := exponent.Int64()
		if ei < 3 || ei > 2147483647 || ei%2 == 0 {
			return nil, ErrUnauthorized
		}
		keys[j.Kid] = &rsa.PublicKey{N: modulus, E: int(ei)}
	}
	if len(keys) == 0 {
		return nil, ErrUnauthorized
	}
	return keys, nil
}

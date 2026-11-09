// Package runtime implements the agent-runtime HTTP boundary without importing
// runtime internals or model-provider credentials.
package runtime

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/yu-min3/kensan-lab/apps/llmops/internal/jobs"
)

const MaxRequestBytes = 512 << 10
const MaxResponseBytes = 1 << 20

type Request struct {
	Profile        string          `json:"profile"`
	Prompt         string          `json:"prompt"`
	Input          json.RawMessage `json:"input"`
	OutputSchema   json.RawMessage `json:"output_schema"`
	IdempotencyKey string          `json:"idempotency_key"`
}
type Usage struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
}
type Job struct {
	ID        string          `json:"id"`
	State     string          `json:"state"`
	Reason    string          `json:"reason,omitempty"`
	Output    json.RawMessage `json:"output,omitempty"`
	CreatedAt string          `json:"created_at,omitempty"`
	UpdatedAt string          `json:"updated_at,omitempty"`
	Model     *string         `json:"model"`
	Usage     *Usage          `json:"usage"`
}
type Config struct {
	Owner            jobs.Owner
	TargetID         string
	BaseURL          string
	TokenURL         string
	ClientID         string
	ClientSecretFile string
	CAFile           string
	Timeout          time.Duration
}

// Error deliberately excludes upstream response bodies, tokens, and secrets.
// Status is zero for transport, authentication, or protocol errors.
type Error struct {
	Kind   string
	Status int
}

func (e *Error) Error() string { return "runtime: " + e.Kind }

var ErrOwner = errors.New("runtime owner mismatch")

type Client struct {
	owner                                        jobs.Owner
	target, clientID, secretFile, base, tokenURL string
	http                                         *http.Client
	tokenMu                                      sync.Mutex
	accessToken                                  string
	refreshAt                                    time.Time
	now                                          func() time.Time
}

func endpoint(raw string, base bool) (string, error) {
	u, e := url.Parse(raw)
	if e != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (base && strings.Trim(u.Path, "/") != "") {
		return "", errors.New("invalid runtime endpoint")
	}
	ip := net.ParseIP(u.Hostname())
	if u.Scheme != "https" && (u.Scheme != "http" || ip == nil || !ip.IsLoopback()) {
		return "", errors.New("HTTPS or literal loopback required")
	}
	return strings.TrimRight(raw, "/"), nil
}
func New(cfg Config) (*Client, error) {
	base, e := endpoint(cfg.BaseURL, true)
	if e != nil {
		return nil, e
	}
	tokenURL, e := endpoint(cfg.TokenURL, false)
	if e != nil {
		return nil, e
	}
	if cfg.Owner.App == "" || cfg.Owner.Role == "" || cfg.Owner.Subject == "" || cfg.TargetID == "" || cfg.ClientID == "" || cfg.ClientSecretFile == "" {
		return nil, errors.New("owner, target, and service credentials required")
	}
	if _, e = privateSecret(cfg.ClientSecretFile); e != nil {
		return nil, e
	}
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 15 * time.Second
	}
	if timeout < time.Millisecond || timeout > time.Minute {
		return nil, errors.New("invalid HTTP timeout")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	if cfg.CAFile != "" {
		pem, e := os.ReadFile(cfg.CAFile)
		if e != nil {
			return nil, errors.New("runtime CA unavailable")
		}
		roots, e := x509.SystemCertPool()
		if e != nil {
			roots = x509.NewCertPool()
		}
		if !roots.AppendCertsFromPEM(pem) {
			return nil, errors.New("invalid runtime CA")
		}
		transport.TLSClientConfig.RootCAs = roots
	}
	client := &http.Client{Timeout: timeout, Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirect refused") }}
	return &Client{owner: cfg.Owner, target: cfg.TargetID, clientID: cfg.ClientID, secretFile: cfg.ClientSecretFile, base: base, tokenURL: tokenURL, http: client, now: time.Now}, nil
}
func privateSecret(path string) (string, error) {
	f, e := os.Open(path)
	if e != nil {
		return "", errors.New("service secret unavailable")
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return "", errors.New("private service secret required")
	}
	b, e := io.ReadAll(io.LimitReader(f, 16385))
	if e != nil || len(b) > 16384 || strings.TrimSpace(string(b)) == "" {
		return "", errors.New("invalid service secret")
	}
	return strings.TrimSpace(string(b)), nil
}
func (c *Client) TargetID() string { return c.target }
func (c *Client) ClientID() string { return c.clientID }
func (c *Client) checkOwner(owner jobs.Owner) error {
	if owner != c.owner {
		return ErrOwner
	}
	return nil
}
func (c *Client) token(ctx context.Context) (string, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()
	if c.accessToken != "" && c.now().Before(c.refreshAt) {
		return c.accessToken, nil
	}
	secret, e := privateSecret(c.secretFile)
	if e != nil {
		return "", &Error{Kind: "authentication_unavailable"}
	}
	form := url.Values{"grant_type": {"client_credentials"}}
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURL, strings.NewReader(form.Encode()))
	if e != nil {
		return "", &Error{Kind: "authentication_unavailable"}
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(url.QueryEscape(c.clientID), url.QueryEscape(secret))
	res, e := c.http.Do(req)
	if e != nil {
		return "", &Error{Kind: "authentication_unavailable"}
	}
	defer res.Body.Close()
	body, e := io.ReadAll(io.LimitReader(res.Body, 65537))
	if e != nil || len(body) > 65536 || res.StatusCode != http.StatusOK {
		return "", &Error{Kind: "authentication_unavailable"}
	}
	var t struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if json.Unmarshal(body, &t) != nil || t.AccessToken == "" || len(t.AccessToken) > 32768 || !strings.EqualFold(t.TokenType, "bearer") || t.ExpiresIn < 1 || t.ExpiresIn > 86400 {
		return "", &Error{Kind: "authentication_unavailable"}
	}
	life := time.Duration(t.ExpiresIn) * time.Second
	skew := life / 10
	if skew > 30*time.Second {
		skew = 30 * time.Second
	}
	c.accessToken = t.AccessToken
	c.refreshAt = c.now().Add(life - skew)
	return c.accessToken, nil
}
func (c *Client) invalidateToken() {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()
	c.accessToken = ""
	c.refreshAt = time.Time{}
}
func validID(id string) bool {
	if len(id) == 0 || len(id) > 128 {
		return false
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}
func validate(r Request) ([]byte, error) {
	if r.Profile != "text" || len(strings.TrimSpace(r.Prompt)) == 0 || len(r.Prompt) > 32768 || len(r.OutputSchema) == 0 || len(r.OutputSchema) > 32768 || len(r.Input) == 0 || !json.Valid(r.Input) || len(r.IdempotencyKey) == 0 || len(r.IdempotencyKey) > 128 {
		return nil, &Error{Kind: "invalid_request"}
	}
	var schema map[string]any
	if json.Unmarshal(r.OutputSchema, &schema) != nil || schema == nil {
		return nil, &Error{Kind: "invalid_request"}
	}
	b, e := json.Marshal(r)
	if e != nil || len(b) > MaxRequestBytes {
		return nil, &Error{Kind: "invalid_request"}
	}
	return b, nil
}
func (c *Client) Create(ctx context.Context, owner jobs.Owner, r Request) (Job, error) {
	if e := c.checkOwner(owner); e != nil {
		return Job{}, e
	}
	b, e := validate(r)
	if e != nil {
		return Job{}, e
	}
	return c.call(ctx, http.MethodPost, "/v1/jobs", b, "")
}
func (c *Client) Get(ctx context.Context, owner jobs.Owner, id string) (Job, error) {
	if e := c.checkOwner(owner); e != nil {
		return Job{}, e
	}
	if !validID(id) {
		return Job{}, &Error{Kind: "invalid_request"}
	}
	return c.call(ctx, http.MethodGet, "/v1/jobs/"+id, nil, id)
}

// Action supports waiting-state retry and cancel. A failed-state rerun must
// create a new attempt/key through the upper API; this method never does so.
func (c *Client) Action(ctx context.Context, owner jobs.Owner, id, action string) (Job, error) {
	if e := c.checkOwner(owner); e != nil {
		return Job{}, e
	}
	if !validID(id) || (action != "cancel" && action != "retry") {
		return Job{}, &Error{Kind: "invalid_request"}
	}
	return c.call(ctx, http.MethodPost, "/v1/jobs/"+id+"/"+action, []byte("{}"), id)
}
func (c *Client) call(ctx context.Context, method, path string, body []byte, expectedID string) (Job, error) {
	token, e := c.token(ctx)
	if e != nil {
		return Job{}, e
	}
	req, e := http.NewRequestWithContext(ctx, method, c.base+path, bytes.NewReader(body))
	if e != nil {
		return Job{}, &Error{Kind: "transport_unavailable"}
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	res, e := c.http.Do(req)
	if e != nil {
		return Job{}, &Error{Kind: "transport_unavailable"}
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusAccepted {
		if res.StatusCode == http.StatusUnauthorized {
			c.invalidateToken()
		}
		kind := "upstream_rejected"
		switch res.StatusCode {
		case 401, 403:
			kind = "authentication_rejected"
		case 404:
			kind = "not_found"
		case 409:
			kind = "conflict"
		case 429:
			kind = "admission_limited"
		case 503:
			kind = "upstream_unavailable"
		}
		return Job{}, &Error{Kind: kind, Status: res.StatusCode}
	}
	b, e := io.ReadAll(io.LimitReader(res.Body, MaxResponseBytes+1))
	if e != nil || len(b) > MaxResponseBytes {
		return Job{}, &Error{Kind: "invalid_response"}
	}
	var j Job
	if json.Unmarshal(b, &j) != nil || !validID(j.ID) || len(j.Reason) > 512 || (expectedID != "" && j.ID != expectedID) {
		return Job{}, &Error{Kind: "invalid_response"}
	}
	switch j.State {
	case "queued", "running", "waiting", "succeeded", "failed", "cancelled":
	default:
		return Job{}, &Error{Kind: "invalid_response"}
	}
	if j.Usage != nil && (j.Usage.InputTokens < 0 || j.Usage.OutputTokens < 0) {
		return Job{}, &Error{Kind: "invalid_response"}
	}
	return j, nil
}

// Package llmopsclient implements the Platform API wire boundary. It does not
// import the platform module or hold model-provider credentials.
package llmopsclient

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/agentclient"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

var safeID = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,127}$`)

type Client struct {
	base  string
	http  *http.Client
	token func(context.Context) (string, error)
}

func New(base string, token func(context.Context) (string, error)) (*Client, error) {
	u, e := url.Parse(base)
	if e != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || strings.Trim(u.Path, "/") != "" {
		return nil, errors.New("invalid Platform API URL")
	}
	ip := net.ParseIP(u.Hostname())
	if u.Scheme != "https" && (u.Scheme != "http" || ip == nil || !ip.IsLoopback()) {
		return nil, errors.New("Platform API requires HTTPS or literal loopback")
	}
	if token == nil {
		return nil, errors.New("Platform authentication required")
	}
	return &Client{base: strings.TrimRight(base, "/"), http: &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("Platform redirect refused") }}, token: token}, nil
}
func EndpointID(kind, base, clientID string) string {
	sum := sha256.Sum256([]byte(strings.TrimRight(base, "/") + "\x00" + clientID))
	return kind + "-" + hex.EncodeToString(sum[:12])
}
func privateFile(path string) (string, error) {
	f, e := os.Open(path)
	if e != nil {
		return "", errors.New("Platform credential unavailable")
	}
	defer f.Close()
	fi, e := f.Stat()
	if e != nil || !fi.Mode().IsRegular() || fi.Mode().Perm()&0077 != 0 {
		return "", errors.New("private Platform credential file required")
	}
	b, e := io.ReadAll(io.LimitReader(f, 16385))
	if e != nil || len(b) > 16384 || strings.TrimSpace(string(b)) == "" {
		return "", errors.New("invalid Platform credential file")
	}
	return strings.TrimSpace(string(b)), nil
}

type Config struct{ URL, TokenURL, ClientID, ClientSecretFile, CAFile string }

func FromEnvironment(ctx context.Context) (agentclient.Gateway, error) {
	return FromConfig(ctx, Config{URL: os.Getenv("KONRO_LLMOPS_URL"), TokenURL: os.Getenv("KONRO_LLMOPS_TOKEN_URL"), ClientID: os.Getenv("KONRO_LLMOPS_CLIENT_ID"), ClientSecretFile: os.Getenv("KONRO_LLMOPS_CLIENT_SECRET_FILE"), CAFile: os.Getenv("KONRO_LLMOPS_CA_FILE")})
}
func FromConfig(ctx context.Context, cfg Config) (agentclient.Gateway, error) {
	base := cfg.URL
	if base == "" {
		return nil, nil
	}
	tokenURL := cfg.TokenURL
	u, e := url.Parse(tokenURL)
	if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("HTTPS Platform token endpoint required")
	}
	id := cfg.ClientID
	secret, e := privateFile(cfg.ClientSecretFile)
	if e != nil || id == "" {
		return nil, errors.New("Platform client credentials unavailable")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	if path := cfg.CAFile; path != "" {
		pem, e := os.ReadFile(path)
		if e != nil {
			return nil, errors.New("Platform CA unavailable")
		}
		roots, e := x509.SystemCertPool()
		if e != nil {
			roots = x509.NewCertPool()
		}
		if !roots.AppendCertsFromPEM(pem) {
			return nil, errors.New("invalid Platform CA")
		}
		transport.TLSClientConfig.RootCAs = roots
	}
	hc := &http.Client{Timeout: 15 * time.Second, Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("Platform redirect refused") }}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, hc)
	source := (&clientcredentials.Config{ClientID: id, ClientSecret: secret, TokenURL: tokenURL}).TokenSource(ctx)
	c, e := New(base, func(context.Context) (string, error) {
		t, e := source.Token()
		if e != nil {
			return "", errors.New("Platform authentication unavailable")
		}
		return t.AccessToken, nil
	})
	if e != nil {
		return nil, e
	}
	c.http = hc
	return c, nil
}
func (c *Client) Create(ctx context.Context, r agentclient.Request) (agentclient.Job, error) {
	return c.call(ctx, "POST", "/v1/jobs", r.IdempotencyKey, map[string]any{"kind": "online", "profile": r.Profile, "prompt": r.Prompt, "input": r.Input, "output_schema": r.OutputSchema})
}
func (c *Client) Get(ctx context.Context, id string) (agentclient.Job, error) {
	if !safeID.MatchString(id) {
		return agentclient.Job{}, errors.New("invalid Platform job ID")
	}
	return c.call(ctx, "GET", "/v1/jobs/"+id, "", nil)
}
func (c *Client) Action(ctx context.Context, id, action string) (agentclient.Job, error) {
	if !safeID.MatchString(id) || (action != "cancel" && action != "retry") {
		return agentclient.Job{}, errors.New("invalid Platform action")
	}
	return c.call(ctx, "POST", "/v1/jobs/"+id+"/"+action, "konro-retry-"+id, struct{}{})
}
func (c *Client) call(ctx context.Context, method, path, key string, input any) (agentclient.Job, error) {
	var body []byte
	var e error
	if input != nil {
		body, e = json.Marshal(input)
		if e != nil {
			return agentclient.Job{}, errors.New("invalid Platform request")
		}
	}
	if len(body) > 512<<10 {
		return agentclient.Job{}, errors.New("Platform input too large; draft preserved")
	}
	req, e := http.NewRequestWithContext(ctx, method, c.base+path, bytes.NewReader(body))
	if e != nil {
		return agentclient.Job{}, e
	}
	token, e := c.token(ctx)
	if e != nil {
		return agentclient.Job{}, errors.New("Platform authentication unavailable; draft preserved")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	res, e := c.http.Do(req)
	if e != nil {
		return agentclient.Job{}, errors.New("Platform connection unavailable; draft preserved")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 && res.StatusCode != 202 {
		return agentclient.Job{}, errors.New("Platform request unavailable; draft preserved")
	}
	raw, e := io.ReadAll(io.LimitReader(res.Body, (1<<20)+1))
	var j agentclient.Job
	if e != nil || len(raw) > 1<<20 || json.Unmarshal(raw, &j) != nil || !safeID.MatchString(j.ID) {
		return j, errors.New("invalid Platform response")
	}
	switch j.State {
	case "queued", "running", "waiting", "succeeded", "failed", "cancelled":
	default:
		return j, errors.New("invalid Platform job state")
	}
	return j, nil
}

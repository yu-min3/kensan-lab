// Package agentclient implements the versioned HTTP contract. No runtime code,
// provider credentials, application types or shared Go modules cross this boundary.
package agentclient

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"syscall"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

type Job struct {
	ID     string          `json:"id"`
	State  string          `json:"state"`
	Reason string          `json:"reason,omitempty"`
	Output json.RawMessage `json:"output,omitempty"`
}
type Request struct {
	Profile        string          `json:"profile"`
	Prompt         string          `json:"prompt"`
	Input          any             `json:"input"`
	OutputSchema   json.RawMessage `json:"output_schema"`
	IdempotencyKey string          `json:"idempotency_key"`
}
type Gateway interface {
	Create(context.Context, Request) (Job, error)
	Get(context.Context, string) (Job, error)
	Action(context.Context, string, string) (Job, error)
}
type Client struct {
	base  string
	http  *http.Client
	token func(context.Context) (string, error)
}

func New(base string, token func(context.Context) (string, error)) (*Client, error) {
	u, e := url.Parse(base)
	if e != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || strings.Trim(u.Path, "/") != "" {
		return nil, errors.New("invalid agent-runtime URL")
	}
	ip := net.ParseIP(u.Hostname())
	if u.Scheme != "https" && (u.Scheme != "http" || ip == nil || !ip.IsLoopback()) {
		return nil, errors.New("agent-runtime requires HTTPS or literal loopback")
	}
	if token == nil {
		return nil, errors.New("agent authentication required")
	}
	return &Client{base: strings.TrimRight(base, "/"), http: &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("runtime redirect refused") }}, token: token}, nil
}
func privateFile(path string) (string, error) {
	info, e := os.Stat(path) // Kubernetes projected volumes use a rotation symlink.
	if e != nil || !info.Mode().IsRegular() {
		return "", errors.New("private credential file required")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	groupReadable := ok && stat.Gid == uint32(os.Getegid()) && info.Mode().Perm()&0027 == 0
	if info.Mode().Perm()&0077 != 0 && !groupReadable {
		return "", errors.New("private credential file required")
	}

	b, e := os.ReadFile(path)
	if e != nil || len(b) > 16384 || strings.TrimSpace(string(b)) == "" {
		return "", errors.New("invalid credential file")
	}
	return strings.TrimSpace(string(b)), nil
}

// FromEnvironment supports local development or a Keycloak client-credentials
// token. These are service credentials; model-provider credentials stay remote.
func FromEnvironment(ctx context.Context) (Gateway, error) {
	base := os.Getenv("KENSAN_AGENT_URL")
	if base == "" {
		return nil, nil
	}
	local := os.Getenv("KENSAN_AGENT_TOKEN_FILE")
	if local != "" {
		return New(base, func(context.Context) (string, error) { return privateFile(local) })
	}
	endpoint := os.Getenv("KENSAN_AGENT_TOKEN_URL")
	u, e := url.Parse(endpoint)
	if e != nil || u.Scheme != "https" || u.Host == "" {
		return nil, errors.New("HTTPS Keycloak token URL required")
	}
	secret, e := privateFile(os.Getenv("KENSAN_AGENT_CLIENT_SECRET_FILE"))
	if e != nil {
		return nil, e
	}
	id := os.Getenv("KENSAN_AGENT_CLIENT_ID")
	if id == "" {
		return nil, errors.New("agent client ID required")
	}
	httpClient := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("token redirect refused") }}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, httpClient)
	source := (&clientcredentials.Config{ClientID: id, ClientSecret: secret, TokenURL: endpoint}).TokenSource(ctx)
	client, err := New(base, func(context.Context) (string, error) {
		t, e := source.Token()
		if e != nil {
			return "", errors.New("agent authentication unavailable")
		}
		return t.AccessToken, nil
	})
	if err != nil {
		return nil, err
	}
	if caFile := os.Getenv("KENSAN_AGENT_CA_FILE"); caFile != "" {
		pem, err := os.ReadFile(caFile)
		if err != nil {
			return nil, errors.New("runtime CA unavailable")
		}
		roots, err := x509.SystemCertPool()
		if err != nil {
			roots = x509.NewCertPool()
		}
		if !roots.AppendCertsFromPEM(pem) {
			return nil, errors.New("invalid runtime CA")
		}
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.TLSClientConfig = &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}
		client.http.Transport = transport
	}
	return client, nil
}
func (c *Client) Create(ctx context.Context, r Request) (Job, error) {
	return c.call(ctx, "POST", "/v1/jobs", r)
}
func (c *Client) Get(ctx context.Context, id string) (Job, error) {
	return c.call(ctx, "GET", "/v1/jobs/"+url.PathEscape(id), nil)
}
func (c *Client) Action(ctx context.Context, id, action string) (Job, error) {
	if action != "cancel" && action != "retry" {
		return Job{}, errors.New("invalid action")
	}
	return c.call(ctx, "POST", "/v1/jobs/"+url.PathEscape(id)+"/"+action, struct{}{})
}
func (c *Client) call(ctx context.Context, method, path string, input any) (Job, error) {
	var body []byte
	if input != nil {
		body, _ = json.Marshal(input)
	}
	r, e := http.NewRequestWithContext(ctx, method, c.base+path, bytes.NewReader(body))
	if e != nil {
		return Job{}, e
	}
	token, e := c.token(ctx)
	if e != nil {
		return Job{}, e
	}
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Content-Type", "application/json")
	res, e := c.http.Do(r)
	if e != nil {
		return Job{}, errors.New("agent-runtimeに接続できません。下書きは保持されています")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 && res.StatusCode != 202 {
		return Job{}, fmt.Errorf("agent-runtime応答 %d。下書きは保持されています", res.StatusCode)
	}
	b, e := io.ReadAll(io.LimitReader(res.Body, (1<<20)+1))
	if e != nil || len(b) > 1<<20 {
		return Job{}, errors.New("runtime response too large")
	}
	var j Job
	if json.Unmarshal(b, &j) != nil || j.ID == "" {
		return Job{}, errors.New("invalid runtime response")
	}
	switch j.State {
	case "queued", "running", "waiting", "succeeded", "failed", "cancelled":
	default:
		return Job{}, errors.New("unknown runtime job state")
	}
	return j, nil
}

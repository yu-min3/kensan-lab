// Package vault wraps HashiCorp Vault Transit secret engine for column-level
// encryption. Designed for kensan's users.name PII encryption (Stage 6).
//
// Usage:
//
//	cli, err := vault.NewClient(vault.Config{
//	    Address:   os.Getenv("VAULT_ADDR"),
//	    Role:      os.Getenv("VAULT_AUTH_ROLE"),
//	    MountPath: "transit",
//	    KeyName:   "users-name",
//	})
//	if err != nil { ... }
//	defer cli.Close()
//
//	ct, err := cli.Encrypt(ctx, []byte("Yu Misaki"))
//	pt, err := cli.Decrypt(ctx, ct)
//	hm, err := cli.HMAC(ctx, []byte("Yu Misaki"))  // HMAC-SHA256 (検索用)
//
// Auth:
//   - Production: Kubernetes Service Account JWT (mounted at the well-known path)
//   - Local dev: VAULT_TOKEN env (skip k8s auth)
//
// Lease management:
//   - On NewClient(), the client logs in once and starts a background goroutine
//     that renews the token before TTL expiry.
//   - Close() stops the renewer.
package vault

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	vapi "github.com/hashicorp/vault/api"
	vauthk8s "github.com/hashicorp/vault/api/auth/kubernetes"
)

// DefaultK8sSATokenPath is the path where Kubernetes mounts the projected
// service account token inside the pod.
const DefaultK8sSATokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"

// Config holds all Transit client configuration. Required fields are tagged.
type Config struct {
	// Address is the Vault server URL (e.g., http://vault.vault.svc.cluster.local:8200).
	// Required.
	Address string

	// Role is the Vault Kubernetes auth role name (e.g., "kensan-users-transit").
	// Required when LocalToken is empty.
	Role string

	// MountPath is the Vault secret engine mount path (default: "transit").
	MountPath string

	// KeyName is the Transit key name (e.g., "users-name").
	// Required.
	KeyName string

	// AuthMountPath is the Kubernetes auth method mount path (default: "kubernetes").
	AuthMountPath string

	// SATokenPath overrides the SA token mount path (test only).
	SATokenPath string

	// LocalToken bypasses k8s auth and uses this token directly (local dev / test).
	LocalToken string

	// RenewBuffer is the duration before TTL expiry at which the renewer triggers
	// (default: 1/3 of token TTL, or 5 min, whichever is smaller).
	RenewBuffer time.Duration
}

// Client is a thin Vault Transit wrapper with automatic token renewal.
type Client struct {
	cfg     Config
	api     *vapi.Client
	stopCh  chan struct{}
	wg      sync.WaitGroup
	closeMu sync.Mutex
	closed  bool
}

// NewClient constructs a Client, performs initial login, and starts the
// background renewer. Caller must call Close() to release resources.
//
// Returns ErrNotConfigured when Address is empty (so caller can degrade
// gracefully — kensan supports running without Vault for local dev).
func NewClient(ctx context.Context, cfg Config) (*Client, error) {
	if cfg.Address == "" {
		return nil, ErrNotConfigured
	}
	if cfg.KeyName == "" {
		return nil, fmt.Errorf("vault: KeyName required")
	}
	if cfg.MountPath == "" {
		cfg.MountPath = "transit"
	}
	if cfg.AuthMountPath == "" {
		cfg.AuthMountPath = "kubernetes"
	}

	apiCfg := vapi.DefaultConfig()
	apiCfg.Address = cfg.Address
	if err := apiCfg.Error; err != nil {
		return nil, fmt.Errorf("vault: default config: %w", err)
	}
	api, err := vapi.NewClient(apiCfg)
	if err != nil {
		return nil, fmt.Errorf("vault: new api client: %w", err)
	}

	c := &Client{
		cfg:    cfg,
		api:    api,
		stopCh: make(chan struct{}),
	}

	// Initial login (sets api token + returns secret with renew metadata)
	secret, err := c.login(ctx)
	if err != nil {
		return nil, fmt.Errorf("vault: initial login: %w", err)
	}

	// Renewer (only when secret is renewable; static tokens skip)
	if secret != nil && secret.Auth != nil && secret.Auth.Renewable {
		c.wg.Add(1)
		go c.renewLoop(secret)
	}

	return c, nil
}

// Close stops the renewer goroutine. Safe to call multiple times.
func (c *Client) Close() {
	c.closeMu.Lock()
	if c.closed {
		c.closeMu.Unlock()
		return
	}
	c.closed = true
	close(c.stopCh)
	c.closeMu.Unlock()
	c.wg.Wait()
}

// Encrypt encrypts plaintext using the configured Transit key and returns the
// Vault ciphertext string (e.g., "vault:v1:abc...").
//
// Empty plaintext is rejected to avoid storing meaningless ciphertext.
func (c *Client) Encrypt(ctx context.Context, plaintext []byte) (string, error) {
	if len(plaintext) == 0 {
		return "", ErrEmptyPlaintext
	}
	path := fmt.Sprintf("%s/encrypt/%s", c.cfg.MountPath, c.cfg.KeyName)
	resp, err := c.api.Logical().WriteWithContext(ctx, path, map[string]any{
		"plaintext": b64Encode(plaintext),
	})
	if err != nil {
		return "", fmt.Errorf("vault: encrypt: %w", err)
	}
	if resp == nil || resp.Data == nil {
		return "", fmt.Errorf("vault: encrypt: empty response")
	}
	ct, ok := resp.Data["ciphertext"].(string)
	if !ok {
		return "", fmt.Errorf("vault: encrypt: ciphertext missing or not string")
	}
	return ct, nil
}

// Decrypt decrypts a Vault ciphertext (e.g., "vault:v1:...") and returns the
// original plaintext bytes.
func (c *Client) Decrypt(ctx context.Context, ciphertext string) ([]byte, error) {
	if ciphertext == "" {
		return nil, ErrEmptyCiphertext
	}
	path := fmt.Sprintf("%s/decrypt/%s", c.cfg.MountPath, c.cfg.KeyName)
	resp, err := c.api.Logical().WriteWithContext(ctx, path, map[string]any{
		"ciphertext": ciphertext,
	})
	if err != nil {
		return nil, fmt.Errorf("vault: decrypt: %w", err)
	}
	if resp == nil || resp.Data == nil {
		return nil, fmt.Errorf("vault: decrypt: empty response")
	}
	b64, ok := resp.Data["plaintext"].(string)
	if !ok {
		return nil, fmt.Errorf("vault: decrypt: plaintext missing or not string")
	}
	pt, err := b64Decode(b64)
	if err != nil {
		return nil, fmt.Errorf("vault: decrypt: base64 decode: %w", err)
	}
	return pt, nil
}

// HMAC computes HMAC-SHA256 (default) of the input using the Transit key
// material, returning Vault's "vault:v1:..." HMAC string. Used for searchable
// equality comparisons over encrypted columns (exact match only — no LIKE).
func (c *Client) HMAC(ctx context.Context, input []byte) (string, error) {
	if len(input) == 0 {
		return "", ErrEmptyPlaintext
	}
	path := fmt.Sprintf("%s/hmac/%s", c.cfg.MountPath, c.cfg.KeyName)
	resp, err := c.api.Logical().WriteWithContext(ctx, path, map[string]any{
		"input":     b64Encode(input),
		"algorithm": "sha2-256",
	})
	if err != nil {
		return "", fmt.Errorf("vault: hmac: %w", err)
	}
	if resp == nil || resp.Data == nil {
		return "", fmt.Errorf("vault: hmac: empty response")
	}
	h, ok := resp.Data["hmac"].(string)
	if !ok {
		return "", fmt.Errorf("vault: hmac: hmac missing or not string")
	}
	return h, nil
}

// Rewrap re-encrypts a ciphertext to the latest key version (used after
// `vault write -f transit/keys/users-name/rotate`). Returns the new ciphertext.
func (c *Client) Rewrap(ctx context.Context, ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", ErrEmptyCiphertext
	}
	path := fmt.Sprintf("%s/rewrap/%s", c.cfg.MountPath, c.cfg.KeyName)
	resp, err := c.api.Logical().WriteWithContext(ctx, path, map[string]any{
		"ciphertext": ciphertext,
	})
	if err != nil {
		return "", fmt.Errorf("vault: rewrap: %w", err)
	}
	if resp == nil || resp.Data == nil {
		return "", fmt.Errorf("vault: rewrap: empty response")
	}
	ct, ok := resp.Data["ciphertext"].(string)
	if !ok {
		return "", fmt.Errorf("vault: rewrap: ciphertext missing or not string")
	}
	return ct, nil
}

// login performs the initial Vault login (k8s SA JWT or LocalToken) and stores
// the token on the api client.
func (c *Client) login(ctx context.Context) (*vapi.Secret, error) {
	if c.cfg.LocalToken != "" {
		c.api.SetToken(c.cfg.LocalToken)
		return nil, nil
	}
	if c.cfg.Role == "" {
		return nil, fmt.Errorf("vault: Role required for k8s auth (or set LocalToken for dev)")
	}

	saTokenPath := c.cfg.SATokenPath
	if saTokenPath == "" {
		saTokenPath = DefaultK8sSATokenPath
	}

	auth, err := vauthk8s.NewKubernetesAuth(
		c.cfg.Role,
		vauthk8s.WithServiceAccountTokenPath(saTokenPath),
		vauthk8s.WithMountPath(c.cfg.AuthMountPath),
	)
	if err != nil {
		return nil, fmt.Errorf("vault: build k8s auth: %w", err)
	}
	secret, err := c.api.Auth().Login(ctx, auth)
	if err != nil {
		return nil, fmt.Errorf("vault: k8s login: %w", err)
	}
	if secret == nil || secret.Auth == nil {
		return nil, fmt.Errorf("vault: k8s login returned nil auth")
	}
	return secret, nil
}

// renewLoop renews the token before TTL expiry. Falls back to re-login if
// renewal fails (e.g., max-TTL reached).
func (c *Client) renewLoop(initial *vapi.Secret) {
	defer c.wg.Done()

	current := initial
	for {
		ttl := time.Duration(current.Auth.LeaseDuration) * time.Second
		if ttl <= 0 {
			ttl = 30 * time.Minute // safety default
		}
		buffer := c.cfg.RenewBuffer
		if buffer <= 0 {
			buffer = ttl / 3
			if buffer > 5*time.Minute {
				buffer = 5 * time.Minute
			}
		}
		wait := ttl - buffer
		if wait < 5*time.Second {
			wait = 5 * time.Second
		}

		select {
		case <-c.stopCh:
			return
		case <-time.After(wait):
		}

		// Try renew-self first.
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		renewed, err := c.api.Auth().Token().RenewSelfWithContext(ctx, int(ttl.Seconds()))
		cancel()
		if err == nil && renewed != nil && renewed.Auth != nil {
			current = renewed
			slog.Debug("vault: token renewed",
				"lease_duration", renewed.Auth.LeaseDuration,
				"renewable", renewed.Auth.Renewable)
			continue
		}

		// Renewal failed (likely max TTL). Re-login from scratch.
		slog.Warn("vault: token renew failed, attempting re-login", "error", err)
		ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
		fresh, lerr := c.login(ctx2)
		cancel2()
		if lerr != nil {
			slog.Error("vault: re-login failed, token will expire", "error", lerr)
			// Keep looping with a short backoff so we recover when Vault is back.
			select {
			case <-c.stopCh:
				return
			case <-time.After(30 * time.Second):
			}
			continue
		}
		if fresh != nil {
			current = fresh
		}
	}
}

// Sentinel errors callers can match with errors.Is.
var (
	ErrNotConfigured   = errors.New("vault: not configured (Address empty)")
	ErrEmptyPlaintext  = errors.New("vault: plaintext empty")
	ErrEmptyCiphertext = errors.New("vault: ciphertext empty")
)

// LoadConfigFromEnv reads VAULT_ADDR / VAULT_AUTH_ROLE / VAULT_TRANSIT_KEY etc.
// from the process environment and returns a Config. Empty Address triggers
// "not configured" mode (callers should treat NewClient(...) as optional).
func LoadConfigFromEnv() Config {
	return Config{
		Address:       os.Getenv("VAULT_ADDR"),
		Role:          os.Getenv("VAULT_AUTH_ROLE"),
		MountPath:     getenvOr("VAULT_TRANSIT_MOUNT", "transit"),
		KeyName:       os.Getenv("VAULT_TRANSIT_KEY"),
		AuthMountPath: getenvOr("VAULT_AUTH_MOUNT", "kubernetes"),
		LocalToken:    os.Getenv("VAULT_TOKEN"),
	}
}

func getenvOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

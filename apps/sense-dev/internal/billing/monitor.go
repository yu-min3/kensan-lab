// Package billing keeps Claude billing credentials and usage checks on the host.
// It never persists an allow decision: each process starts denied.
package billing

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const Interval = 5 * time.Minute
const endpoint = "https://api.anthropic.com/api/oauth/usage"

var ErrBlocked = errors.New("Claude extra usage OFF confirmation unavailable or expired")

type credential struct {
	token    string
	identity [32]byte
	expires  time.Time
}

// Status contains only safe metadata; no token, account data or response body.
type Status struct {
	Allowed   bool
	Reason    string
	CheckedAt time.Time
	ExpiresAt time.Time
	NextCheck time.Time
}

type Monitor struct {
	authHome string
	client   *http.Client
	now      func() time.Time
	mu       sync.Mutex
	pollMu   sync.Mutex
	status   Status
	identity [32]byte
}

func New(authHome string) *Monitor {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	return &Monitor{authHome: authHome, now: time.Now, client: &http.Client{
		Timeout: 15 * time.Second, Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("billing redirect rejected") },
	}, status: Status{Reason: "not_checked"}}
}

func (m *Monitor) credential() (credential, error) {
	var c credential
	home, err := os.Lstat(m.authHome)
	if err != nil || !home.IsDir() || home.Mode().Perm()&0077 != 0 {
		return c, ErrBlocked
	}
	path := filepath.Join(m.authHome, ".credentials.json")
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return c, ErrBlocked
	}
	f, err := os.Open(path)
	if err != nil {
		return c, ErrBlocked
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) || opened.Mode().Perm()&0077 != 0 {
		return c, ErrBlocked
	}
	b, err := io.ReadAll(io.LimitReader(f, (1<<20)+1))
	if err != nil || len(b) > 1<<20 {
		return c, ErrBlocked
	}
	var v struct {
		OAuth struct {
			AccessToken string `json:"accessToken"`
			ExpiresAt   int64  `json:"expiresAt"`
		} `json:"claudeAiOauth"`
	}
	if json.Unmarshal(b, &v) != nil || v.OAuth.AccessToken == "" {
		return c, ErrBlocked
	}
	c.token = v.OAuth.AccessToken
	c.identity = sha256.Sum256([]byte(c.token))
	c.expires = time.UnixMilli(v.OAuth.ExpiresAt)
	if !m.now().Before(c.expires) {
		return credential{}, ErrBlocked
	}
	return c, nil
}

// Status invalidates cached permission immediately when credentials change or
// expire. It performs no network requests, even when called by many workers.
func (m *Monitor) Status() Status {
	c, err := m.credential()
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.status
	if s.Allowed && (err != nil || c.identity != m.identity) {
		s.Allowed = false
		s.Reason = "credentials_changed_or_unavailable"
	}
	if s.Allowed && !m.now().Before(s.ExpiresAt) {
		s.Allowed = false
		s.Reason = "confirmation_expired"
	}
	return s
}

func (m *Monitor) Check(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !m.Status().Allowed {
		return ErrBlocked
	}
	return nil
}

// Poll serializes requests and respects the provider's Retry-After. Only a
// complete, explicit OFF + zero-used response grants permission for five minutes.
func (m *Monitor) Poll(ctx context.Context) {
	m.pollMu.Lock()
	defer m.pollMu.Unlock()
	started := m.now()
	m.mu.Lock()
	due := m.status.NextCheck
	m.mu.Unlock()
	if started.Before(due) {
		return
	}
	c, err := m.credential()
	if err != nil {
		m.deny(started, "credentials_unavailable")
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		m.deny(started, "request_failed")
		return
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")
	response, err := m.client.Do(req)
	if err != nil {
		m.deny(started, "request_failed")
		return
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusTooManyRequests {
		next := retryAfter(response.Header.Get("Retry-After"), m.now())
		m.mu.Lock()
		// Keep only the previous unexpired OFF decision; never extend its expiry.
		m.status.NextCheck = next
		if !m.status.Allowed {
			m.status.Reason = "rate_limited"
		}
		m.mu.Unlock()
		return
	}
	if response.StatusCode != http.StatusOK {
		m.deny(started, "http_error")
		return
	}
	b, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	if err != nil || len(b) > 1<<20 {
		m.deny(started, "invalid_response")
		return
	}
	var usage struct {
		Extra *struct {
			Enabled *bool    `json:"is_enabled"`
			Used    *float64 `json:"used_credits"`
		} `json:"extra_usage"`
	}
	if json.Unmarshal(b, &usage) != nil || usage.Extra == nil || usage.Extra.Enabled == nil || usage.Extra.Used == nil {
		m.deny(started, "invalid_response")
		return
	}
	if *usage.Extra.Enabled || *usage.Extra.Used != 0 {
		m.deny(started, "extra_usage_enabled_or_used")
		return
	}
	current, err := m.credential()
	if err != nil || current.identity != c.identity {
		m.deny(started, "credentials_changed")
		return
	}
	expires := started.Add(Interval)
	if c.expires.Before(expires) {
		expires = c.expires
	}
	m.mu.Lock()
	m.identity = c.identity
	m.status = Status{Allowed: true, Reason: "confirmed_off", CheckedAt: started, ExpiresAt: expires, NextCheck: started.Add(Interval)}
	m.mu.Unlock()
}

func (m *Monitor) deny(now time.Time, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status = Status{Reason: reason, NextCheck: now.Add(Interval)}
}

func retryAfter(value string, now time.Time) time.Time {
	if seconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64); err == nil && seconds >= 0 && seconds <= int64((1<<63-1)/int64(time.Second)) {
		// A zero delay must not create an immediate busy retry loop.
		if seconds < 1 {
			seconds = 1
		}
		return now.Add(time.Duration(seconds) * time.Second)
	}
	if date, err := http.ParseTime(value); err == nil && date.After(now) {
		return date
	}
	return now.Add(Interval)
}

// Run only probes when due. A 1s local timer lets Retry-After be honored without
// creating extra HTTP calls. Cancellation interrupts an in-flight HTTP request.
func (m *Monitor) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	previous := m.Status()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.Poll(ctx)
			current := m.Status()
			if previous.Allowed != current.Allowed || previous.Reason != current.Reason {
				log.Printf("Claude billing admission: allowed=%t reason=%s", current.Allowed, current.Reason)
			}
			previous = current
		}
	}
}

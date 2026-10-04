package billing

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func fixture(t *testing.T) (*Monitor, *time.Time, *atomic.Int32) {
	t.Helper()
	home := t.TempDir()
	if err := os.Chmod(home, 0700); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)
	credentialFile(t, home, "fixture-secret", now.Add(time.Hour))
	m := New(home)
	m.now = func() time.Time { return now }
	calls := new(atomic.Int32)
	m.client.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		if r.URL.String() != endpoint || r.Header.Get("Authorization") != "Bearer fixture-secret" || r.Header.Get("anthropic-beta") == "" {
			t.Error("request escaped pinned endpoint or lost authentication")
		}
		return response(200, `{"extra_usage":{"is_enabled":false,"used_credits":0}}`, ""), nil
	})
	return m, &now, calls
}
func credentialFile(t *testing.T, home, token string, expiry time.Time) {
	t.Helper()
	b := `{"claudeAiOauth":{"accessToken":"` + token + `","expiresAt":` + strconv.FormatInt(expiry.UnixMilli(), 10) + `}}`
	if err := os.WriteFile(filepath.Join(home, ".credentials.json"), []byte(b), 0600); err != nil {
		t.Fatal(err)
	}
}
func response(code int, body, retry string) *http.Response {
	h := make(http.Header)
	h.Set("Retry-After", retry)
	return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(body)), Header: h}
}
func TestSharedConfirmationHasNoPerWorkerHTTP(t *testing.T) {
	m, now, calls := fixture(t)
	if m.Check(context.Background()) == nil {
		t.Fatal("startup allowed without fresh confirmation")
	}
	m.Poll(context.Background())
	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.Poll(context.Background())
			if err := m.Check(context.Background()); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("shared consumers made %d requests", calls.Load())
	}
	*now = now.Add(Interval)
	if m.Check(context.Background()) == nil {
		t.Fatal("expired confirmation allowed a model")
	}
	m.Poll(context.Background())
	if calls.Load() != 2 || m.Check(context.Background()) != nil {
		t.Fatal("due successful poll did not renew")
	}
	// A new process cannot inherit the previous allow decision.
	fresh := New(m.authHome)
	if fresh.Check(context.Background()) == nil {
		t.Fatal("restart inherited permission")
	}
}
func TestRejectsEnabledUsedMissingAndMalformedResponse(t *testing.T) {
	for _, body := range []string{
		`{"extra_usage":{"is_enabled":true,"used_credits":0}}`,
		`{"extra_usage":{"is_enabled":false,"used_credits":0.01}}`,
		`{"extra_usage":{"is_enabled":false,"used_credits":-1}}`,
		`{"extra_usage":{"is_enabled":false}}`,
		`{"extra_usage":{"used_credits":0}}`,
		`{"extra_usage":null}`, `{}`, `fixture-secret-not-json`,
	} {
		t.Run(body, func(t *testing.T) {
			m, now, _ := fixture(t)
			m.Poll(context.Background())
			*now = now.Add(Interval)
			m.client.Transport = transportFunc(func(*http.Request) (*http.Response, error) { return response(200, body, ""), nil })
			m.Poll(context.Background())
			if m.Check(context.Background()) == nil {
				t.Fatal("invalid billing response granted permission")
			}
			if strings.Contains(m.Status().Reason, "fixture-secret") {
				t.Fatal("response leaked to status")
			}
		})
	}
}
func TestRateLimitCannotExtendLastPermission(t *testing.T) {
	m, now, calls := fixture(t)
	m.Poll(context.Background())
	// Even an early scheduled recheck must retain the ORIGINAL expiry.
	*now = now.Add(time.Minute)
	m.mu.Lock()
	m.status.NextCheck = *now
	m.mu.Unlock()
	m.client.Transport = transportFunc(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return response(429, "fixture-secret", "600"), nil
	})
	original := m.Status().ExpiresAt
	m.Poll(context.Background())
	if m.Status().ExpiresAt != original || m.Check(context.Background()) != nil {
		t.Fatal("429 discarded unexpired OFF or extended expiry")
	}
	*now = now.Add(4 * time.Minute)
	if m.Check(context.Background()) == nil {
		t.Fatal("429 extended permission past expiry")
	}
	m.Poll(context.Background())
	if calls.Load() != 2 {
		t.Fatal("Retry-After violated")
	}
	*now = now.Add(6 * time.Minute)
	m.Poll(context.Background())
	if calls.Load() != 3 {
		t.Fatal("did not retry when due")
	}
}
func TestRetryAfterDateAndInvalidValues(t *testing.T) {
	now := time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)
	for _, v := range []string{"bad", "-1", "999999999999999999999"} {
		if retryAfter(v, now) != now.Add(Interval) {
			t.Fatal("unsafe Retry-After fallback")
		}
	}
	expected := now.Add(2 * time.Minute)
	if retryAfter(expected.Format(http.TimeFormat), now) != expected {
		t.Fatal("HTTP date ignored")
	}
	if !retryAfter("0", now).After(now) {
		t.Fatal("zero retry can busy loop")
	}
}
func TestCredentialRotationExpiryAndPermissionsRevokePermission(t *testing.T) {
	for _, change := range []string{"rotation", "expiry", "permissions", "symlink"} {
		t.Run(change, func(t *testing.T) {
			m, now, calls := fixture(t)
			m.Poll(context.Background())
			switch change {
			case "rotation":
				credentialFile(t, m.authHome, "other-account-token", now.Add(time.Hour))
			case "expiry":
				credentialFile(t, m.authHome, "fixture-secret", now.Add(-time.Second))
			case "permissions":
				if err := os.Chmod(filepath.Join(m.authHome, ".credentials.json"), 0644); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				path := filepath.Join(m.authHome, ".credentials.json")
				if err := os.Rename(path, path+".old"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(path+".old", path); err != nil {
					t.Fatal(err)
				}
			}
			if m.Check(context.Background()) == nil {
				t.Fatal("changed credential reused old permission")
			}
			if calls.Load() != 1 {
				t.Fatal("check made network request")
			}
		})
	}
}
func TestTransportFailureRedactedAndRevokesPermission(t *testing.T) {
	m, now, _ := fixture(t)
	m.Poll(context.Background())
	*now = now.Add(Interval)
	m.client.Transport = transportFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("fixture-secret transport failure")
	})
	m.Poll(context.Background())
	if m.Check(context.Background()) == nil || strings.Contains(m.Status().Reason, "fixture-secret") {
		t.Fatal("failed or leaked open")
	}
}
func TestCredentialChangeDuringProbeRejected(t *testing.T) {
	m, now, _ := fixture(t)
	m.client.Transport = transportFunc(func(*http.Request) (*http.Response, error) {
		credentialFile(t, m.authHome, "replacement-token", now.Add(time.Hour))
		return response(200, `{"extra_usage":{"is_enabled":false,"used_credits":0}}`, ""), nil
	})
	m.Poll(context.Background())
	if m.Check(context.Background()) == nil {
		t.Fatal("response bound to old credentials admitted new account")
	}
}

func TestBillingRedirectCannotForwardCredentials(t *testing.T) {
	m, _, calls := fixture(t)
	m.client.Transport = transportFunc(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		r := response(http.StatusFound, "", "")
		r.Header.Set("Location", "https://untrusted.invalid/steal")
		return r, nil
	})
	m.Poll(context.Background())
	if calls.Load() != 1 || m.Check(context.Background()) == nil {
		t.Fatal("redirect followed or admitted despite unverified billing")
	}
}

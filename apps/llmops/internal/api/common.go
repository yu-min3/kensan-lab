// Package api exposes authenticated Platform API handlers. It never trusts
// identity headers or owner fields supplied by a caller.
package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/yu-min3/kensan-lab/apps/llmops/contracts"
	"github.com/yu-min3/kensan-lab/apps/llmops/internal/identity"
	"github.com/yu-min3/kensan-lab/apps/llmops/internal/objects"
	"github.com/yu-min3/kensan-lab/apps/llmops/internal/policy"
)

type Verifier interface {
	Verify(context.Context, string) (identity.Principal, error)
}
type CaptureFeedbackConfig struct {
	Auth   Verifier
	Policy *policy.Engine
	Stores map[string]*objects.Store
	Now    func() time.Time
}
type CaptureFeedback struct {
	auth   Verifier
	policy *policy.Engine
	stores map[string]*objects.Store
	now    func() time.Time
}

func NewCaptureFeedback(cfg CaptureFeedbackConfig) (*CaptureFeedback, error) {
	if cfg.Auth == nil || cfg.Policy == nil || len(cfg.Stores) == 0 {
		return nil, errors.New("authentication, policy and app stores required")
	}
	stores := make(map[string]*objects.Store, len(cfg.Stores))
	for app, s := range cfg.Stores {
		if s == nil || s.AppID() != app {
			return nil, errors.New("invalid app store")
		}
		stores[app] = s
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &CaptureFeedback{cfg.Auth, cfg.Policy, stores, cfg.Now}, nil
}
func (h *CaptureFeedback) Register(mux *http.ServeMux) {
	mux.Handle("POST /v1/captures", h.Authenticated(http.HandlerFunc(h.capture)))
	mux.Handle("POST /v1/feedback", h.Authenticated(http.HandlerFunc(h.feedback)))
}

type principalKey struct{}

func Principal(r *http.Request) (identity.Principal, bool) {
	p, ok := r.Context().Value(principalKey{}).(identity.Principal)
	return p, ok
}
func (h *CaptureFeedback) Authenticated(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		values := r.Header.Values("Authorization")
		if len(values) != 1 || !strings.HasPrefix(values[0], "Bearer ") {
			writeError(w, 401, "unauthorized")
			return
		}
		token := strings.TrimPrefix(values[0], "Bearer ")
		if token == "" || strings.ContainsAny(token, " \t\r\n") {
			writeError(w, 401, "unauthorized")
			return
		}
		p, e := h.auth.Verify(r.Context(), token)
		if e != nil {
			writeError(w, 401, "unauthorized")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalKey{}, p)))
	})
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, status int, code string) {
	var id [16]byte
	if _, e := rand.Read(id[:]); e != nil {
		id[0] = 1
	}
	writeJSON(w, status, map[string]string{"code": code, "message": code, "trace_id": hex.EncodeToString(id[:])})
}
func storageError(w http.ResponseWriter, e error) {
	switch {
	case errors.Is(e, objects.ErrNotFound), errors.Is(e, policy.ErrNotFound):
		writeError(w, 404, "not_found")
	case errors.Is(e, policy.ErrForbidden):
		writeError(w, 403, "forbidden")
	case errors.Is(e, objects.ErrConflict):
		writeError(w, 409, "conflict")
	case errors.Is(e, objects.ErrTooLarge):
		writeError(w, 413, "too_large")
	case errors.Is(e, objects.ErrInvalid):
		writeError(w, 400, "invalid_request")
	default:
		writeError(w, 503, "unavailable")
	}
}

// decodeObject rejects duplicate keys at every depth, trailing JSON, and scalar
// roots before schema validation. UseNumber preserves integer reference sizes.
func decodeObject(b []byte, schema string) (map[string]any, error) {
	if !utf8.Valid(b) {
		return nil, errors.New("invalid UTF-8")
	}
	d := json.NewDecoder(strings.NewReader(string(b)))
	d.UseNumber()
	v, e := uniqueValue(d, 0)
	if e != nil {
		return nil, e
	}
	if _, e = d.Token(); e != io.EOF {
		return nil, errors.New("trailing JSON")
	}
	m, ok := v.(map[string]any)
	if !ok || contracts.Validate(schema, m) != nil {
		return nil, errors.New("invalid object")
	}
	return m, nil
}
func uniqueValue(d *json.Decoder, depth int) (any, error) {
	if depth > 64 {
		return nil, errors.New("JSON nesting limit exceeded")
	}
	token, e := d.Token()
	if e != nil {
		return nil, e
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return token, nil
	}
	switch delim {
	case '{':
		m := map[string]any{}
		for d.More() {
			key, e := d.Token()
			if e != nil {
				return nil, e
			}
			name, ok := key.(string)
			if !ok {
				return nil, errors.New("invalid key")
			}
			if _, ok = m[name]; ok {
				return nil, errors.New("duplicate key")
			}
			v, e := uniqueValue(d, depth+1)
			if e != nil {
				return nil, e
			}
			m[name] = v
		}
		end, e := d.Token()
		if e != nil || end != json.Delim('}') {
			return nil, errors.New("invalid object")
		}
		return m, nil
	case '[':
		a := []any{}
		for d.More() {
			v, e := uniqueValue(d, depth+1)
			if e != nil {
				return nil, e
			}
			a = append(a, v)
		}
		end, e := d.Token()
		if e != nil || end != json.Delim(']') {
			return nil, errors.New("invalid array")
		}
		return a, nil
	default:
		return nil, errors.New("invalid JSON delimiter")
	}
}

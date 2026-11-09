// Package policy authorizes verified identities against trusted resource
// metadata. Caller-supplied owner fields never become authorization evidence.
package policy

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/yu-min3/kensan-lab/apps/llmops/contracts"
	"github.com/yu-min3/kensan-lab/apps/llmops/internal/identity"
)

var (
	ErrForbidden   = errors.New("operation forbidden")
	ErrNotFound    = errors.New("resource not found")
	ErrUnavailable = errors.New("authorization metadata unavailable")
)

const (
	CaptureCreate  = "capture:create"
	CaptureRead    = "capture:read"
	FeedbackCreate = "feedback:create"
	JobCreate      = "job:create"
	JobList        = "job:list"
	JobRead        = "job:read"
	JobCancel      = "job:cancel"
	JobRetry       = "job:retry"
	ArtifactRead   = "artifact:read"
)

var idPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,127}$`)
var appPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)

// Resource is returned only by the trusted metadata store adapter, never decoded
// from an incoming request. EvalReadable denotes approved dataset provenance.
type Resource struct {
	ID, Type, AppID, OwnerSubject, OwnerRole, Kind string
	EvalReadable                                   bool
}
type Resolver interface {
	Lookup(context.Context, identity.Principal, string, string) (Resource, error)
}
type Scope struct {
	AppID   string `json:"app_id"`
	Role    string `json:"role"`
	Subject string `json:"subject"`
	Kind    string `json:"kind,omitempty"`
}
type Config struct {
	Resolver  Resolver
	CursorKey []byte
	CursorTTL time.Duration
	Now       func() time.Time
}
type Engine struct {
	resolver  Resolver
	cursorKey []byte
	cursorTTL time.Duration
	now       func() time.Time
}

func New(c Config) (*Engine, error) {
	if c.Resolver == nil || len(c.CursorKey) < 32 {
		return nil, errors.New("trusted resolver and cursor signing key required")
	}
	if c.CursorTTL == 0 {
		c.CursorTTL = 15 * time.Minute
	}
	if c.CursorTTL < time.Second || c.CursorTTL > time.Hour {
		return nil, errors.New("invalid cursor TTL")
	}
	if c.Now == nil {
		c.Now = time.Now
	}
	return &Engine{c.Resolver, append([]byte(nil), c.CursorKey...), c.CursorTTL, c.Now}, nil
}
func verified(p identity.Principal) bool {
	return idPattern.MatchString(p.ClientID) && appPattern.MatchString(p.AppID) && p.Subject != "" && len(p.Subject) <= 256 && p.PolicyVersion != "" && (p.Role == "app" || p.Role == "eval" || p.Role == "transform")
}
func principalScope(p identity.Principal) Scope {
	return Scope{AppID: p.AppID, Role: p.Role, Subject: p.Subject}
}
func jobKind(p identity.Principal) string {
	if p.Role == "app" {
		return "online"
	}
	if p.Role == "eval" {
		return "eval"
	}
	return ""
}

// ListScope must be applied to the store query BEFORE pagination or totals.
// The handler must not fetch all apps and filter the serialized response later.
func (e *Engine) ListScope(p identity.Principal, action string) (Scope, error) {
	if !verified(p) || action != JobList || jobKind(p) == "" {
		return Scope{}, ErrForbidden
	}
	s := principalScope(p)
	s.Kind = jobKind(p)
	return s, nil
}

// AuthorizeCreate validates the request schema and referenced resources. The
// returned scope supplies persisted owner fields; the request body does not.
func (e *Engine) AuthorizeCreate(ctx context.Context, p identity.Principal, action string, body map[string]any) (Scope, error) {
	if !verified(p) {
		return Scope{}, ErrForbidden
	}
	if app, ok := body["app_id"]; ok && app != p.AppID {
		return Scope{}, ErrForbidden
	}
	schema := ""
	switch action {
	case CaptureCreate:
		if p.Role != "app" {
			return Scope{}, ErrForbidden
		}
		schema = "capture-metadata"
	case FeedbackCreate:
		if p.Role != "app" {
			return Scope{}, ErrForbidden
		}
		schema = "feedback-request"
	case JobCreate:
		if jobKind(p) == "" || p.RuntimeClient == "" {
			return Scope{}, ErrForbidden
		}
		schema = "job-request"
	default:
		return Scope{}, ErrForbidden
	}
	if contracts.Validate(schema, body) != nil {
		return Scope{}, ErrForbidden
	}
	scope := principalScope(p)
	if action == JobCreate {
		scope.Kind = jobKind(p)
		if body["kind"] != scope.Kind || body["profile"] != "text" {
			return Scope{}, ErrForbidden
		}
	}
	if action == FeedbackCreate {
		for field, check := range map[string]string{"capture_id": CaptureRead, "job_id": JobRead} {
			if raw, ok := body[field]; ok {
				id, valid := raw.(string)
				if !valid {
					return Scope{}, ErrForbidden
				}
				if _, err := e.AuthorizeResource(ctx, p, check, id); err != nil {
					return Scope{}, err
				}
			}
		}
		for _, field := range []string{"before", "after"} {
			if raw, ok := body[field]; ok {
				ref, valid := raw.(map[string]any)
				if !valid {
					return Scope{}, ErrForbidden
				}
				if ref["app_id"] != p.AppID {
					return Scope{}, ErrNotFound
				}
				if _, err := e.AuthorizeResource(ctx, p, ArtifactRead, ref["object_id"].(string)); err != nil {
					return Scope{}, err
				}
			}
		}
	}
	return scope, nil
}

// AuthorizeResource resolves metadata internally. Unknown or scope-hidden IDs
// return the same 404-class result; unsupported operations return 403-class.
func (e *Engine) AuthorizeResource(ctx context.Context, p identity.Principal, action, id string) (Resource, error) {
	if !verified(p) {
		return Resource{}, ErrForbidden
	}
	typ := ""
	switch action {
	case CaptureRead:
		typ = "capture"
	case JobRead, JobCancel, JobRetry:
		if jobKind(p) == "" {
			return Resource{}, ErrForbidden
		}
		typ = "job"
	case ArtifactRead:
		if jobKind(p) == "" {
			return Resource{}, ErrForbidden
		}
		typ = "artifact"
	default:
		return Resource{}, ErrForbidden
	}
	if !idPattern.MatchString(id) {
		return Resource{}, ErrNotFound
	}
	r, err := e.resolver.Lookup(ctx, p, typ, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return Resource{}, ErrNotFound
		}
		return Resource{}, ErrUnavailable
	}
	if r.ID != id || r.Type != typ || r.AppID != p.AppID {
		return Resource{}, ErrNotFound
	}
	if typ == "capture" {
		if p.Role == "transform" {
			return r, nil
		}
		if p.Role == "eval" && r.EvalReadable {
			return r, nil
		}
		if p.Role == "app" && r.OwnerRole == "app" && r.OwnerSubject == p.Subject {
			return r, nil
		}
		return Resource{}, ErrNotFound
	}
	if r.Kind != jobKind(p) || r.OwnerRole != p.Role || r.OwnerSubject != p.Subject {
		return Resource{}, ErrNotFound
	}
	return r, nil
}

type cursor struct {
	Scope   Scope  `json:"scope"`
	After   string `json:"after"`
	Expires int64  `json:"expires"`
}

// Cursor signs a scope-bound pagination position. Its contents are not encrypted.
func (e *Engine) Cursor(p identity.Principal, after string) (string, error) {
	scope, err := e.ListScope(p, JobList)
	if err != nil {
		return "", err
	}
	if !idPattern.MatchString(after) {
		return "", ErrNotFound
	}
	raw, _ := json.Marshal(cursor{scope, after, e.now().Add(e.cursorTTL).Unix()})
	tag := hmac.New(sha256.New, e.cursorKey)
	tag.Write(raw)
	return base64.RawURLEncoding.EncodeToString(raw) + "." + base64.RawURLEncoding.EncodeToString(tag.Sum(nil)), nil
}
func (e *Engine) ReadCursor(p identity.Principal, token string) (string, error) {
	scope, err := e.ListScope(p, JobList)
	if err != nil {
		return "", err
	}
	if len(token) > 4096 {
		return "", ErrNotFound
	}
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return "", ErrNotFound
	}
	raw, a := base64.RawURLEncoding.DecodeString(parts[0])
	mac, b := base64.RawURLEncoding.DecodeString(parts[1])
	if a != nil || b != nil {
		return "", ErrNotFound
	}
	tag := hmac.New(sha256.New, e.cursorKey)
	tag.Write(raw)
	if !hmac.Equal(mac, tag.Sum(nil)) {
		return "", ErrNotFound
	}
	var c cursor
	if json.Unmarshal(raw, &c) != nil || c.Scope != scope || c.Expires <= e.now().Unix() || !idPattern.MatchString(c.After) {
		return "", ErrNotFound
	}
	return c.After, nil
}

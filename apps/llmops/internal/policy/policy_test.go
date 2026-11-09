package policy

import (
	"context"
	"errors"
	"github.com/yu-min3/kensan-lab/apps/llmops/internal/identity"
	"strings"
	"testing"
	"time"
)

type store struct {
	resources map[string]Resource
	calls     int
	fail      bool
}

func (s *store) Lookup(_ context.Context, _ identity.Principal, typ, id string) (Resource, error) {
	s.calls++
	if s.fail {
		return Resource{}, errors.New("private storage details")
	}
	r, ok := s.resources[typ+":"+id]
	if !ok {
		return Resource{}, ErrNotFound
	}
	return r, nil
}
func actor(role string) identity.Principal {
	return identity.Principal{ClientID: "konro-" + role, AppID: "konro", Role: role, Subject: "konro-" + role + "-subject", RuntimeClient: "runtime-konro-" + role, PolicyVersion: "v1"}
}
func fixture(t *testing.T) (*Engine, *store, *time.Time) {
	t.Helper()
	s := &store{resources: map[string]Resource{
		"job:online-one":         {ID: "online-one", Type: "job", AppID: "konro", OwnerSubject: "konro-app-subject", OwnerRole: "app", Kind: "online"},
		"job:eval-one":           {ID: "eval-one", Type: "job", AppID: "konro", OwnerSubject: "konro-eval-subject", OwnerRole: "eval", Kind: "eval"},
		"job:sample-one":         {ID: "sample-one", Type: "job", AppID: "sample", OwnerSubject: "konro-app-subject", OwnerRole: "app", Kind: "online"},
		"job:other-user":         {ID: "other-user", Type: "job", AppID: "konro", OwnerSubject: "other-user", OwnerRole: "app", Kind: "online"},
		"capture:raw-one":        {ID: "raw-one", Type: "capture", AppID: "konro", OwnerSubject: "konro-app-subject", OwnerRole: "app"},
		"capture:raw-approved":   {ID: "raw-approved", Type: "capture", AppID: "konro", OwnerSubject: "konro-app-subject", OwnerRole: "app", EvalReadable: true},
		"capture:sample-raw":     {ID: "sample-raw", Type: "capture", AppID: "sample", OwnerSubject: "konro-app-subject", OwnerRole: "app"},
		"artifact:result-one":    {ID: "result-one", Type: "artifact", AppID: "konro", OwnerSubject: "konro-app-subject", OwnerRole: "app", Kind: "online"},
		"artifact:sample-result": {ID: "sample-result", Type: "artifact", AppID: "sample", OwnerSubject: "konro-app-subject", OwnerRole: "app", Kind: "online"},
	}}
	now := time.Unix(1800000000, 0)
	e, err := New(Config{Resolver: s, CursorKey: []byte(strings.Repeat("k", 32)), Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	return e, s, &now
}
func TestResourceOwnershipForEveryAction(t *testing.T) {
	e, s, _ := fixture(t)
	ctx := context.Background()
	for _, action := range []string{JobRead, JobCancel, JobRetry} {
		for _, tc := range []struct {
			role, id string
			allowed  bool
		}{{"app", "online-one", true}, {"eval", "eval-one", true}, {"app", "eval-one", false}, {"eval", "online-one", false}, {"app", "sample-one", false}, {"app", "other-user", false}, {"app", "missing", false}} {
			_, err := e.AuthorizeResource(ctx, actor(tc.role), action, tc.id)
			if (err == nil) != tc.allowed {
				t.Fatalf("%s %s %s: %v", action, tc.role, tc.id, err)
			}
			if !tc.allowed && !errors.Is(err, ErrNotFound) {
				t.Fatal("hidden resource must be 404", err)
			}
		}
	}
	for _, tc := range []struct {
		role, action, id string
		allowed          bool
	}{{"app", ArtifactRead, "result-one", true}, {"app", ArtifactRead, "sample-result", false}, {"eval", ArtifactRead, "result-one", false}, {"transform", CaptureRead, "raw-one", true}, {"transform", CaptureRead, "sample-raw", false}, {"eval", CaptureRead, "raw-one", false}, {"eval", CaptureRead, "raw-approved", true}} {
		_, err := e.AuthorizeResource(ctx, actor(tc.role), tc.action, tc.id)
		if (err == nil) != tc.allowed {
			t.Fatalf("%+v err=%v", tc, err)
		}
	}
	calls := s.calls
	if _, err := e.AuthorizeResource(ctx, actor("app"), JobRead, "../../sample-one"); !errors.Is(err, ErrNotFound) || s.calls != calls {
		t.Fatal("invalid ID reached store")
	}
	if _, err := e.AuthorizeResource(ctx, actor("app"), "release:promote", "online-one"); !errors.Is(err, ErrForbidden) {
		t.Fatal("unknown operation allowed")
	}
	s.fail = true
	if _, err := e.AuthorizeResource(ctx, actor("app"), JobRead, "online-one"); !errors.Is(err, ErrUnavailable) || strings.Contains(err.Error(), "private") {
		t.Fatal("storage error leaked", err)
	}
}
func TestCreateUsesVerifiedIdentityAndStrictSchema(t *testing.T) {
	e, _, _ := fixture(t)
	ctx := context.Background()
	job := func(kind string) map[string]any {
		return map[string]any{"profile": "text", "kind": kind, "prompt": "extract recipe", "input": map[string]any{"text": "captured"}, "output_schema": map[string]any{}}
	}
	for _, tc := range []struct {
		role, kind string
		allowed    bool
	}{{"app", "online", true}, {"app", "eval", false}, {"eval", "eval", true}, {"eval", "online", false}, {"transform", "online", false}} {
		scope, err := e.AuthorizeCreate(ctx, actor(tc.role), JobCreate, job(tc.kind))
		if (err == nil) != tc.allowed {
			t.Fatalf("%+v %v", tc, err)
		}
		if tc.allowed && (scope.AppID != "konro" || scope.Subject != "konro-"+tc.role+"-subject" || scope.Kind != tc.kind) {
			t.Fatalf("bad persisted scope %+v", scope)
		}
	}
	for _, override := range []string{"owner", "runtime_client", "credentials", "prefix", "bucket"} {
		body := job("online")
		body[override] = "admin"
		if _, err := e.AuthorizeCreate(ctx, actor("app"), JobCreate, body); !errors.Is(err, ErrForbidden) {
			t.Fatal("override accepted", override)
		}
	}
	body := job("online")
	body["app_id"] = "sample"
	if _, err := e.AuthorizeCreate(ctx, actor("app"), JobCreate, body); !errors.Is(err, ErrForbidden) {
		t.Fatal("app spoofing allowed")
	}
	body = job("online")
	body["app_id"] = "konro"
	if _, err := e.AuthorizeCreate(ctx, actor("app"), JobCreate, body); err != nil {
		t.Fatal("matching redundant app rejected", err)
	}
	body = job("online")
	body["profile"] = "shell"
	if _, err := e.AuthorizeCreate(ctx, actor("app"), JobCreate, body); err == nil {
		t.Fatal("shell profile allowed")
	}
	capture := map[string]any{"capture_key": "capture-1", "mime": "text/plain", "sha256": strings.Repeat("a", 64)}
	if _, err := e.AuthorizeCreate(ctx, actor("app"), CaptureCreate, capture); err != nil {
		t.Fatal(err)
	}
	if _, err := e.AuthorizeCreate(ctx, actor("eval"), CaptureCreate, capture); !errors.Is(err, ErrForbidden) {
		t.Fatal("eval wrote raw")
	}
}
func TestFeedbackResolvesAllParents(t *testing.T) {
	e, _, _ := fixture(t)
	ctx := context.Background()
	body := map[string]any{"event_id": "event-1", "reason": "extraction_error", "capture_id": "raw-one", "job_id": "online-one"}
	if _, err := e.AuthorizeCreate(ctx, actor("app"), FeedbackCreate, body); err != nil {
		t.Fatal(err)
	}
	body["job_id"] = "sample-one"
	if _, err := e.AuthorizeCreate(ctx, actor("app"), FeedbackCreate, body); !errors.Is(err, ErrNotFound) {
		t.Fatal("foreign parent allowed")
	}
	delete(body, "job_id")
	body["after"] = map[string]any{"app_id": "konro", "object_id": "sample-result", "size": 1, "sha256": strings.Repeat("b", 64)}
	if _, err := e.AuthorizeCreate(ctx, actor("app"), FeedbackCreate, body); !errors.Is(err, ErrNotFound) {
		t.Fatal("forged object owner accepted")
	}
	body["after"].(map[string]any)["object_id"] = "result-one"
	if _, err := e.AuthorizeCreate(ctx, actor("app"), FeedbackCreate, body); err != nil {
		t.Fatal(err)
	}
	if _, err := e.AuthorizeCreate(ctx, actor("eval"), FeedbackCreate, body); !errors.Is(err, ErrForbidden) {
		t.Fatal("eval created feedback")
	}
}
func TestListAndCursorCannotCrossScopes(t *testing.T) {
	e, _, now := fixture(t)
	scope, err := e.ListScope(actor("app"), JobList)
	if err != nil || scope != (Scope{AppID: "konro", Role: "app", Subject: "konro-app-subject", Kind: "online"}) {
		t.Fatal(scope, err)
	}
	if _, err := e.ListScope(actor("transform"), JobList); !errors.Is(err, ErrForbidden) {
		t.Fatal("transform listed jobs")
	}
	token, err := e.Cursor(actor("app"), "online-one")
	if err != nil {
		t.Fatal(err)
	}
	if after, err := e.ReadCursor(actor("app"), token); err != nil || after != "online-one" {
		t.Fatal(after, err)
	}
	for _, p := range []identity.Principal{actor("eval"), {ClientID: "sample-app", AppID: "sample", Role: "app", Subject: "konro-app-subject", PolicyVersion: "v1"}, {ClientID: "konro-app", AppID: "konro", Role: "app", Subject: "other-user", PolicyVersion: "v1"}} {
		if _, err := e.ReadCursor(p, token); !errors.Is(err, ErrNotFound) {
			t.Fatal("foreign cursor accepted")
		}
	}
	if _, err := e.ReadCursor(actor("app"), token+"x"); !errors.Is(err, ErrNotFound) {
		t.Fatal("tampered cursor accepted")
	}
	*now = now.Add(15 * time.Minute)
	if _, err := e.ReadCursor(actor("app"), token); !errors.Is(err, ErrNotFound) {
		t.Fatal("expired cursor accepted")
	}
}

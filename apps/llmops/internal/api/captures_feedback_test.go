package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"sync"
	"testing"

	"github.com/yu-min3/kensan-lab/apps/llmops/internal/identity"
	"github.com/yu-min3/kensan-lab/apps/llmops/internal/objects"
	"github.com/yu-min3/kensan-lab/apps/llmops/internal/policy"
)

type testBackend struct {
	mu    sync.Mutex
	app   string
	items map[string]objects.Blob
	puts  int
}

func (b *testBackend) AppID() string { return b.app }
func (b *testBackend) PutIfAbsent(_ context.Context, key string, raw []byte, mime string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.puts++
	if _, ok := b.items[key]; ok {
		return objects.ErrExists
	}
	b.items[key] = objects.Blob{Bytes: append([]byte(nil), raw...), MIME: mime}
	return nil
}
func (b *testBackend) Get(_ context.Context, key string, max int64) (objects.Blob, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	blob, ok := b.items[key]
	if !ok {
		return objects.Blob{}, objects.ErrNotFound
	}
	if int64(len(blob.Bytes)) > max {
		return objects.Blob{}, objects.ErrTooLarge
	}
	blob.Bytes = append([]byte(nil), blob.Bytes...)
	return blob, nil
}

type testAuth map[string]identity.Principal

func (a testAuth) Verify(_ context.Context, token string) (identity.Principal, error) {
	p, ok := a[token]
	if !ok {
		return identity.Principal{}, errors.New("bad token")
	}
	return p, nil
}
func fixtureAPI(t *testing.T) (http.Handler, map[string]*objects.Store, *testBackend) {
	t.Helper()
	stores := map[string]*objects.Store{}
	backends := map[string]*testBackend{}
	for _, app := range []string{"example", "other"} {
		b := &testBackend{app: app, items: map[string]objects.Blob{}}
		s, e := objects.NewStore(app, b)
		if e != nil {
			t.Fatal(e)
		}
		stores[app] = s
		backends[app] = b
	}
	auth := testAuth{"app": {ClientID: "example-app", AppID: "example", Role: "app", Subject: "sub-one", PolicyVersion: "v1"}, "other": {ClientID: "other-app", AppID: "other", Role: "app", Subject: "sub-other", PolicyVersion: "v1"}, "eval": {ClientID: "example-eval", AppID: "example", Role: "eval", Subject: "sub-eval", PolicyVersion: "v1"}, "second": {ClientID: "second-app", AppID: "example", Role: "app", Subject: "sub-second", PolicyVersion: "v1"}}
	engine, e := policy.New(policy.Config{Resolver: &StorageResolver{Stores: stores}, CursorKey: bytes.Repeat([]byte("x"), 32)})
	if e != nil {
		t.Fatal(e)
	}
	h, e := NewCaptureFeedback(CaptureFeedbackConfig{Auth: auth, Policy: engine, Stores: stores})
	if e != nil {
		t.Fatal(e)
	}
	mux := http.NewServeMux()
	h.Register(mux)
	return mux, stores, backends["example"]
}
func multipartRequest(t *testing.T, token string, meta, source []byte, extraName string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", `form-data; name="metadata"`)
	header.Set("Content-Type", "application/json")
	part, e := w.CreatePart(header)
	if e != nil {
		t.Fatal(e)
	}
	part.Write(meta)
	part, e = w.CreateFormFile("source", "raw.txt")
	if e != nil {
		t.Fatal(e)
	}
	part.Write(source)
	if extraName != "" {
		part, e = w.CreateFormField(extraName)
		if e != nil {
			t.Fatal(e)
		}
		part.Write([]byte("extra"))
	}
	w.Close()
	r := httptest.NewRequest("POST", "/v1/captures", &buf)
	r.Header.Set("Content-Type", w.FormDataContentType())
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	return r
}
func metadata(key string, source []byte) []byte {
	b, _ := json.Marshal(map[string]any{"capture_key": key, "mime": "text/plain", "sha256": hashBytes(source)})
	return b
}
func captureFor(t *testing.T, h http.Handler, token, key string) objects.Capture {
	t.Helper()
	source := []byte("玉ねぎ 1/2個\r\n")
	r := multipartRequest(t, token, metadata(key, source), source, "")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 202 {
		t.Fatalf("capture status %d %s", w.Code, w.Body.String())
	}
	var c objects.Capture
	if e := json.Unmarshal(w.Body.Bytes(), &c); e != nil {
		t.Fatal(e)
	}
	return c
}
func jsonRequest(path, token, body string) *http.Request {
	r := httptest.NewRequest("POST", path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer "+token)
	return r
}
func TestCapturePreservesOriginalAndTrustedOwner(t *testing.T) {
	h, stores, _ := fixtureAPI(t)
	c := captureFor(t, h, "app", "one")
	again := captureFor(t, h, "app", "one")
	if c != again {
		t.Fatal("replay changed capture")
	}
	blob, e := stores["example"].GetArtifact(context.Background(), c.Object.ObjectID)
	if e != nil || string(blob.Bytes) != "玉ねぎ 1/2個\r\n" {
		t.Fatal("source changed")
	}
	subject, role, e := stores["example"].CaptureOwnership(context.Background(), c.CaptureID)
	if e != nil || subject != "sub-one" || role != "app" {
		t.Fatalf("owner %s %s %v", subject, role, e)
	}
}
func TestCaptureRejectsInvalidOrForgedRequest(t *testing.T) {
	h, _, backend := fixtureAPI(t)
	source := []byte("sample")
	meta := metadata("one", source)
	cases := []struct {
		name, token string
		meta        []byte
		extra       string
		status      int
	}{
		{"auth", "", meta, "", 401}, {"role", "eval", meta, "", 403},
		{"app-forgery", "app", []byte(`{"capture_key":"one","mime":"text/plain","sha256":"` + hashBytes(source) + `","app_id":"other"}`), "", 403},
		{"owner", "app", []byte(`{"capture_key":"one","mime":"text/plain","sha256":"` + hashBytes(source) + `","owner":"other"}`), "", 400},
		{"duplicate-json", "app", []byte(`{"capture_key":"one","capture_key":"two","mime":"text/plain","sha256":"` + hashBytes(source) + `"}`), "", 400},
		{"unknown-part", "app", meta, "admin", 400}, {"duplicate-source", "app", meta, "source", 400},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := multipartRequest(t, tc.token, tc.meta, source, tc.extra)
			r.Header.Set("X-Client-ID", "example-app")
			h.ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("got %d: %s", w.Code, w.Body.String())
			}
		})
	}
	if backend.puts != 0 {
		t.Fatal("rejected input wrote objects")
	}
	huge := bytes.Repeat([]byte("x"), int(objects.MaxRawBytes+1))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, multipartRequest(t, "app", metadata("huge", huge), huge, ""))
	if w.Code != 413 {
		t.Fatalf("oversize %d", w.Code)
	}
}
func TestFeedbackReplayOwnershipAndNoGoldPromotion(t *testing.T) {
	h, stores, _ := fixtureAPI(t)
	capture := captureFor(t, h, "app", "parent")
	other := captureFor(t, h, "other", "foreign")
	body := `{"event_id":"event-one","capture_id":"` + capture.CaptureID + `","reason":"annotation_proposal"}`
	post := func(token, body string) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		h.ServeHTTP(w, jsonRequest("/v1/feedback", token, body))
		return w
	}
	first := post("app", body)
	if first.Code != 202 {
		t.Fatalf("first %d %s", first.Code, first.Body.String())
	}
	second := post("app", body)
	if second.Code != 202 || second.Body.String() != first.Body.String() {
		t.Fatal("replay changed feedback")
	}
	var data map[string]any
	json.Unmarshal(first.Body.Bytes(), &data)
	if data["annotation_status"] != "unreviewed" || data["app_id"] != "example" {
		t.Fatal("untrusted status/owner")
	}
	published, e := stores["example"].GetRecord(context.Background(), "feedback", "event-one")
	if e != nil || !bytes.Equal(published.Data, bytes.TrimSpace(first.Body.Bytes())) {
		t.Fatalf("feedback not in S3 records: %v", e)
	}
	if w := post("app", strings.Replace(body, "annotation_proposal", "preference_change", 1)); w.Code != 409 {
		t.Fatalf("conflict %d", w.Code)
	}
	for _, tc := range []struct {
		token, body string
		status      int
	}{
		{"eval", body, 403}, {"second", body, 404},
		{"app", `{"event_id":"foreign","capture_id":"` + other.CaptureID + `","reason":"annotation_proposal"}`, 404},
		{"app", strings.TrimSuffix(body, "}") + `,"annotation_status":"reviewed"}`, 400},
		{"app", strings.TrimSuffix(body, "}") + `,"runtime_client":"admin"}`, 400},
		{"app", `{"event_id":"nested","capture_id":"` + capture.CaptureID + `","reason":"annotation_proposal","before":{"object_id":"x","object_id":"y","app_id":"example","sha256":"` + hashBytes(nil) + `","size":0}}`, 400},
	} {
		w := post(tc.token, tc.body)
		if w.Code != tc.status {
			t.Fatalf("got %d want %d body=%s", w.Code, tc.status, w.Body.String())
		}
	}
}
func TestFeedbackReferenceHashIsVerified(t *testing.T) {
	h, _, _ := fixtureAPI(t)
	capture := captureFor(t, h, "app", "ref")
	ref := capture.Object
	ref.SHA256 = hashBytes([]byte("wrong"))
	body, _ := json.Marshal(map[string]any{"event_id": "bad-ref", "capture_id": capture.CaptureID, "reason": "annotation_proposal", "before": ref})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, jsonRequest("/v1/feedback", "app", string(body)))
	if w.Code != 400 {
		t.Fatalf("bad ref %d %s", w.Code, w.Body.String())
	}
}
func TestNoIdentityHeaderShortcut(t *testing.T) {
	h, _, _ := fixtureAPI(t)
	r := httptest.NewRequest("POST", "/v1/feedback", io.NopCloser(strings.NewReader(`{}`)))
	r.Header.Set("X-Client-ID", "example-app")
	r.Header.Set("X-App-ID", "example")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatal("identity headers accepted")
	}
}

func TestValidFeedbackByteReference(t *testing.T) {
	h, _, _ := fixtureAPI(t)
	c := captureFor(t, h, "app", "valid-ref")
	body, _ := json.Marshal(map[string]any{"event_id": "valid-ref-event", "capture_id": c.CaptureID, "reason": "annotation_proposal", "before": c.Object})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, jsonRequest("/v1/feedback", "app", string(body)))
	if w.Code != 202 {
		t.Fatalf("valid reference %d %s", w.Code, w.Body.String())
	}
}

package publisherbridge

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/core"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type fakeTransport struct{ calls atomic.Int32 }

func (f *fakeTransport) Inspect(context.Context, core.PublishIntent) (string, bool, error) {
	return "", false, nil
}
func (f *fakeTransport) Execute(context.Context, core.PublishIntent) (string, error) {
	f.calls.Add(1)
	return strings.Repeat("b", 40), nil
}
func bridgeAuth(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "auth")
	if err := os.WriteFile(p, []byte(strings.Repeat("x", 40)), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}
func bridgeIntent() core.PublishIntent {
	return core.PublishIntent{ID: "intent", DecisionID: "gate-decision", Operation: "merge", Repository: "yu-min3/kensan-lab", Ref: "refs/heads/app-canary-feature", HeadSHA: strings.Repeat("a", 40), PolicyVersion: core.ReleasePolicyVersion, TargetEnvironment: "private-canary", ExpiresAt: time.Now().Add(time.Minute)}
}
func TestBridgeRejectsUnauthorizedAndExpiredBeforeTransport(t *testing.T) {
	f := &fakeTransport{}
	h, err := Handler(bridgeAuth(t), f)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, token, action string
		modify              func(*core.PublishIntent)
	}{
		{"missing auth", "", "execute", nil}, {"wrong auth", strings.Repeat("y", 40), "execute", nil},
		{"expired", strings.Repeat("x", 40), "execute", func(i *core.PublishIntent) { i.ExpiresAt = time.Now().Add(-time.Second) }},
		{"missing expiry", strings.Repeat("x", 40), "execute", func(i *core.PublishIntent) { i.ExpiresAt = time.Time{} }},
		{"public env", strings.Repeat("x", 40), "execute", func(i *core.PublishIntent) { i.TargetEnvironment = "public" }},
		{"wrong repo", strings.Repeat("x", 40), "execute", func(i *core.PublishIntent) { i.Repository = "other/repo" }},
		{"wrong operation", strings.Repeat("x", 40), "execute", func(i *core.PublishIntent) { i.Operation = "delete" }},
		{"old policy", strings.Repeat("x", 40), "execute", func(i *core.PublishIntent) { i.PolicyVersion = "old" }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			i := bridgeIntent()
			if c.modify != nil {
				c.modify(&i)
			}
			b, _ := json.Marshal(request{Action: c.action, Intent: i})
			r := httptest.NewRequest(http.MethodPost, "/v1/publish", bytes.NewReader(b))
			r.Header.Set("Authorization", "Bearer "+c.token)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code == http.StatusOK {
				t.Fatal("invalid request accepted")
			}
		})
	}
	if f.calls.Load() != 0 {
		t.Fatal("rejected requests reached transport")
	}
	i := bridgeIntent()
	i.ExpiresAt = time.Now().Add(-time.Second)
	b, _ := json.Marshal(request{Action: "inspect", Intent: i})
	r := httptest.NewRequest(http.MethodPost, "/v1/publish", bytes.NewReader(b))
	r.Header.Set("Authorization", "Bearer "+strings.Repeat("x", 40))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal("expired inspection unavailable")
	}
}
func TestUnixClientRoundTripWithoutOpeningLedger(t *testing.T) {
	t.Setenv("TMPDIR", "/private/tmp")
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	socket := filepath.Join(dir, "publisher.sock")
	auth := bridgeAuth(t)
	f := &fakeTransport{}
	h, err := Handler(auth, f)
	if err != nil {
		t.Fatal(err)
	}
	l, err := Listen(socket, -1)
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: h}
	go server.Serve(l)
	defer server.Close()
	client := Client{Socket: socket, AuthFile: auth}
	i := bridgeIntent()
	if _, exists, err := client.Inspect(context.Background(), i); err != nil || exists {
		t.Fatalf("inspect %v %v", exists, err)
	}
	id, err := client.Execute(context.Background(), i)
	if err != nil || id != strings.Repeat("b", 40) || f.calls.Load() != 1 {
		t.Fatalf("execute %s %v", id, err)
	}
	if _, err := Listen(socket, -1); err == nil {
		t.Fatal("existing daemon socket replaced")
	}
}
func TestBridgeRejectsWeakPermissions(t *testing.T) {
	p := bridgeAuth(t)
	if err := os.Chmod(p, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Handler(p, &fakeTransport{}); err == nil {
		t.Fatal("public auth accepted")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0770); err != nil {
		t.Fatal(err)
	}
	if _, err := Listen(filepath.Join(dir, "bridge.sock"), -1); err == nil {
		t.Fatal("writable socket directory accepted")
	}
}

type mergedTransport struct {
	fakeTransport
	revision string
}

func (m *mergedTransport) Inspect(context.Context, core.PublishIntent) (string, bool, error) {
	return m.revision, true, nil
}
func TestObservationRequiresAuthenticatedSentMatchingMerge(t *testing.T) {
	m := &mergedTransport{revision: strings.Repeat("b", 40)}
	var observations atomic.Int32
	h, err := HandlerWithObserver(bridgeAuth(t), m, func(_ context.Context, head, rev string, spec core.ImageDeploymentSpec) (core.DeploymentReceipt, error) {
		observations.Add(1)
		return core.DeploymentReceipt{HeadSHA: head, Revision: rev}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, valid := range []bool{false, true} {
		i := bridgeIntent()
		i.Status = "sent"
		i.ImageDeployment = validBridgeImageDeployment()
		i.ExternalID = m.revision
		if !valid {
			i.ExternalID = strings.Repeat("c", 40)
		}
		b, _ := json.Marshal(request{Action: "observe", Intent: i, TaskID: "task"})
		r := httptest.NewRequest(http.MethodPost, "/v1/publish", bytes.NewReader(b))
		r.Header.Set("Authorization", "Bearer "+strings.Repeat("x", 40))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if valid && w.Code != 200 || !valid && w.Code == 200 {
			t.Fatalf("valid=%v status=%d", valid, w.Code)
		}
	}
	if observations.Load() != 1 || m.calls.Load() != 0 {
		t.Fatal("observation bypassed merge inspection or invoked mutator")
	}
}

package jobs

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func admission(key string) Admission {
	return Admission{Owner: Owner{App: "example", Role: "app", Subject: "example-app"}, Key: key, Kind: "online", Payload: json.RawMessage(`{"text":"sample","nested":{"a":1}}`), RuntimeClient: "runtime-example-online", RuntimeTarget: "local-runtime"}
}
func openTest(t *testing.T) (*Store, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "jobs")
	s, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	return s, dir
}
func TestLockAndPrivateFiles(t *testing.T) {
	s, dir := openTest(t)
	if second, e := Open(dir); !errors.Is(e, ErrLocked) {
		if second != nil {
			second.Close()
		}
		t.Fatalf("second writer: %v", e)
	}
	if _, _, e := s.Admit(admission("k1"), 1); e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"writer.lock", "snapshot.json"} {
		i, e := os.Stat(filepath.Join(dir, name))
		if e != nil {
			t.Fatal(e)
		}
		if i.Mode().Perm() != 0600 {
			t.Fatalf("%s permissions %v", name, i.Mode())
		}
	}
}
func TestAdmissionReplayBudgetAndScopes(t *testing.T) {
	s, _ := openTest(t)
	a := admission("same-key")
	r, reused, e := s.Admit(a, 1)
	if e != nil || reused {
		t.Fatalf("fresh %v %v", reused, e)
	}
	// Reordered object fields remain the same request.
	a.Payload = json.RawMessage(`{ "nested": {"a":1}, "text":"sample" }`)
	again, reused, e := s.Admit(a, 0)
	if e != nil || !reused || again.ID != r.ID {
		t.Fatalf("replay %v %v", reused, e)
	}
	changed := a
	changed.Payload = json.RawMessage(`{"text":"different"}`)
	if _, _, e = s.Admit(changed, 10); !errors.Is(e, ErrConflict) {
		t.Fatal(e)
	}
	changed = a
	changed.RuntimeClient = "another-client"
	if _, _, e = s.Admit(changed, 10); !errors.Is(e, ErrConflict) {
		t.Fatal(e)
	}
	changed = a
	changed.Key = "next"
	if _, _, e = s.Admit(changed, 1); !errors.Is(e, ErrQuota) {
		t.Fatal(e)
	}
	// Limits are app × kind, not owner × kind.
	changed.Owner.Subject = "second-client"
	if _, _, e = s.Admit(changed, 1); !errors.Is(e, ErrQuota) {
		t.Fatal(e)
	}
	changed.Kind = "eval"
	changed.Owner.Role = "eval"
	if _, _, e = s.Admit(changed, 1); e != nil {
		t.Fatal(e)
	}
	changed.Owner.App = "another"
	if _, _, e = s.Admit(changed, 1); e != nil {
		t.Fatal(e)
	}
	for _, owner := range []Owner{{App: "another", Role: "app", Subject: "example-app"}, {App: "example", Role: "eval", Subject: "example-app"}, {App: "example", Role: "app", Subject: "another-client"}} {
		if _, e = s.Get(owner, r.ID); !errors.Is(e, ErrNotFound) {
			t.Fatal(e)
		}
		rows, e := s.List(owner)
		if e != nil || len(rows) != 0 {
			t.Fatalf("owner leak: %v %v", rows, e)
		}
	}
	// Returned mutable buffers cannot alter durable request state.
	r.Admission.Payload[0] = 'x'
	stored, e := s.Get(a.Owner, r.ID)
	if e != nil || stored.Admission.Payload[0] != '{' {
		t.Fatal("payload aliased")
	}
}
func TestConcurrentBudgetReservation(t *testing.T) {
	s, _ := openTest(t)
	var accepted atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			a := admission(strconv.Itoa(i))
			_, _, e := s.Admit(a, 3)
			if e == nil {
				accepted.Add(1)
			} else if !errors.Is(e, ErrQuota) {
				t.Errorf("admission %v", e)
			}
		}(i)
	}
	wg.Wait()
	if accepted.Load() != 3 {
		t.Fatalf("accepted %d", accepted.Load())
	}
}
func TestBudgetPersistsAcrossRestartAndResetsUTC(t *testing.T) {
	s, dir := openTest(t)
	now := time.Date(2026, 1, 1, 23, 59, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	a := admission("k1")
	if _, _, e := s.Admit(a, 1); e != nil {
		t.Fatal(e)
	}
	s.Close()
	next, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer next.Close()
	next.now = func() time.Time { return now }
	a.Key = "k2"
	if _, _, e = next.Admit(a, 1); !errors.Is(e, ErrQuota) {
		t.Fatal(e)
	}
	now = now.Add(2 * time.Minute)
	if _, _, e = next.Admit(a, 1); e != nil {
		t.Fatal(e)
	}
}
func TestResponseLossRecoveryAndStableCorrelation(t *testing.T) {
	s, dir := openTest(t)
	a := admission("k1")
	r, _, e := s.Admit(a, 1)
	if e != nil {
		t.Fatal(e)
	}
	dispatched, e := s.BeginDispatch(a.Owner, r.ID)
	if e != nil {
		t.Fatal(e)
	}
	// Simulate runtime accepting Create and losing its response before Link.
	runtimeCreates := map[string]string{dispatched.DownstreamKey: "runtime-job-1"}
	s.Close()
	next, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer next.Close()
	pending, e := next.Pending()
	if e != nil || len(pending) != 1 || pending[0].Phase != "reconciling" {
		t.Fatalf("pending %v %v", pending, e)
	}
	restored := pending[0]
	if restored.DownstreamKey != dispatched.DownstreamKey || restored.Admission.RuntimeClient != a.RuntimeClient || restored.Admission.RuntimeTarget != a.RuntimeTarget || !bytes.Equal(restored.Admission.Payload, dispatched.Admission.Payload) {
		t.Fatal("recovery changed dispatch identity")
	}
	// An idempotent runtime returns the already accepted job for this key.
	runtimeID := runtimeCreates[restored.DownstreamKey]
	if runtimeID == "" {
		t.Fatal("duplicate Create would be required")
	}
	if _, e = next.Link(a.Owner, r.ID, runtimeID, "running", ""); e != nil {
		t.Fatal(e)
	}
	if _, e = next.Link(a.Owner, r.ID, "different-job", "running", ""); !errors.Is(e, ErrConflict) {
		t.Fatal(e)
	}
	if _, e = next.Link(a.Owner, r.ID, runtimeID, "failed", "interrupted"); e != nil {
		t.Fatal(e)
	}
	if _, e = next.Link(a.Owner, r.ID, runtimeID, "queued", ""); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	pending, e = next.Pending()
	if e != nil || len(pending) != 0 {
		t.Fatal("terminal job requeued")
	}
	again, reused, e := next.Admit(a, 0)
	if e != nil || !reused || again.ID != r.ID {
		t.Fatalf("replay after failure: %v", e)
	}
}
func TestFailedPersistDoesNotPublishTransition(t *testing.T) {
	s, dir := openTest(t)
	a := admission("k1")
	r, _, e := s.Admit(a, 1)
	if e != nil {
		t.Fatal(e)
	}
	before, e := os.ReadFile(filepath.Join(dir, "snapshot.json"))
	if e != nil {
		t.Fatal(e)
	}
	s.save = func([]Record) error { return syscall.ENOSPC }
	if _, e = s.BeginDispatch(a.Owner, r.ID); !errors.Is(e, ErrUnavailable) {
		t.Fatal(e)
	}
	if s.records[r.ID].Phase != "prepared" {
		t.Fatal("failed transition published in memory")
	}
	if _, e = s.Get(a.Owner, r.ID); !errors.Is(e, ErrUnavailable) {
		t.Fatal("failed store kept serving")
	}
	after, _ := os.ReadFile(filepath.Join(dir, "snapshot.json"))
	if !bytes.Equal(before, after) {
		t.Fatal("snapshot altered")
	}
	s.Close()
	reopened, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	rr, e := reopened.Get(a.Owner, r.ID)
	if e != nil || rr.Phase != "prepared" {
		t.Fatalf("recovered %v %v", rr, e)
	}
}
func TestFailedAdmissionDoesNotReserveQuota(t *testing.T) {
	s, dir := openTest(t)
	s.save = func([]Record) error { return syscall.ENOSPC }
	if _, _, e := s.Admit(admission("k1"), 1); !errors.Is(e, ErrUnavailable) {
		t.Fatal(e)
	}
	if len(s.records) != 0 {
		t.Fatal("reservation published")
	}
	s.Close()
	next, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer next.Close()
	if _, _, e = next.Admit(admission("k2"), 1); e != nil {
		t.Fatal(e)
	}
}
func TestCorruptAndInconsistentSnapshotRejected(t *testing.T) {
	for _, mutate := range []func([]byte) []byte{
		func([]byte) []byte { return []byte("not json") },
		func(b []byte) []byte { return bytes.Replace(b, []byte("sample"), []byte("tampered"), 1) },
		func(b []byte) []byte {
			var e envelope
			json.Unmarshal(b, &e)
			e.Records[0].DownstreamKey = "changed"
			e.SHA256 = checksum(e.Records)
			out, _ := json.Marshal(e)
			return out
		},
	} {
		s, dir := openTest(t)
		if _, _, e := s.Admit(admission("k1"), 1); e != nil {
			t.Fatal(e)
		}
		s.Close()
		path := filepath.Join(dir, "snapshot.json")
		b, _ := os.ReadFile(path)
		if e := os.WriteFile(path, mutate(b), 0600); e != nil {
			t.Fatal(e)
		}
		if next, e := Open(dir); e == nil {
			next.Close()
			t.Fatal("corrupt snapshot accepted")
		}
	}
}
func TestInvalidTransitionAndAdmission(t *testing.T) {
	s, _ := openTest(t)
	a := admission("k1")
	r, _, e := s.Admit(a, 1)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Link(a.Owner, r.ID, "runtime1", "running", ""); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	if _, e = s.BeginDispatch(Owner{App: "other"}, r.ID); !errors.Is(e, ErrNotFound) {
		t.Fatal(e)
	}
	a.Payload = json.RawMessage(`{} {}`)
	if _, _, e = s.Admit(a, 1); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	a = admission("k2")
	a.Kind = "admin"
	if _, _, e = s.Admit(a, 1); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
}

func TestCancelBeforeDispatch(t *testing.T) {
	s, dir := openTest(t)
	a := admission("cancel-me")
	r, _, e := s.Admit(a, 1)
	if e != nil {
		t.Fatal(e)
	}
	cancelled, e := s.CancelPrepared(a.Owner, r.ID)
	if e != nil || cancelled.State != "cancelled" {
		t.Fatal(e)
	}
	if _, e = s.BeginDispatch(a.Owner, r.ID); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	s.Close()
	next, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer next.Close()
	rows, e := next.Pending()
	if e != nil || len(rows) != 0 {
		t.Fatal("local cancelled job requeued")
	}
	if _, e = next.Link(a.Owner, r.ID, "unexpected-runtime-job", "cancelled", "cancelled_before_dispatch"); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
}

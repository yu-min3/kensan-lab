package dataset

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/yu-min3/kensan-lab/apps/llmops/internal/objects"
	"strings"
	"sync"
	"testing"
)

type memory struct {
	mu   sync.Mutex
	data map[string]objects.Blob
}

func (m *memory) AppID() string { return "konro" }
func (m *memory) PutIfAbsent(_ context.Context, k string, b []byte, mime string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.data[k]; ok {
		return objects.ErrExists
	}
	m.data[k] = objects.Blob{Bytes: append([]byte(nil), b...), MIME: mime}
	return nil
}
func (m *memory) Get(_ context.Context, k string, _ int64) (objects.Blob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.data[k]
	if !ok {
		return v, objects.ErrNotFound
	}
	return v, nil
}
func fixture() Spec {
	return Spec{ReleaseID: "release-a", TaskID: "recipe-import", SourceSnapshot: "123", CodeVersion: "fixture-v1", AdapterVersion: "konro-v1", Entries: []Entry{{RecordID: strings.Repeat("a", 64), Case: Case{SchemaVersion: "1", AppID: "konro", CaseID: "case-a", Input: objects.Reference{ObjectID: "raw_a", SHA256: strings.Repeat("b", 64), Size: 1, AppID: "konro"}, Expected: json.RawMessage(`{"required_ingredients":["onion 1/2"]}`), AnnotationVersion: "synthetic-v1", AnnotationStatus: "synthetic", Split: "development", SourceGroup: "group-a"}}}}
}
func TestSplitAndAnnotationValidation(t *testing.T) {
	s := fixture()
	if e := Validate("konro", s); e != nil {
		t.Fatal(e)
	}
	b := s.Entries[0]
	b.Case.CaseID = "case-b"
	b.Case.Split = "holdout"
	s.Entries = append(s.Entries, b)
	if Validate("konro", s) == nil {
		t.Fatal("source group leaked into holdout")
	}
	s = fixture()
	s.Entries[0].Case.AnnotationStatus = "assumed-correct"
	if Validate("konro", s) == nil {
		t.Fatal("unknown annotation status accepted")
	}
	s = fixture()
	s.Entries[0].Case.Input.AppID = "sample"
	if Validate("konro", s) == nil {
		t.Fatal("other app accepted")
	}
}
func TestReservationFirstWriterAndReplay(t *testing.T) {
	ctx := context.Background()
	m := &memory{data: map[string]objects.Blob{}}
	key := "request"
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, body := range []string{`{"a":1}`, `{"a":2}`} {
		wg.Add(1)
		go func(b string) { defer wg.Done(); results <- immutable(ctx, m, key, []byte(b)) }(body)
	}
	wg.Wait()
	close(results)
	success, conflicts := 0, 0
	for e := range results {
		if e == nil {
			success++
		} else if errors.Is(e, objects.ErrConflict) {
			conflicts++
		} else {
			t.Fatal(e)
		}
	}
	if success != 1 || conflicts != 1 {
		t.Fatal("different request race was not rejected")
	}
	winner, _ := m.Get(ctx, key, 100)
	if e := immutable(ctx, m, key, winner.Bytes); e != nil {
		t.Fatal("same reservation cannot resume")
	}
}
func TestRawRetentionGuardAndCanonicalHash(t *testing.T) {
	ctx := context.Background()
	m := &memory{data: map[string]objects.Blob{}}
	ref := fixture().Entries[0].Case.Input
	if e := GuardRawRemoval(ctx, m, ref); e != nil {
		t.Fatal(e)
	}
	if e := immutable(ctx, m, rawPin("konro", ref.ObjectID), canonical(ref)); e != nil {
		t.Fatal(e)
	}
	if !errors.Is(GuardRawRemoval(ctx, m, ref), ErrPinned) {
		t.Fatal("pinned Raw removable")
	}
	a := json.RawMessage(`{"b":2,"a":1}`)
	b := json.RawMessage(`{"a":1,"b":2}`)
	if digest(canonical(a)) != digest(canonical(b)) {
		t.Fatal("equivalent constraints hash differently")
	}
}

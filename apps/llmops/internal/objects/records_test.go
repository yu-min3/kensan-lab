package objects

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestRecordReplayPreservesFirstMetadata(t *testing.T) {
	s, _ := fixture(t)
	ctx := context.Background()
	first, _, e := s.PublishRecord(ctx, "feedback", "event", json.RawMessage(`{"x":1,"y":2}`), json.RawMessage(`{ "text": "<script>", "timestamp": "first" }`))
	if e != nil {
		t.Fatal(e)
	}
	second, reused, e := s.PublishRecord(ctx, "feedback", "event", json.RawMessage(`{"y":2,"x":1}`), json.RawMessage(`{"timestamp":"second"}`))
	if e != nil || !reused || string(first.Data) != string(second.Data) {
		t.Fatalf("replay %v %v", reused, e)
	}
	if _, _, e = s.PublishRecord(ctx, "feedback", "event", json.RawMessage(`{"x":2}`), json.RawMessage(`{}`)); !errors.Is(e, ErrConflict) {
		t.Fatal(e)
	}
}

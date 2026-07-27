package feeds

import (
	"testing"
	"time"

	"github.com/yu-min3/kensan-lab/apps/kensan/backend/internal/workspace"
)

func TestAcknowledgementsRoundTrip(t *testing.T) {
	ws := workspace.New(t.TempDir())
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.FixedZone("Asia/Tokyo", 9*60*60))

	if err := SetAcknowledgement(ws, "https://example.com/thread/1", "確認する通知", true, now); err != nil {
		t.Fatal(err)
	}
	items, err := ListAcknowledgements(ws)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Key != "https://example.com/thread/1" || items[0].Title != "確認する通知" {
		t.Fatalf("unexpected acknowledgements: %+v", items)
	}
	if items[0].AcknowledgedAt != "2026-07-27T12:00:00+09:00" {
		t.Fatalf("unexpected acknowledgedAt: %s", items[0].AcknowledgedAt)
	}

	if err := SetAcknowledgement(ws, items[0].Key, items[0].Title, false, now); err != nil {
		t.Fatal(err)
	}
	items, err = ListAcknowledgements(ws)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("want empty acknowledgements, got %+v", items)
	}
}

func TestAcknowledgementRejectsEmptyKey(t *testing.T) {
	ws := workspace.New(t.TempDir())
	if err := SetAcknowledgement(ws, " ", "title", true, time.Now()); err == nil {
		t.Fatal("want validation error")
	}
}

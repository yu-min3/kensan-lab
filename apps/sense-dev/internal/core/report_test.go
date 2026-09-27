package core

import (
	"strings"
	"testing"
	"time"
)

func TestDailyPreviewIsJSTIdempotentAndNotSent(t *testing.T) {
	s := testStore(t)
	task, agent := taskAgent(t, s, App, "acceptance")
	q, err := s.AskQuestion(agent.ID, "Which scenario?")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 27, 15, 30, 0, 0, time.UTC)
	report, err := s.PreviewDailyReport(now)
	if err != nil {
		t.Fatal(err)
	}
	if report.Date != "2026-09-28" || report.Status != "preview" || report.DeliveryStatus != "not_configured" || len(report.TaskIDs) != 1 || report.TaskIDs[0] != task.ID || len(report.PendingIDs) != 1 || report.PendingIDs[0] != "question-"+q.ID {
		t.Fatalf("wrong report %+v", report)
	}
	if !strings.Contains(report.Summary, "判断待ち 1 件") {
		t.Fatal("pending item omitted")
	}
	retry, err := s.PreviewDailyReport(now.Add(time.Hour))
	if err != nil || retry.CreatedAt != report.CreatedAt || len(s.Snapshot().Reports) != 1 {
		t.Fatal("daily preview was duplicated")
	}
	if report.SentAt != nil {
		t.Fatal("preview marked sent")
	}
}

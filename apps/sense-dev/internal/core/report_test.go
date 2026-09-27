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

func TestScheduledOutboxIsSeparateFromPreviewAndJSTIdempotent(t *testing.T) {
	s := testStore(t)
	before := time.Date(2026, 9, 27, 10, 59, 0, 0, time.UTC) // 09/27 19:59 JST
	if _, created, err := s.QueueDailyReport(before); err != nil || created {
		t.Fatalf("report queued before cutoff: %t %v", created, err)
	}
	now := before.Add(time.Minute)
	entry, created, err := s.QueueDailyReport(now)
	if err != nil || !created || entry.Date != "2026-09-27" || entry.Status != "waiting_destination" {
		t.Fatalf("scheduled report not queued: %+v %t %v", entry, created, err)
	}
	preview, err := s.PreviewDailyReport(now)
	if err != nil || preview.Date != "2026-09-27" || len(s.Snapshot().ReportOutbox) != 1 || len(s.Snapshot().Reports) != 1 {
		t.Fatal("scheduled report and manual preview were conflated")
	}
	if _, err := s.CreateTask("mission", App, "analysis", "after cutoff", "v1"); err != nil {
		t.Fatal(err)
	}
	again, created, err := s.QueueDailyReport(now.Add(time.Hour))
	if err != nil || created || again.CreatedAt != entry.CreatedAt || len(again.TaskIDs) != 0 {
		t.Fatal("scheduled report duplicated or changed after cutoff")
	}
	if _, created, err := s.QueueDailyReport(now.Add(24*time.Hour - time.Minute)); err != nil || created {
		t.Fatalf("next report queued before 20:00 JST: %t %v", created, err)
	}
	next, created, err := s.QueueDailyReport(now.Add(24 * time.Hour))
	if err != nil || !created || next.Date != "2026-09-28" {
		t.Fatalf("next JST report was not queued at 20:00: %+v %t %v", next, created, err)
	}
}

func TestReportDeliveryUnknownAfterRestartNeverBlindlyRetries(t *testing.T) {
	s := testStore(t)
	now := time.Date(2026, 9, 27, 11, 0, 0, 0, time.UTC)
	entry, _, err := s.QueueDailyReport(now)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ConfigureReportDestination(entry.Date, "https://example.invalid/hook"); err == nil {
		t.Fatal("URL or credential-shaped destination accepted")
	}
	if err := s.ConfigureReportDestination(entry.Date, "C123456789"); err != nil {
		t.Fatal(err)
	}
	attempt, err := s.BeginReportDelivery(entry.Date, now)
	if err != nil || attempt.Status != "sending" || attempt.AttemptID == "" {
		t.Fatalf("delivery intent not persisted: %+v %v", attempt, err)
	}
	if _, err := s.BeginReportDelivery(entry.Date, now); err == nil {
		t.Fatal("duplicate outbound attempt accepted")
	}
	if err := s.FinishReportDelivery(entry.Date, "wrong-attempt", "sent", "external-1", now); err == nil {
		t.Fatal("wrong attempt marked sent")
	}
	root := s.root
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.RecoverSendingReports(); err != nil {
		t.Fatal(err)
	}
	if got := s.Snapshot().ReportOutbox[entry.Date]; got.Status != "unknown" || got.AttemptID != attempt.AttemptID || got.SentAt != nil {
		t.Fatalf("ambiguous send was lost or falsely confirmed: %+v", got)
	}
	if _, err := s.BeginReportDelivery(entry.Date, now); err == nil {
		t.Fatal("unknown delivery automatically retried")
	}
	if err := s.ReconcileUnknownReport(entry.Date, attempt.AttemptID, "sent", "external-1", "", now); err == nil {
		t.Fatal("unknown delivery reconciled without external evidence")
	}
	if err := s.ReconcileUnknownReport(entry.Date, attempt.AttemptID, "sent", "external-1", "operator checked channel history", now); err != nil {
		t.Fatal(err)
	}
	if got := s.Snapshot().ReportOutbox[entry.Date]; got.Status != "sent" || got.ReconciliationEvidence == "" || got.SentAt == nil {
		t.Fatalf("reconciled receipt not preserved: %+v", got)
	}
}

func TestReportDeliveryReceiptAndMissedGap(t *testing.T) {
	s := testStore(t)
	first := time.Date(2026, 9, 27, 11, 0, 0, 0, time.UTC)
	entry, _, err := s.QueueDailyReport(first)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ConfigureReportDestination(entry.Date, "C123456789"); err != nil {
		t.Fatal(err)
	}
	attempt, err := s.BeginReportDelivery(entry.Date, first)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.FinishReportDelivery(entry.Date, attempt.AttemptID, "sent", "slack-ts-1", first); err != nil {
		t.Fatal(err)
	}
	if got := s.Snapshot().ReportOutbox[entry.Date]; got.Status != "sent" || got.SentAt == nil || got.ExternalID != "slack-ts-1" {
		t.Fatalf("delivery receipt not recorded: %+v", got)
	}
	later := first.AddDate(0, 0, 3)
	if _, created, err := s.QueueDailyReport(later); err != nil || !created {
		t.Fatalf("later report not queued: %t %v", created, err)
	}
	state := s.Snapshot()
	if state.ReportOutbox["2026-09-28"].Status != "missed" || state.ReportOutbox["2026-09-29"].Status != "missed" || state.ReportOutbox["2026-09-30"].Status != "waiting_destination" {
		t.Fatalf("downtime gaps were fabricated or hidden: %+v", state.ReportOutbox)
	}
}

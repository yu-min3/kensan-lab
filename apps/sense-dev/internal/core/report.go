package core

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"time"
)

var jst = time.FixedZone("JST", 9*60*60)
var reportDestination = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

func reportSnapshot(st *State, date string, now time.Time) (string, []string, []string) {
	statusCounts := map[string]int{}
	tasks := make([]string, 0, len(st.Tasks))
	for id, task := range st.Tasks {
		if task.Status != "done" || task.UpdatedAt.In(jst).Format("2006-01-02") == date {
			tasks = append(tasks, id)
			statusCounts[task.Status]++
		}
	}
	sort.Strings(tasks)
	pending := make([]string, 0)
	for id, q := range st.Questions {
		if q.Status == "pending" {
			pending = append(pending, "question-"+id)
		}
	}
	for id, a := range st.Approvals {
		if a.Status == "pending" && now.Before(a.ExpiresAt) {
			pending = append(pending, "approval-"+id)
		}
	}
	sort.Strings(pending)
	eventCount := 0
	for _, e := range st.Events {
		if e.At.In(jst).Format("2006-01-02") == date {
			eventCount++
		}
	}
	summary := fmt.Sprintf("対象案件 %d 件（完了 %d / 公開待ち %d）、当日の記録 %d 件、生成時点の判断待ち %d 件。", len(tasks), statusCounts["done"], statusCounts["publish_wait"], eventCount, len(pending))
	return summary, tasks, pending
}

// PreviewDailyReport creates one immutable, private report preview per JST day.
// It records no delivery success and never contacts an external service.
func (s *Store) PreviewDailyReport(now time.Time) (DailyReport, error) {
	date := now.In(jst).Format("2006-01-02")
	var out DailyReport
	err := s.update(func(st *State) error {
		if old, ok := st.Reports[date]; ok {
			out = old
			return nil
		}
		summary, tasks, pending := reportSnapshot(st, date, now)
		out = DailyReport{Date: date, Status: "preview", DeliveryStatus: "not_configured", Summary: summary, TaskIDs: tasks, PendingIDs: pending, CreatedAt: now.UTC()}
		st.Reports[date] = out
		st.Events = append(st.Events, event("daily_report_preview", date, "delivery not configured"))
		return nil
	})
	return out, err
}

// QueueDailyReport freezes yesterday's JST report after 00:05. It never
// contacts a destination. A prior preview stays separate and immutable.
func (s *Store) QueueDailyReport(now time.Time) (ReportOutboxEntry, bool, error) {
	local := now.In(jst)
	if local.Hour() == 0 && local.Minute() < 5 {
		return ReportOutboxEntry{}, false, nil
	}
	date := local.AddDate(0, 0, -1).Format("2006-01-02")
	s.mu.Lock()
	old, exists := s.data.ReportOutbox[date]
	s.mu.Unlock()
	if exists {
		return old, false, nil
	}
	var out ReportOutboxEntry
	created := false
	err := s.update(func(st *State) error {
		if old, ok := st.ReportOutbox[date]; ok {
			out = old
			return nil
		}
		// A gap cannot be backfilled as a truthful historical snapshot. Keep
		// explicit missed entries rather than manufacturing old task states.
		latest := ""
		for prior := range st.ReportOutbox {
			if prior < date && prior > latest {
				latest = prior
			}
		}
		if latest != "" {
			start, err := time.ParseInLocation("2006-01-02", latest, jst)
			if err != nil || start.Format("2006-01-02") != latest {
				return errors.New("invalid prior report date in outbox")
			}
			target, _ := time.ParseInLocation("2006-01-02", date, jst)
			if target.Sub(start) > 366*24*time.Hour {
				return errors.New("daily report gap exceeds one year; operator reconciliation required")
			}
			for day := start.AddDate(0, 0, 1); day.Format("2006-01-02") < date; day = day.AddDate(0, 0, 1) {
				missedDate := day.Format("2006-01-02")
				if _, ok := st.ReportOutbox[missedDate]; !ok {
					st.ReportOutbox[missedDate] = ReportOutboxEntry{Date: missedDate, Status: "missed", Summary: "controller 停止中のため当日の確定 snapshot はありません。", CreatedAt: now.UTC(), UpdatedAt: now.UTC()}
					st.Events = append(st.Events, event("daily_report_missed", missedDate, "historical snapshot unavailable"))
				}
			}
		}
		summary, tasks, pending := reportSnapshot(st, date, now)
		out = ReportOutboxEntry{Date: date, Status: "waiting_destination", Summary: summary, TaskIDs: tasks, PendingIDs: pending, CreatedAt: now.UTC(), UpdatedAt: now.UTC()}
		st.ReportOutbox[date] = out
		st.Events = append(st.Events, event("daily_report_queued", date, "delivery not configured"))
		created = true
		return nil
	})
	return out, created, err
}

// ConfigureReportDestination accepts only an opaque recipient ID, never a
// URL or credential. No sender is wired until Yu chooses a delivery channel.
func (s *Store) ConfigureReportDestination(date, destination string) error {
	if !reportDestination.MatchString(destination) {
		return errors.New("report destination must be an opaque ID")
	}
	return s.update(func(st *State) error {
		entry, ok := st.ReportOutbox[date]
		if !ok || entry.Status != "waiting_destination" {
			return errors.New("report is not awaiting a destination")
		}
		entry.Destination, entry.Status, entry.UpdatedAt = destination, "queued", time.Now().UTC()
		st.ReportOutbox[date] = entry
		return nil
	})
}

// BeginReportDelivery persists the outbound intent before any sender call.
func (s *Store) BeginReportDelivery(date string, now time.Time) (ReportOutboxEntry, error) {
	id, err := newID()
	if err != nil {
		return ReportOutboxEntry{}, err
	}
	var out ReportOutboxEntry
	err = s.update(func(st *State) error {
		entry, ok := st.ReportOutbox[date]
		if !ok || entry.Status != "queued" || entry.Destination == "" {
			return errors.New("report delivery is not queued")
		}
		entry.Status, entry.AttemptID, entry.UpdatedAt = "sending", id, now.UTC()
		st.ReportOutbox[date] = entry
		st.Events = append(st.Events, event("daily_report_sending", date, id))
		out = entry
		return nil
	})
	return out, err
}

func (s *Store) FinishReportDelivery(date, attemptID, status, externalID string, now time.Time) error {
	if status != "sent" && status != "failed" && status != "unknown" || status == "sent" && externalID == "" || status != "sent" && externalID != "" {
		return errors.New("invalid delivery outcome")
	}
	return s.update(func(st *State) error {
		entry, ok := st.ReportOutbox[date]
		if !ok || entry.Status != "sending" || entry.AttemptID != attemptID {
			return errors.New("delivery attempt mismatch")
		}
		entry.Status, entry.ExternalID, entry.UpdatedAt = status, externalID, now.UTC()
		if status == "sent" {
			sent := now.UTC()
			entry.SentAt = &sent
		}
		st.ReportOutbox[date] = entry
		st.Events = append(st.Events, event("daily_report_"+status, date, attemptID))
		return nil
	})
}

// RecoverSendingReports leaves ambiguous sends in the outbox for manual
// reconciliation. It never automatically retries a possibly delivered post.
func (s *Store) RecoverSendingReports() error {
	return s.update(func(st *State) error {
		for date, entry := range st.ReportOutbox {
			if entry.Status != "sending" {
				continue
			}
			entry.Status, entry.UpdatedAt = "unknown", time.Now().UTC()
			st.ReportOutbox[date] = entry
			st.Events = append(st.Events, event("daily_report_unknown", date, entry.AttemptID))
		}
		return nil
	})
}

// ReconcileUnknownReport records an externally verified outcome. This is a
// distinct human/operator action, never part of the automatic timer.
func (s *Store) ReconcileUnknownReport(date, attemptID, status, externalID, evidence string, now time.Time) error {
	if status != "sent" && status != "failed" || status == "sent" && externalID == "" || status == "failed" && externalID != "" || len(evidence) == 0 || len(evidence) > 512 {
		return errors.New("invalid report reconciliation evidence")
	}
	return s.update(func(st *State) error {
		entry, ok := st.ReportOutbox[date]
		if !ok || entry.Status != "unknown" || entry.AttemptID != attemptID {
			return errors.New("report is not an unknown matching attempt")
		}
		entry.Status, entry.ExternalID, entry.ReconciliationEvidence, entry.UpdatedAt = status, externalID, evidence, now.UTC()
		if status == "sent" {
			sent := now.UTC()
			entry.SentAt = &sent
		}
		st.ReportOutbox[date] = entry
		st.Events = append(st.Events, event("daily_report_reconciled", date, attemptID))
		return nil
	})
}

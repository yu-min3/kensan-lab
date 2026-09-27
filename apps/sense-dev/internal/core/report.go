package core

import (
	"fmt"
	"sort"
	"time"
)

var jst = time.FixedZone("JST", 9*60*60)

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
		out = DailyReport{Date: date, Status: "preview", DeliveryStatus: "not_configured", Summary: fmt.Sprintf("対象案件 %d 件（完了 %d / 公開待ち %d）、本日の記録 %d 件、判断待ち %d 件。", len(tasks), statusCounts["done"], statusCounts["publish_wait"], eventCount, len(pending)), TaskIDs: tasks, PendingIDs: pending, CreatedAt: now.UTC()}
		st.Reports[date] = out
		st.Events = append(st.Events, event("daily_report_preview", date, "delivery not configured"))
		return nil
	})
	return out, err
}

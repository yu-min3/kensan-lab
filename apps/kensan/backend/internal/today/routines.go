// Package today joins file-backed goals, tasks and their Markdown history for the daily view.
package today

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/yu-min3/kensan-lab/apps/kensan/backend/internal/tasks"
)

var JST = time.FixedZone("JST", 9*60*60)
var routineRE = regexp.MustCompile(`^- \[([^]]+)\] (.+)$`)
var weeklyRE = regexp.MustCompile(`^週([1-7])回$`)
var monthRE = regexp.MustCompile(`^月([1-9][0-9]?)$`)
var dayRE = regexp.MustCompile(`^([1-9]|[12][0-9]|3[01])日$`)

type Period struct {
	Start  string `json:"start"`
	Count  int    `json:"count"`
	Target int    `json:"target"`
	Status string `json:"status"` // met | pending | missed | unknown
}
type Routine struct {
	ID            string         `json:"id"`
	Project       string         `json:"project"`
	Text          string         `json:"text"`
	Schedule      string         `json:"schedule"`
	Unit          string         `json:"unit"`
	Target        int            `json:"target"`
	File          string         `json:"file"`
	Line          int            `json:"line"`
	Supported     bool           `json:"supported"`
	DoneToday     bool           `json:"doneToday"`
	ExpectedToday bool           `json:"expectedToday"`
	Periods       []Period       `json:"periods"`
	Streak        int            `json:"streak"`
	Weekdays      []time.Weekday `json:"-"`
	MonthDay      int            `json:"-"`
}

func ParseRoutines(content, file, project string) []Routine {
	out := []Routine{}
	inSection := false
	for i, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "## ") {
			inSection = strings.TrimSpace(strings.TrimPrefix(line, "## ")) == "ルーティン"
			continue
		}
		if !inSection {
			continue
		}
		m := routineRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		r := Routine{Project: project, File: file, Line: i + 1, Text: m[2], Schedule: m[1], Target: 1, Unit: "日", Periods: []Period{}}
		r.ID = fmt.Sprintf("routine-%x", sha256.Sum256([]byte(project+"\n"+m[1]+"\n"+m[2])))
		switch {
		case m[1] == "毎日":
			r.Supported = true
		case weeklyRE.MatchString(m[1]):
			r.Unit = "週"
			r.Target, _ = strconv.Atoi(weeklyRE.FindStringSubmatch(m[1])[1])
			r.Supported = true
		case monthRE.MatchString(m[1]):
			r.Unit = "月"
			r.Target, _ = strconv.Atoi(monthRE.FindStringSubmatch(m[1])[1])
			r.Supported = r.Target <= 31
		case dayRE.MatchString(m[1]):
			r.Unit = "月"
			r.MonthDay, _ = strconv.Atoi(dayRE.FindStringSubmatch(m[1])[1])
			r.Supported = true
		default:
			valid := true
			seen := map[time.Weekday]bool{}
			for _, s := range strings.Split(m[1], ",") {
				index := strings.Index("日月火水木金土", strings.TrimSpace(s))
				if index < 0 || len([]rune(strings.TrimSpace(s))) != 1 {
					valid = false
					break
				}
				d := time.Weekday(index / 3)
				if !seen[d] {
					r.Weekdays = append(r.Weekdays, d)
					seen[d] = true
				}
			}
			r.Supported = valid && len(r.Weekdays) > 0
		}
		// Prose exceptions are not a machine-readable schedule. Keep a record button,
		// but do not claim misses/streaks against a contradictory target.
		if strings.Contains(r.Text, "どちらか") {
			r.Supported = false
		}
		out = append(out, r)
	}
	return out
}

func (r Routine) expected(d time.Time) bool {
	if r.MonthDay > 0 {
		return d.Day() == r.MonthDay
	}
	if len(r.Weekdays) == 0 {
		return true
	}
	for _, day := range r.Weekdays {
		if day == d.Weekday() {
			return true
		}
	}
	return false
}
func (r Routine) start(d time.Time) time.Time {
	d = time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, JST)
	if r.Unit == "月" {
		return time.Date(d.Year(), d.Month(), 1, 0, 0, 0, 0, JST)
	}
	if r.Unit == "週" {
		return d.AddDate(0, 0, -(int(d.Weekday())+6)%7)
	}
	return d
}
func (r Routine) next(d time.Time, step int) time.Time {
	if r.Unit == "月" {
		return d.AddDate(0, step, 0)
	}
	if r.Unit == "週" {
		return d.AddDate(0, 0, 7*step)
	}
	return d.AddDate(0, 0, step)
}

// Routines は README の定義と、daily の ## 習慣 に残った実施日から期間ごとの達成を出す。
// 記録の最初の日より前は未知（unknown）として扱い、未達と断定しない。
func Routines(root string, h History, now time.Time) ([]Routine, error) {
	out := []Routine{}
	for _, project := range tasks.Projects(root) {
		file := "projects/" + project + "/README.md"
		b, err := os.ReadFile(filepath.Join(root, file))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, r := range ParseRoutines(string(b), file, project) {
			dates := h.Routines[routineKey(project, r.Text)]
			first := ""
			for d := range dates {
				if first == "" || d < first {
					first = d
				}
			}
			r.DoneToday = dates[now.Format("2006-01-02")]
			r.ExpectedToday = r.expected(now)
			start := r.start(now)
			for len(r.Periods) < 8 || (first != "" && start.Format("2006-01-02") >= first) {
				if r.MonthDay > 0 && start.AddDate(0, 1, -1).Day() < r.MonthDay {
					start = r.next(start, -1)
					continue
				}
				if r.Unit == "日" && !r.expected(start) {
					start = r.next(start, -1)
					continue
				}
				end := r.next(start, 1)
				p := Period{Start: start.Format("2006-01-02"), Target: r.Target, Status: "missed"}
				for d := range dates {
					if d >= p.Start && d < end.Format("2006-01-02") {
						p.Count++
					}
				}
				switch {
				case !r.Supported:
					p.Status = "unknown"
				case p.Count >= p.Target:
					p.Status = "met"
				case end.After(now):
					p.Status = "pending"
				case first == "" || p.Start < first:
					p.Status = "unknown"
				}
				r.Periods = append(r.Periods, p)
				start = r.next(start, -1)
			}
			for i, p := range r.Periods {
				if i == 0 && p.Status == "pending" {
					continue
				}
				if p.Status != "met" {
					break
				}
				r.Streak++
			}
			r.Periods = r.Periods[:8]
			out = append(out, r)
		}
	}
	return out, nil
}

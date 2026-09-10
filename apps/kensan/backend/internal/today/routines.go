// Package today joins file-backed goals, tasks and observed activity for the daily view.
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
	"github.com/yu-min3/kensan-lab/apps/kensan/backend/internal/workspace"
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

func Routines(root string, events []workspace.Activity, now time.Time) ([]Routine, error) {
	out := []Routine{}
	first := map[string]string{}
	states := map[string]workspace.Activity{}
	for _, e := range events {
		if e.Kind == "routine.state" {
			if first[e.ID] == "" || e.Date < first[e.ID] {
				first[e.ID] = e.Date
			}
			states[e.ID+"/"+e.Date] = e
		}
	}
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
			r.DoneToday = states[r.ID+"/"+now.Format("2006-01-02")].State == "done"
			r.ExpectedToday = r.expected(now)
			start := r.start(now)
			for len(r.Periods) < 8 || (first[r.ID] != "" && start.Format("2006-01-02") >= first[r.ID]) {
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
				for _, e := range states {
					if e.ID == r.ID && e.State == "done" && e.Date >= p.Start && e.Date < end.Format("2006-01-02") {
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
				case first[r.ID] == "" || p.Start < first[r.ID]:
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

func SetRoutine(ws *workspace.Workspace, file, id, date string, done bool, now time.Time) error {
	parts := strings.Split(file, "/")
	if len(parts) != 3 || parts[0] != "projects" || parts[2] != "README.md" {
		return fmt.Errorf("invalid routine file")
	}
	if date != now.Format("2006-01-02") {
		return fmt.Errorf("only today's routine can be changed")
	}
	return ws.MutateEvent(file, func(b []byte, events []workspace.Activity) ([]byte, *workspace.Activity, error) {
		for _, r := range ParseRoutines(string(b), file, parts[1]) {
			if r.ID != id {
				continue
			}
			state := "todo"
			if done {
				state = "done"
			}
			previous := "todo"
			for _, e := range events {
				if e.Kind == "routine.state" && e.ID == id && e.Date == date {
					previous = e.State
				}
			}
			if previous == state {
				return nil, nil, nil
			}
			return nil, &workspace.Activity{At: now, Kind: "routine.state", ID: id, Project: r.Project, Text: r.Text, Date: date, State: state}, nil
		}
		return nil, nil, tasks.ErrLineMismatch
	})
}

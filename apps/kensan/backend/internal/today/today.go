package today

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/yu-min3/kensan-lab/apps/kensan/backend/internal/goals"
	"github.com/yu-min3/kensan-lab/apps/kensan/backend/internal/metrics"
	"github.com/yu-min3/kensan-lab/apps/kensan/backend/internal/projects"
	"github.com/yu-min3/kensan-lab/apps/kensan/backend/internal/tasks"
	"github.com/yu-min3/kensan-lab/apps/kensan/backend/internal/workspace"
)

type Day struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}
type View struct {
	Date          string             `json:"date"`
	Goals         goals.Goals        `json:"goals"`
	Projects      []projects.Summary `json:"projects"`
	Board         tasks.Board        `json:"board"`
	Routines      []Routine          `json:"routines"`
	Activity      []Day              `json:"activity"`
	RecordedSince string             `json:"recordedSince"`
	Triage        *tasks.Task        `json:"triage"`
	DeferredToday bool               `json:"deferredToday"`
	Skipped       []tasks.Task       `json:"skipped"`
	Forecasts     map[string]string  `json:"forecasts"`
	Phases        map[string]string  `json:"phases"`
}

func Load(ws *workspace.Workspace, now time.Time) (View, error) {
	now = now.In(JST)
	v := View{Date: now.Format("2006-01-02"), Projects: projects.Summaries(ws.Root), Activity: []Day{}, Skipped: []tasks.Task{}}
	v.Forecasts = map[string]string{}
	v.Phases = map[string]string{}
	var err error
	if v.Goals, err = goals.Load(ws.Root); err != nil {
		return v, err
	}
	if v.Board, err = tasks.CollectAt(ws.Root, now); err != nil {
		return v, err
	}
	events, err := ws.Activities()
	if err != nil {
		return v, err
	}
	if v.Routines, err = Routines(ws.Root, events, now); err != nil {
		return v, err
	}
	states := map[string]workspace.Activity{}
	reviews := map[string]time.Time{}
	for _, e := range events {
		if v.RecordedSince == "" || e.Date < v.RecordedSince {
			v.RecordedSince = e.Date
		}
		switch e.Kind {
		case "task.state":
			states[e.ID] = e
		case "routine.state":
			states[e.ID+"/"+e.Date] = e
		case "task.deferred", "task.triaged":
			reviews[e.ID] = e.At
			if e.Date == v.Date {
				v.DeferredToday = true
			}
		}
	}
	counts := map[string]int{}
	for _, e := range states {
		if e.State == "done" {
			counts[e.Date]++
		}
	}
	for d := now.AddDate(0, 0, -125); !d.After(now); d = d.AddDate(0, 0, 1) {
		date := d.Format("2006-01-02")
		v.Activity = append(v.Activity, Day{date, counts[date]})
	}
	candidates := []tasks.Task{}
	for _, t := range v.Board.Later {
		if t.State == "todo" {
			candidates = append(candidates, t)
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool { return reviews[candidates[i].ID].Before(reviews[candidates[j].ID]) })
	if !v.DeferredToday && len(candidates) > 0 {
		v.Triage = &candidates[0]
	}
	for _, p := range v.Projects {
		d, err := projects.Load(ws.Root, p.Name)
		if err != nil {
			return v, err
		}
		for _, m := range d.Milestones {
			if m.State == "todo" {
				v.Phases[p.Name] = m.Display
				break
			}
		}
		if r, err := metrics.Load(ws.Root, p.Name, now); err == nil && len(r.Metrics) > 0 {
			v.Forecasts[p.Name] = Forecast(r.Metrics[0], now)
		}
		for _, t := range d.Tasks {
			if t.State == "skipped" {
				v.Skipped = append(v.Skipped, t)
			}
		}
	}
	return v, nil
}

// Forecast is a linear extrapolation of at least three observations spanning
// fourteen days within the last thirty days. Never infer progress from task counts.
func Forecast(v metrics.View, now time.Time) string {
	if v.Current == nil || v.Target == nil || v.Stale || v.Direction != "increase" {
		return ""
	}
	if *v.Current >= *v.Target {
		return "目標値に到達"
	}
	points := []metrics.Point{}
	for _, p := range v.Series {
		at, err := time.Parse(time.RFC3339, p.At)
		if err != nil {
			at, err = time.ParseInLocation("2006-01-02", p.At, JST)
		}
		if err != nil || at.After(now) {
			continue
		}
		date := at.In(JST).Format("2006-01-02")
		if date >= now.AddDate(0, 0, -30).Format("2006-01-02") {
			points = append(points, metrics.Point{At: date, Value: p.Value})
		}
	}
	sort.SliceStable(points, func(i, j int) bool { return points[i].At < points[j].At })
	if len(points) < 3 {
		return ""
	}
	first, last := points[0], points[len(points)-1]
	a, errA := time.Parse("2006-01-02", first.At[:min(10, len(first.At))])
	b, errB := time.Parse("2006-01-02", last.At[:min(10, len(last.At))])
	span := b.Sub(a).Hours() / 24
	if errA != nil || errB != nil || span < 14 || last.Value <= first.Value {
		return ""
	}
	days := math.Ceil((*v.Target - last.Value) * span / (last.Value - first.Value))
	if days < 0 || days > 3650 {
		return ""
	}
	return fmt.Sprintf("直近%.0f日のペースが続けば %s 頃（推計）", span, b.AddDate(0, 0, int(days)).Format("2006-01"))
}

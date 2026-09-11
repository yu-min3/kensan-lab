package today

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yu-min3/kensan-lab/apps/kensan/backend/internal/metrics"
	"github.com/yu-min3/kensan-lab/apps/kensan/backend/internal/tasks"
	"github.com/yu-min3/kensan-lab/apps/kensan/backend/internal/workspace"
)

func TestEmptyWorkspaceUsesArrayContract(t *testing.T) {
	v, err := Load(workspace.New(t.TempDir()), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"projects":[]`) {
		t.Fatalf("empty projects must be an array: %s", b)
	}
}

func TestTriageReplayAndConflictingEdits(t *testing.T) {
	for _, action := range []string{"today", "later", "skip"} {
		t.Run(action, func(t *testing.T) {
			ws := fixture(t, "## タスク\n- [ ] Item @p(1000)\n")
			v, err := Load(ws, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			a := *v.Triage
			for i := 0; i < 2; i++ {
				if _, err := tasks.Triage(ws, a.File, a.Line, a.Text, action); err != nil {
					t.Fatalf("request %d: %v", i, err)
				}
			}
			events, err := ws.Activities()
			if err != nil || len(events) != 1 {
				t.Fatalf("retry duplicated events: %v %v", events, err)
			}
			content, err := os.ReadFile(filepath.Join(ws.Root, a.File))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(ws.Root, a.File), []byte(strings.Replace(string(content), "Item", "Edited", 1)), 0644); err != nil {
				t.Fatal(err)
			}
			if _, err := tasks.Triage(ws, a.File, a.Line, a.Text, action); !errors.Is(err, tasks.ErrLineMismatch) {
				t.Fatalf("stale edit accepted: %v", err)
			}
		})
	}
}

func TestTriageReplayRejectsChangedBand(t *testing.T) {
	ws := fixture(t, "## タスク\n- [ ] Item\n")
	v, err := Load(ws, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	a := *v.Triage
	updated, err := tasks.Triage(ws, a.File, a.Line, a.Text, "today")
	if err != nil {
		t.Fatal(err)
	}
	if replay, err := tasks.Triage(ws, a.File, a.Line, a.Text, "today"); err != nil || replay.Project != "demo" || replay.ID != updated.ID {
		t.Fatalf("replay lost identity: %+v %v", replay, err)
	}
	if _, err := tasks.SetBand(ws, updated.File, updated.Line, updated.Text, "later"); err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.Triage(ws, a.File, a.Line, a.Text, "today"); !errors.Is(err, tasks.ErrLineMismatch) {
		t.Fatalf("replay accepted a changed band: %v", err)
	}
}

func fixture(t *testing.T, body string) *workspace.Workspace {
	t.Helper()
	ws := workspace.New(t.TempDir())
	if err := ws.Create("projects/demo/README.md", []byte("---\ntype: project\nstatus: active\n---\n## 目標\nTest goal\n"+body)); err != nil {
		t.Fatal(err)
	}
	return ws
}
func taskAt(t *testing.T, ws *workspace.Workspace) tasks.Task {
	t.Helper()
	b, err := tasks.Collect(ws.Root)
	if err != nil {
		t.Fatal(err)
	}
	return b.Today[0]
}
func total(v View) int {
	n := 0
	for _, d := range v.Activity {
		n += d.Count
	}
	return n
}

func TestCompletionRetryUndoAndRename(t *testing.T) {
	ws := fixture(t, "## タスク\n- [ ] Ship @today\n")
	original := taskAt(t, ws)
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := tasks.SetState(ws, original.File, original.Line, original.Text, "done"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	events, err := ws.Activities()
	if err != nil || len(events) != 1 {
		t.Fatalf("retry generated events: %v %+v", err, events)
	}
	current := taskAt(t, ws)
	if current.ID == "" {
		t.Fatal("missing stable ID")
	}
	if updated, err := tasks.SetText(ws, current.File, current.Line, current.Text, "Ship renamed"); err != nil {
		t.Fatal(err)
	} else if updated.ID != current.ID || updated.Project != "demo" {
		t.Fatalf("mutation response lost identity: %+v", updated)
	}
	renamed := taskAt(t, ws)
	if renamed.ID != current.ID {
		t.Fatal("ID lost on text edit")
	}
	if _, err := tasks.SetState(ws, renamed.File, renamed.Line, renamed.Text, "todo"); err != nil {
		t.Fatal(err)
	}
	v, err := Load(ws, time.Now())
	if err != nil || total(v) != 0 {
		t.Fatalf("undo: %v %+v", err, v.Activity)
	}
	if _, err := tasks.SetState(ws, renamed.File, renamed.Line, renamed.Text, "done"); err != nil {
		t.Fatal(err)
	}
	v, err = Load(ws, time.Now())
	if err != nil || total(v) != 1 {
		t.Fatalf("re-completion: %v %d", err, total(v))
	}
}

func TestSaveAndMoveKeepIdentity(t *testing.T) {
	ws := fixture(t, "## タスク\n- [ ] Original @today\n")
	if err := ws.Create("projects/other/README.md", []byte("## タスク\n")); err != nil {
		t.Fatal(err)
	}
	a := taskAt(t, ws)
	tasks.SetState(ws, a.File, a.Line, a.Text, "done")
	a = taskAt(t, ws)
	_, err := tasks.EditTask(ws, a.File, a.Line, a.Text, "other", "Moved", "today", "", "")
	if err != nil {
		t.Fatal(err)
	}
	b := taskAt(t, ws)
	if a.ID != b.ID {
		t.Fatalf("identity lost: %s vs %s", a.ID, b.ID)
	}
	if _, err = tasks.SetState(ws, b.File, b.Line, b.Text, "todo"); err != nil {
		t.Fatal(err)
	}
	v, err := Load(ws, time.Now())
	if err != nil || total(v) != 0 {
		t.Fatalf("moved undo: %v %d", err, total(v))
	}
}

func TestUnreadableActivityDoesNotCompleteTask(t *testing.T) {
	ws := fixture(t, "## タスク\n- [ ] Safe @today\n")
	a := taskAt(t, ws)
	if err := os.Mkdir(filepath.Join(ws.Root, workspace.ActivityFile), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.SetState(ws, a.File, a.Line, a.Text, "done"); err == nil {
		t.Fatal("wanted failure")
	}
	if taskAt(t, ws).State != "todo" {
		t.Fatal("task changed despite log failure")
	}
}

func TestTriageSurvivesReloadAndRestoresSkipped(t *testing.T) {
	ws := fixture(t, "## タスク\n- [ ] First\n- [ ] Second\n")
	now := time.Now().In(JST)
	v, _ := Load(ws, now)
	a := v.Triage
	if _, err := tasks.ReviewLater(ws, a.File, a.Line, a.Text); err != nil {
		t.Fatal(err)
	}
	v, _ = Load(ws, now)
	if !v.DeferredToday || v.Triage != nil {
		t.Fatal("defer not persisted")
	}
	v, _ = Load(ws, now.AddDate(0, 0, 1))
	if v.Triage == nil || v.Triage.Display != "Second" {
		t.Fatalf("defer did not rotate: %+v", v.Triage)
	}
	a = v.Triage
	if _, err := tasks.SetState(ws, a.File, a.Line, a.Text, "skipped"); err != nil {
		t.Fatal(err)
	}
	v, _ = Load(ws, now)
	if len(v.Skipped) != 1 {
		t.Fatal("skipped not recoverable")
	}
	a = &v.Skipped[0]
	if _, err := tasks.SetState(ws, a.File, a.Line, a.Text, "todo"); err != nil {
		t.Fatal(err)
	}
	v, _ = Load(ws, now)
	if len(v.Skipped) != 0 || len(v.Board.Later) != 2 {
		t.Fatal("restore failed")
	}
}

func TestRoutinePeriodsAndUnknownHistory(t *testing.T) {
	ws := fixture(t, "## ルーティン\n- [週3回] English\n- [月1] Article\n- [月,水,金] Gym\n- [月,水,木,金,土,日] Gym（土日はどちらか1日）\n- [31日] Close\n")
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, JST) // Tuesday
	routines, _ := Routines(ws.Root, nil, now)
	if len(routines) != 5 {
		t.Fatal(routines)
	}
	events := []workspace.Activity{}
	for _, date := range []string{"2026-08-31", "2026-09-02", "2026-09-04", "2026-09-07"} {
		events = append(events, workspace.Activity{ID: routines[0].ID, Kind: "routine.state", Date: date, State: "done"})
	}
	routines, _ = Routines(ws.Root, events, now)
	week := routines[0]
	if week.Streak != 1 || week.Periods[0].Count != 1 || week.Periods[0].Status != "pending" {
		t.Fatalf("week: %+v", week)
	}
	if routines[1].Periods[1].Status != "unknown" {
		t.Fatal("new routine falsely marked missed")
	}
	if routines[2].ExpectedToday || routines[2].Periods[0].Start != "2026-09-07" {
		t.Fatal("rest day should not be an expectation")
	}
	if routines[3].Supported {
		t.Fatal("prose exception silently interpreted")
	}
	for _, p := range routines[4].Periods {
		if strings.HasSuffix(p.Start, "02-01") || strings.HasSuffix(p.Start, "09-01") {
			t.Fatal("31-day expectation in short month")
		}
	}
}

func TestRoutineRetryAndCancel(t *testing.T) {
	ws := fixture(t, "## ルーティン\n- [週3回] English\n")
	now := time.Date(2026, 9, 8, 23, 0, 0, 0, JST)
	rs, _ := Routines(ws.Root, nil, now)
	r := rs[0]
	for i := 0; i < 2; i++ {
		if err := SetRoutine(ws, r.File, r.ID, "2026-09-08", true, now); err != nil {
			t.Fatal(err)
		}
	}
	es, _ := ws.Activities()
	if len(es) != 1 {
		t.Fatal("retry duplicated")
	}
	if err := SetRoutine(ws, r.File, r.ID, "2026-09-08", false, now); err != nil {
		t.Fatal(err)
	}
	v, err := Load(ws, now)
	if err != nil || total(v) != 0 || v.Routines[0].DoneToday {
		t.Fatalf("cancel failed: %v", err)
	}
	if err := SetRoutine(ws, r.File, r.ID, "2026-09-08", true, now.Add(time.Hour)); err == nil {
		t.Fatal("accepted yesterday after midnight")
	}
}

func TestLongStreakAndMonthBoundary(t *testing.T) {
	ws := fixture(t, "## ルーティン\n- [毎日] Read\n- [月1] Publish\n")
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, JST)
	rs, _ := Routines(ws.Root, nil, now)
	es := []workspace.Activity{}
	for i := 1; i <= 12; i++ {
		es = append(es, workspace.Activity{ID: rs[0].ID, Kind: "routine.state", Date: now.AddDate(0, 0, -i).Format("2006-01-02"), State: "done"})
	}
	es = append(es, workspace.Activity{ID: rs[1].ID, Kind: "routine.state", Date: "2026-09-15", State: "done"})
	rs, _ = Routines(ws.Root, es, now)
	if rs[0].Streak != 12 || len(rs[0].Periods) != 8 {
		t.Fatalf("long streak truncated: %+v", rs[0])
	}
	if rs[1].Streak != 1 || rs[1].Periods[0].Count != 0 {
		t.Fatal("month rollover broken")
	}
}

func TestForecastRequiresEvidence(t *testing.T) {
	now := time.Date(2026, 9, 8, 0, 0, 0, 0, JST)
	current, target := 20.0, 100.0
	v := metrics.View{Current: &current, Target: &target, Direction: "increase", Series: []metrics.Point{{At: "2026-08-10", Value: 10}, {At: "2026-08-20", Value: 15}, {At: "2026-09-08", Value: 20}}}
	if got := Forecast(v, now); !strings.Contains(got, "推計") {
		t.Fatal(got)
	}
	for _, mutation := range []func(*metrics.View){func(v *metrics.View) { v.Stale = true }, func(v *metrics.View) { v.Series = v.Series[:2] }, func(v *metrics.View) { v.Direction = "decrease" }} {
		c := v
		mutation(&c)
		if Forecast(c, now) != "" {
			t.Fatal(fmt.Sprint(c))
		}
	}
}

func TestForecastIncludesTodaysTimestamp(t *testing.T) {
	now := time.Date(2026, 9, 10, 23, 0, 0, 0, JST)
	current, target := 30.0, 100.0
	v := metrics.View{Current: &current, Target: &target, Direction: "increase", Series: []metrics.Point{{At: "2026-08-15T12:00:00+09:00", Value: 10}, {At: "2026-08-25T12:00:00+09:00", Value: 20}, {At: "2026-09-10T12:00:00+09:00", Value: 30}}}
	if Forecast(v, now) == "" {
		t.Fatal("today's observation was dropped")
	}
}

func TestAllTriageChoicesFinishDay(t *testing.T) {
	for _, action := range []string{"today", "later", "skip"} {
		t.Run(action, func(t *testing.T) {
			ws := fixture(t, "## タスク\n- [ ] First\n- [ ] Second\n")
			now := time.Now().In(JST)
			v, _ := Load(ws, now)
			a := v.Triage
			if _, err := tasks.Triage(ws, a.File, a.Line, a.Text, action); err != nil {
				t.Fatal(err)
			}
			v, err := Load(ws, now)
			if err != nil || v.Triage != nil || !v.DeferredToday {
				t.Fatalf("day not finished: %v %+v", err, v.Triage)
			}
			if action == "today" && len(v.Board.Today) != 1 {
				t.Fatal("not promoted")
			}
			if action == "skip" && len(v.Skipped) != 1 {
				t.Fatal("not recoverable")
			}
		})
	}
}

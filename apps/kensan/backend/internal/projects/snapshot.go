package projects

import (
	"path/filepath"
	"sort"
	"time"

	"github.com/yu-min3/kensan-lab/apps/kensan/backend/internal/metrics"
	"github.com/yu-min3/kensan-lab/apps/kensan/backend/internal/tasks"
)

// Snapshot はREADMEと指標を一度だけ読み取った表示用の入力。
// 関連文書走査は含めない。複数ファイル間のトランザクションを意味しない。
type Snapshot struct {
	Content    string
	Summary    Summary
	Milestones []tasks.Task
	Tasks      []tasks.Task
	Metrics    []metrics.View
}

func ReadSnapshot(root, name string, now time.Time) (Snapshot, error) {
	content, err := readme(root, name)
	if err != nil {
		return Snapshot{}, err
	}
	fm := frontmatter(content)
	s := Summary{Name: name, Status: fm["status"], Deadline: fm["deadline"], Goal: firstLine(section(content, "目標")), Current: parseCurrent(section(content, "現在地"))}
	snapshot := Snapshot{Content: content, Tasks: []tasks.Task{}, Milestones: []tasks.Task{}}
	file := filepath.ToSlash(filepath.Join("projects", name, "README.md"))
	for _, t := range tasks.ExtractLines(content, file) {
		t.Project = name
		switch t.Section {
		case "マイルストーン":
			snapshot.Milestones = append(snapshot.Milestones, t)
			s.MilestonesTotal++
			if t.State == "done" {
				s.MilestonesDone++
			}
		case "タスク", "いつかやる":
			snapshot.Tasks = append(snapshot.Tasks, t)
			if t.State == "todo" {
				s.OpenTasks++
			}
		}
	}
	// 指標はopt-in。読み取れない定義から値を推測しない。
	if result, err := metrics.Load(root, name, now); err == nil {
		snapshot.Metrics = result.Metrics
	}
	date := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	s.State = ComputeState(s.Deadline, snapshot.Milestones, snapshot.Tasks, parseLog(section(content, "ログ")), snapshot.Metrics, date)
	s.Metric = briefOf(snapshot.Metrics)
	snapshot.Summary = s
	return snapshot, nil
}

func Snapshots(root string, now time.Time) []Snapshot {
	out := []Snapshot{}
	for _, name := range tasks.Projects(root) {
		snapshot, err := ReadSnapshot(root, name, now)
		if err == nil {
			out = append(out, snapshot)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i].Summary, out[j].Summary
		if (a.Status == "active") != (b.Status == "active") {
			return a.Status == "active"
		}
		if (a.Deadline == "") != (b.Deadline == "") {
			return a.Deadline != ""
		}
		if a.Deadline != b.Deadline {
			return a.Deadline < b.Deadline
		}
		return a.Name < b.Name
	})
	return out
}

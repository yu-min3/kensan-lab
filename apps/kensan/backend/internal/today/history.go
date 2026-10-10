package today

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/yu-min3/kensan-lab/apps/kensan/backend/internal/tasks"
	"github.com/yu-min3/kensan-lab/apps/kensan/backend/internal/workspace"
)

// 履歴は Markdown だけに持つ（DB も別ログも持たない）。
//   - タスクの完了日: 行末の @done(YYYY-MM-DD)。daily へ移動しても行と一緒に残る
//   - 習慣の実施: その日の daily の ## 習慣 に `- [x] 本文 @routine(project)`
const habitHeading = "## 習慣"

var (
	routineTagRe = regexp.MustCompile(`\s*@routine\(([^)]+)\)`)
	habitLineRe  = regexp.MustCompile(`^- \[x\] (.+)$`)
	dailyPathRe  = regexp.MustCompile(`^daily/(\d{4})/(\d{2})/(\d{2})\.md$`)
)

// History は Markdown から読み直した実績。
type History struct {
	Done     map[string]int             // 日付 → 完了タスク数
	Routines map[string]map[string]bool // routineKey → 実施日
}

func routineKey(project, text string) string { return project + "\n" + strings.TrimSpace(text) }

func habitLine(project, text string) string {
	return "- [x] " + strings.TrimSpace(text) + " @routine(" + project + ")"
}

// ReadHistory は project README・todo.md・daily を読み、完了と習慣の実績を集める。
func ReadHistory(root string) (History, error) {
	h := History{Done: map[string]int{}, Routines: map[string]map[string]bool{}}
	files := []string{"todo.md"}
	for _, p := range tasks.Projects(root) {
		files = append(files, "projects/"+p+"/README.md")
	}
	err := filepath.WalkDir(filepath.Join(root, "daily"), func(p string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return filepath.SkipDir
			}
			return err
		}
		if rel, _ := filepath.Rel(root, p); !d.IsDir() && dailyPathRe.MatchString(filepath.ToSlash(rel)) {
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return h, err
	}
	for _, rel := range files {
		b, err := os.ReadFile(filepath.Join(root, rel))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return h, err
		}
		content := string(b)
		for _, t := range tasks.ExtractLines(content, rel) {
			if t.State == "done" && t.Done != "" && t.Section != "習慣" {
				h.Done[t.Done]++
			}
		}
		if m := dailyPathRe.FindStringSubmatch(rel); m != nil {
			date := m[1] + "-" + m[2] + "-" + m[3]
			for _, line := range habitLines(content) {
				if hm := habitLineRe.FindStringSubmatch(line); hm != nil {
					if pm := routineTagRe.FindStringSubmatch(hm[1]); pm != nil {
						key := routineKey(pm[1], routineTagRe.ReplaceAllString(hm[1], ""))
						if h.Routines[key] == nil {
							h.Routines[key] = map[string]bool{}
						}
						h.Routines[key][date] = true
					}
				}
			}
		}
	}
	return h, nil
}

// habitLines は ## 習慣 の節の行を返す。
func habitLines(content string) []string {
	var out []string
	in := false
	for _, l := range strings.Split(content, "\n") {
		if strings.HasPrefix(l, "# ") || strings.HasPrefix(l, "## ") {
			in = strings.TrimSpace(l) == habitHeading
			continue
		}
		if in {
			out = append(out, strings.TrimSpace(l))
		}
	}
	return out
}

// setHabit は daily の ## 習慣 に実施行を足す／外す。外して節が空になれば見出しも消す。
// 変更不要なら nil を返す（再送は何もしない）。
func setHabit(content, line string, done bool) []byte {
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	start, end := -1, len(lines)
	for i, l := range lines {
		if start == -1 {
			if strings.TrimSpace(l) == habitHeading {
				start = i
			}
			continue
		}
		if strings.HasPrefix(l, "# ") || strings.HasPrefix(l, "## ") {
			end = i
			break
		}
	}
	has := -1
	if start != -1 {
		for i := start + 1; i < end; i++ {
			if strings.TrimSpace(lines[i]) == line {
				has = i
				break
			}
		}
	}
	switch {
	case done && has != -1, !done && has == -1:
		return nil
	case done && start == -1:
		return []byte(strings.Join(lines, "\n") + "\n\n" + habitHeading + "\n\n" + line + "\n")
	case done:
		insert := end
		for insert > start+1 && strings.TrimSpace(lines[insert-1]) == "" {
			insert--
		}
		add := []string{line}
		if insert == start+1 {
			add = []string{"", line}
		}
		if insert < len(lines) && strings.TrimSpace(lines[insert]) != "" {
			add = append(add, "") // 次の見出しとの間を空ける
		}
		out := append(append([]string{}, lines[:insert]...), add...)
		out = append(out, lines[insert:]...)
		return []byte(strings.Join(out, "\n") + "\n")
	default:
		out := append(append([]string{}, lines[:has]...), lines[has+1:]...)
		empty := true
		for i := start + 1; i < end-1; i++ {
			if strings.TrimSpace(out[i]) != "" {
				empty = false
				break
			}
		}
		if empty {
			// 見出しと、その前後の空行をまとめて落とす
			from, to := start, end-1
			for from > 0 && strings.TrimSpace(out[from-1]) == "" {
				from--
			}
			out = append(out[:from], out[to:]...)
		}
		return []byte(strings.TrimRight(strings.Join(out, "\n"), "\n") + "\n")
	}
}

// SetRoutine は今日の習慣の実施を daily に記録する（取消は行を外す）。
func SetRoutine(ws *workspace.Workspace, file, id, date string, done bool, now time.Time) error {
	parts := strings.Split(file, "/")
	if len(parts) != 3 || parts[0] != "projects" || parts[2] != "README.md" {
		return fmt.Errorf("invalid routine file")
	}
	if date != now.Format("2006-01-02") {
		return fmt.Errorf("only today's routine can be changed")
	}
	b, err := os.ReadFile(filepath.Join(ws.Root, file))
	if err != nil {
		return err
	}
	var target *Routine
	for _, r := range ParseRoutines(string(b), file, parts[1]) {
		if r.ID == id {
			target = &r
			break
		}
	}
	if target == nil {
		return tasks.ErrLineMismatch
	}
	day, _ := time.ParseInLocation("2006-01-02", date, JST)
	rel := fmt.Sprintf("daily/%04d/%02d/%02d.md", day.Year(), day.Month(), day.Day())
	return ws.Mutate(rel, func(content []byte, exists bool) ([]byte, error) {
		src := string(content)
		if !exists {
			if !done {
				return nil, nil
			}
			src = workspace.DailySkeleton(day)
		}
		out := setHabit(src, habitLine(target.Project, target.Text), done)
		if out == nil {
			return nil, nil
		}
		return workspace.TouchUpdated(out, now), nil
	})
}

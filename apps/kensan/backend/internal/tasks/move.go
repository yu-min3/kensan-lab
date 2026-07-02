package tasks

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/yu-min3/kensan-lab/apps/kensan/backend/internal/workspace"
)

// ErrLineMismatch は指定行の内容が期待と異なる場合（他クライアントが先に編集した）。
var ErrLineMismatch = errors.New("line content mismatch (file changed by another client?)")

// Dest はタスク行の移動先。今は完了タスクの daily 退避（/reflection）にのみ使う。
// 今日やる ⇄ ストックは「場所の移動」ではなく @today タグの切替（SetToday）で表現する。
type Dest struct {
	Kind string // daily
	Date time.Time
}

func (d Dest) resolve() (file, section string, err error) {
	switch d.Kind {
	case "daily":
		t := d.Date
		if t.IsZero() {
			t = ReflectionDate(time.Now())
		}
		return fmt.Sprintf("daily/%04d/%02d/%02d.md", t.Year(), t.Month(), t.Day()), "完了タスク", nil
	default:
		return "", "", fmt.Errorf("unknown dest: %q", d.Kind)
	}
}

// normalized は date 未指定の daily 移動に ReflectionDate を適用した Dest を返す。
// 日付の解決はここ 1 箇所。resolve / newDailySkeleton は解決済みの Date を使うだけ。
func (d Dest) normalized(now time.Time) Dest {
	if d.Kind == "daily" && d.Date.IsZero() {
		d.Date = ReflectionDate(now)
	}
	return d
}

// ReflectionDate は「振り返りとしての今日」を返す。
// CLAUDE.md の /reflection 日付判定規約: 0:00〜6:00 の操作は前日分として扱う。
// API / CLI で date 未指定の daily 移動はこの規約に従う（深夜に完了タスクを
// 片付けたとき、翌日の daily に書かれてしまうのを防ぐ）。
func ReflectionDate(now time.Time) time.Time {
	if now.Hour() < 6 {
		return now.AddDate(0, 0, -1)
	}
	return now
}

// Move はチェックボックス行をファイル間で移動する。
// かんばんのドラッグも /morning のタスク配置も、この 1 つの操作に帰着する。
//
// 楽観ロック: line 行目のテキストが expectText と一致しない場合は ErrLineMismatch。
// （Claude / VSCode が同じファイルを先に編集したケースの検出）
func Move(ws *workspace.Workspace, file string, line int, expectText string, dest Dest) (Task, error) {
	// 移動先パスと daily 骨組みが必ず同じ日付を見るよう、最初に正規化する
	// （resolve だけが ReflectionDate を知っていると、ファイル新規作成時の
	// newDailySkeleton が time.Now() に落ちてパスと中身の日付がズレる）
	dest = dest.normalized(time.Now())
	destFile, destSection, err := dest.resolve()
	if err != nil {
		return Task{}, err
	}
	if destFile == file {
		return Task{}, errors.New("source and destination are the same file")
	}

	// 1. 移動元から行を取り除く
	var taskLine string
	err = ws.Mutate(file, func(content []byte, exists bool) ([]byte, error) {
		if !exists {
			return nil, fmt.Errorf("source not found: %s", file)
		}
		lines := strings.Split(string(content), "\n")
		if line < 1 || line > len(lines) {
			return nil, fmt.Errorf("%w: line %d out of range", ErrLineMismatch, line)
		}
		m := checkboxRe.FindStringSubmatch(lines[line-1])
		if m == nil || strings.TrimSpace(m[2]) != strings.TrimSpace(expectText) {
			return nil, fmt.Errorf("%w: %s:%d", ErrLineMismatch, file, line)
		}
		taskLine = "- [" + m[1] + "] " + strings.TrimSpace(m[2])
		out := append(lines[:line-1], lines[line:]...)
		return []byte(workspaceTouch(strings.Join(out, "\n"))), nil
	})
	if err != nil {
		return Task{}, err
	}

	// 2. 移動先のセクション末尾に追加
	var newLine int
	err = ws.Mutate(destFile, func(content []byte, exists bool) ([]byte, error) {
		text := string(content)
		if !exists {
			if dest.Kind != "daily" {
				return nil, fmt.Errorf("destination not found: %s", destFile)
			}
			text = newDailySkeleton(dest.Date)
		}
		out, ln := insertIntoSection(text, destSection, taskLine)
		newLine = ln
		return []byte(workspaceTouch(out)), nil
	})
	if err != nil {
		// 移動元からは消えている。失敗を握り潰すと行が消失するため、復元を試みる
		_ = ws.Mutate(file, func(content []byte, exists bool) ([]byte, error) {
			if !exists {
				return nil, nil
			}
			return []byte(strings.TrimRight(string(content), "\n") + "\n" + taskLine + "\n"), nil
		})
		return Task{}, err
	}

	m := checkboxRe.FindStringSubmatch(taskLine)
	return Task{
		Text: strings.TrimSpace(m[2]), State: stateOf(m[1]),
		File: destFile, Line: newLine, Section: destSection,
	}, nil
}

// SetToday は行の @today タグを付け外しする（今日やる ⇄ ストックの切替）。
// 行はその場（= project ファイル）で書き換わるので project 紐付きは保たれる。
func SetToday(ws *workspace.Workspace, file string, line int, expectText string, on bool) (Task, error) {
	var out Task
	err := ws.Mutate(file, func(content []byte, exists bool) ([]byte, error) {
		if !exists {
			return nil, fmt.Errorf("file not found: %s", file)
		}
		lines := strings.Split(string(content), "\n")
		if line < 1 || line > len(lines) {
			return nil, fmt.Errorf("%w: line %d out of range", ErrLineMismatch, line)
		}
		m := checkboxRe.FindStringSubmatch(lines[line-1])
		if m == nil || strings.TrimSpace(m[2]) != strings.TrimSpace(expectText) {
			return nil, fmt.Errorf("%w: %s:%d", ErrLineMismatch, file, line)
		}
		newText := toggleToday(strings.TrimSpace(m[2]), on)
		out = rewriteLine(lines, line, m[1], newText, file)
		return []byte(workspaceTouch(strings.Join(lines, "\n"))), nil
	})
	return out, err
}

// projectTarget は project 名から書き込み先（ファイル, セクション）を返す。
// project が空なら todo.md ## Now（project 外の即席 today）。
func projectTarget(project string) (file, section string) {
	if project == "" {
		return "todo.md", "Now"
	}
	return fmt.Sprintf("projects/%s/README.md", project), "タスク"
}

// buildBody は表示テキストに行内タグを固定順で付けた「- [mark] 」以降の本文を作る。
func buildBody(display string, today bool, due, ms, pTag string) string {
	parts := []string{strings.TrimSpace(display)}
	if today {
		parts = append(parts, "@today")
	}
	if due != "" {
		parts = append(parts, "@due("+due+")")
	}
	if ms != "" {
		parts = append(parts, "@ms("+ms+")")
	}
	if pTag != "" {
		parts = append(parts, pTag) // pTag は "@p(1000)" 形式
	}
	return strings.Join(parts, " ")
}

func taskFromBody(mark, body, file string, line int, project string) Task {
	tg := parseInline(body)
	return Task{
		Text: body, Display: tg.Display, State: stateOf(mark), File: file, Line: line, Project: project,
		Today: tg.Today, Due: tg.Due, Milestone: tg.Milestone, Priority: tg.Priority,
	}
}

// AddLine は file の指定セクション末尾にチェックボックス行を 1 件追加する（マイルストーン追加など）。
func AddLine(ws *workspace.Workspace, file, section, display string) (Task, error) {
	if strings.TrimSpace(display) == "" {
		return Task{}, fmt.Errorf("display must not be empty")
	}
	body := strings.TrimSpace(display)
	var out Task
	err := ws.Mutate(file, func(content []byte, exists bool) ([]byte, error) {
		if !exists {
			return nil, fmt.Errorf("file not found: %s", file)
		}
		text, ln := insertIntoSection(string(content), section, "- [ ] "+body)
		out = taskFromBody(" ", body, file, ln, "")
		return []byte(workspaceTouch(text)), nil
	})
	return out, err
}

// CreateTask は project（空なら todo.md ## Now）にタスクを 1 件追加する。
func CreateTask(ws *workspace.Workspace, project, display string, today bool, due, ms string) (Task, error) {
	if strings.TrimSpace(display) == "" {
		return Task{}, fmt.Errorf("display must not be empty")
	}
	destFile, destSection := projectTarget(project)
	body := buildBody(display, today, due, ms, "")
	var out Task
	err := ws.Mutate(destFile, func(content []byte, exists bool) ([]byte, error) {
		if !exists {
			return nil, fmt.Errorf("destination not found: %s", destFile)
		}
		text, ln := insertIntoSection(string(content), destSection, "- [ ] "+body)
		out = taskFromBody(" ", body, destFile, ln, project)
		return []byte(workspaceTouch(text)), nil
	})
	return out, err
}

// EditTask はタスクを編集する。project が変わる場合はファイル間移動になる。
// 既存の @p（優先度）は引き継ぐ。本文・@today・@due・@ms はフォームの値で置換。
func EditTask(ws *workspace.Workspace, file string, line int, expectText, project, display string, today bool, due, ms string) (Task, error) {
	if strings.TrimSpace(display) == "" {
		return Task{}, fmt.Errorf("display must not be empty")
	}
	destFile, destSection := projectTarget(project)

	// 同一ファイル: その場で書き換え
	if destFile == file {
		var out Task
		err := ws.Mutate(file, func(content []byte, exists bool) ([]byte, error) {
			if !exists {
				return nil, fmt.Errorf("file not found: %s", file)
			}
			lines := strings.Split(string(content), "\n")
			if line < 1 || line > len(lines) {
				return nil, fmt.Errorf("%w: line %d out of range", ErrLineMismatch, line)
			}
			m := checkboxRe.FindStringSubmatch(lines[line-1])
			if m == nil || strings.TrimSpace(m[2]) != strings.TrimSpace(expectText) {
				return nil, fmt.Errorf("%w: %s:%d", ErrLineMismatch, file, line)
			}
			body := buildBody(display, today, due, ms, pRe.FindString(strings.TrimSpace(m[2])))
			out = rewriteLine(lines, line, m[1], body, file)
			out.Project = project
			return []byte(workspaceTouch(strings.Join(lines, "\n"))), nil
		})
		return out, err
	}

	// project 変更: 移動元から削除 → 移動先へ挿入（@p 引き継ぎ）
	var body, mark string
	err := ws.Mutate(file, func(content []byte, exists bool) ([]byte, error) {
		if !exists {
			return nil, fmt.Errorf("source not found: %s", file)
		}
		lines := strings.Split(string(content), "\n")
		if line < 1 || line > len(lines) {
			return nil, fmt.Errorf("%w: line %d out of range", ErrLineMismatch, line)
		}
		m := checkboxRe.FindStringSubmatch(lines[line-1])
		if m == nil || strings.TrimSpace(m[2]) != strings.TrimSpace(expectText) {
			return nil, fmt.Errorf("%w: %s:%d", ErrLineMismatch, file, line)
		}
		mark = m[1]
		body = buildBody(display, today, due, ms, pRe.FindString(strings.TrimSpace(m[2])))
		out := append(lines[:line-1:line-1], lines[line:]...)
		return []byte(workspaceTouch(strings.Join(out, "\n"))), nil
	})
	if err != nil {
		return Task{}, err
	}
	newLineText := "- [" + mark + "] " + body
	var newLine int
	err = ws.Mutate(destFile, func(content []byte, exists bool) ([]byte, error) {
		if !exists {
			return nil, fmt.Errorf("destination not found: %s", destFile)
		}
		text, ln := insertIntoSection(string(content), destSection, newLineText)
		newLine = ln
		return []byte(workspaceTouch(text)), nil
	})
	if err != nil {
		// 挿入失敗時は移動元へ復元（行消失を防ぐ）
		_ = ws.Mutate(file, func(content []byte, exists bool) ([]byte, error) {
			if !exists {
				return nil, nil
			}
			return []byte(strings.TrimRight(string(content), "\n") + "\n" + newLineText + "\n"), nil
		})
		return Task{}, err
	}
	return taskFromBody(mark, body, destFile, newLine, project), nil
}

// SetText はタスクの表示テキストを書き換える（インライン編集）。
// 行内タグ（@today/@due/@ms/@p）は維持し、本文だけ差し替える。
func SetText(ws *workspace.Workspace, file string, line int, expectText, newDisplay string) (Task, error) {
	var out Task
	err := ws.Mutate(file, func(content []byte, exists bool) ([]byte, error) {
		if !exists {
			return nil, fmt.Errorf("file not found: %s", file)
		}
		lines := strings.Split(string(content), "\n")
		if line < 1 || line > len(lines) {
			return nil, fmt.Errorf("%w: line %d out of range", ErrLineMismatch, line)
		}
		m := checkboxRe.FindStringSubmatch(lines[line-1])
		if m == nil || strings.TrimSpace(m[2]) != strings.TrimSpace(expectText) {
			return nil, fmt.Errorf("%w: %s:%d", ErrLineMismatch, file, line)
		}
		nd := strings.TrimSpace(newDisplay)
		if nd == "" {
			return nil, fmt.Errorf("display must not be empty")
		}
		if suffix := tagSuffix(strings.TrimSpace(m[2])); suffix != "" {
			nd += " " + suffix
		}
		out = rewriteLine(lines, line, m[1], nd, file)
		return []byte(workspaceTouch(strings.Join(lines, "\n"))), nil
	})
	return out, err
}

// tagSuffix は行テキストに含まれる行内タグを固定順（today→due→ms→p）で 1 文字列にまとめる。
func tagSuffix(raw string) string {
	var parts []string
	if todayRe.MatchString(raw) {
		parts = append(parts, "@today")
	}
	if s := dueRe.FindString(raw); s != "" {
		parts = append(parts, s)
	}
	if s := msRe.FindString(raw); s != "" {
		parts = append(parts, s)
	}
	if s := pRe.FindString(raw); s != "" {
		parts = append(parts, s)
	}
	return strings.Join(parts, " ")
}

// SetDue は行の @due(YYYY-MM-DD) タグを設定する（空文字で除去）。
func SetDue(ws *workspace.Workspace, file string, line int, expectText, due string) (Task, error) {
	var out Task
	err := ws.Mutate(file, func(content []byte, exists bool) ([]byte, error) {
		if !exists {
			return nil, fmt.Errorf("file not found: %s", file)
		}
		lines := strings.Split(string(content), "\n")
		if line < 1 || line > len(lines) {
			return nil, fmt.Errorf("%w: line %d out of range", ErrLineMismatch, line)
		}
		m := checkboxRe.FindStringSubmatch(lines[line-1])
		if m == nil || strings.TrimSpace(m[2]) != strings.TrimSpace(expectText) {
			return nil, fmt.Errorf("%w: %s:%d", ErrLineMismatch, file, line)
		}
		out = rewriteLine(lines, line, m[1], setDueTag(strings.TrimSpace(m[2]), due), file)
		return []byte(workspaceTouch(strings.Join(lines, "\n"))), nil
	})
	return out, err
}

// setDueTag は行テキストの @due(...) を差し替える（空文字で除去）。タグは行末に置く。
func setDueTag(text, due string) string {
	t := dueRe.ReplaceAllString(text, "")
	t = strings.TrimSpace(multiSpace.ReplaceAllString(t, " "))
	if due != "" {
		return t + " @due(" + due + ")"
	}
	return t
}

// SetPriority は行の @p(N) タグを設定する（n<=0 で除去）。ストックの並べ替えに使う。
func SetPriority(ws *workspace.Workspace, file string, line int, expectText string, n int) (Task, error) {
	var out Task
	err := ws.Mutate(file, func(content []byte, exists bool) ([]byte, error) {
		if !exists {
			return nil, fmt.Errorf("file not found: %s", file)
		}
		lines := strings.Split(string(content), "\n")
		if line < 1 || line > len(lines) {
			return nil, fmt.Errorf("%w: line %d out of range", ErrLineMismatch, line)
		}
		m := checkboxRe.FindStringSubmatch(lines[line-1])
		if m == nil || strings.TrimSpace(m[2]) != strings.TrimSpace(expectText) {
			return nil, fmt.Errorf("%w: %s:%d", ErrLineMismatch, file, line)
		}
		out = rewriteLine(lines, line, m[1], setPriorityTag(strings.TrimSpace(m[2]), n), file)
		return []byte(workspaceTouch(strings.Join(lines, "\n"))), nil
	})
	return out, err
}

// rewriteLine は line 行目を「- [mark] newText」で書き換え、解析済み Task を返す。
func rewriteLine(lines []string, line int, mark, newText, file string) Task {
	indent := lines[line-1][:strings.Index(lines[line-1], "- [")]
	lines[line-1] = indent + "- [" + mark + "] " + newText
	tg := parseInline(newText)
	return Task{
		Text: newText, Display: tg.Display, State: stateOf(mark), File: file, Line: line,
		Today: tg.Today, Due: tg.Due, Milestone: tg.Milestone, Priority: tg.Priority,
	}
}

// toggleToday は行テキストの末尾の @today を付け外しする。
func toggleToday(text string, on bool) string {
	has := todayRe.MatchString(text)
	if on {
		if has {
			return text
		}
		return strings.TrimRight(text, " ") + " @today"
	}
	if !has {
		return text
	}
	t := todayRe.ReplaceAllString(text, "")
	return strings.TrimSpace(multiSpace.ReplaceAllString(t, " "))
}

// setPriorityTag は行テキストの @p(N) を差し替える（n<=0 で除去）。タグは行末に置く。
func setPriorityTag(text string, n int) string {
	t := pRe.ReplaceAllString(text, "")
	t = strings.TrimSpace(multiSpace.ReplaceAllString(t, " "))
	if n > 0 {
		return t + " @p(" + strconv.Itoa(n) + ")"
	}
	return t
}

// DeleteLine はチェックボックス行を 1 行削除する（楽観ロック付き）。
func DeleteLine(ws *workspace.Workspace, file string, line int, expectText string) error {
	return ws.Mutate(file, func(content []byte, exists bool) ([]byte, error) {
		if !exists {
			return nil, fmt.Errorf("file not found: %s", file)
		}
		lines := strings.Split(string(content), "\n")
		if line < 1 || line > len(lines) {
			return nil, fmt.Errorf("%w: line %d out of range", ErrLineMismatch, line)
		}
		m := checkboxRe.FindStringSubmatch(lines[line-1])
		if m == nil || strings.TrimSpace(m[2]) != strings.TrimSpace(expectText) {
			return nil, fmt.Errorf("%w: %s:%d", ErrLineMismatch, file, line)
		}
		// full slice expression で元 slice の clobber を防ぐ
		out := append(lines[:line-1:line-1], lines[line:]...)
		return []byte(workspaceTouch(strings.Join(out, "\n"))), nil
	})
}

// SetState はチェックボックスの状態を書き換える（todo / done / skipped）。
func SetState(ws *workspace.Workspace, file string, line int, expectText, state string) (Task, error) {
	var mark string
	switch state {
	case "todo":
		mark = " "
	case "done":
		mark = "x"
	case "skipped":
		mark = "-"
	default:
		return Task{}, fmt.Errorf("unknown state: %q", state)
	}
	var out Task
	err := ws.Mutate(file, func(content []byte, exists bool) ([]byte, error) {
		if !exists {
			return nil, fmt.Errorf("file not found: %s", file)
		}
		lines := strings.Split(string(content), "\n")
		if line < 1 || line > len(lines) {
			return nil, fmt.Errorf("%w: line %d out of range", ErrLineMismatch, line)
		}
		m := checkboxRe.FindStringSubmatch(lines[line-1])
		if m == nil || strings.TrimSpace(m[2]) != strings.TrimSpace(expectText) {
			return nil, fmt.Errorf("%w: %s:%d", ErrLineMismatch, file, line)
		}
		indent := lines[line-1][:strings.Index(lines[line-1], "- [")]
		lines[line-1] = indent + "- [" + mark + "] " + strings.TrimSpace(m[2])
		out = Task{Text: strings.TrimSpace(m[2]), State: state, File: file, Line: line}
		return []byte(workspaceTouch(strings.Join(lines, "\n"))), nil
	})
	return out, err
}

// insertIntoSection は指定見出しセクションの末尾に行を挿入する。
// セクションが無ければファイル末尾に見出しごと作る。挿入後の行番号（1-based）を返す。
func insertIntoSection(content, section, line string) (string, int) {
	lines := strings.Split(content, "\n")
	start := -1
	for i, l := range lines {
		if h := headingRe.FindStringSubmatch(l); h != nil && h[2] == section {
			start = i
			break
		}
	}
	if start == -1 {
		// セクションが無い: 末尾に作成（daily の 完了タスク は h3、それ以外は h2）
		prefix := "## "
		if section == "完了タスク" {
			prefix = "### "
		}
		out := strings.TrimRight(content, "\n") + "\n\n" + prefix + section + "\n\n" + line + "\n"
		return out, len(strings.Split(out, "\n")) - 1
	}
	// セクションの終わり（次の見出し or EOF）を探す
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if headingRe.MatchString(lines[i]) {
			end = i
			break
		}
	}
	// セクション末尾の空行の手前に挿入する
	insert := end
	for insert > start+1 && strings.TrimSpace(lines[insert-1]) == "" {
		insert--
	}
	out := make([]string, 0, len(lines)+1)
	out = append(out, lines[:insert]...)
	out = append(out, line)
	out = append(out, lines[insert:]...)
	return strings.Join(out, "\n"), insert + 1
}

// newDailySkeleton は conventions.md に従った daily の骨組みを返す。
func newDailySkeleton(date time.Time) string {
	if date.IsZero() {
		date = time.Now()
	}
	d := date.Format("2006-01-02")
	return fmt.Sprintf(`---
type: daily
tags: []
created: %s
updated: %s
---

# %s

## 日記
`, d, d, d)
}

// workspaceTouch は frontmatter の updated を今日に更新する。
func workspaceTouch(content string) string {
	return string(workspace.TouchUpdated([]byte(content), time.Now()))
}

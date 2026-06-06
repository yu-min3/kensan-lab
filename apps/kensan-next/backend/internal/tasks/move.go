package tasks

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yu-min3/kensan-lab/apps/kensan-next/backend/internal/workspace"
)

// ErrLineMismatch は指定行の内容が期待と異なる場合（他クライアントが先に編集した）。
var ErrLineMismatch = errors.New("line content mismatch (file changed by another client?)")

// Dest はタスク行の移動先。かんばんの列に対応する。
type Dest struct {
	Kind    string // today | stock | daily
	Project string // Kind=stock のとき必須
	Date    time.Time
}

func (d Dest) resolve() (file, section string, err error) {
	switch d.Kind {
	case "today":
		return "todo.md", "Now", nil
	case "stock":
		if d.Project == "" {
			return "", "", errors.New("project is required for dest=stock")
		}
		return fmt.Sprintf("projects/%s/README.md", d.Project), "タスク", nil
	case "daily":
		t := d.Date
		if t.IsZero() {
			t = time.Now()
		}
		return fmt.Sprintf("daily/%04d/%02d/%02d.md", t.Year(), t.Month(), t.Day()), "完了タスク", nil
	default:
		return "", "", fmt.Errorf("unknown dest: %q", d.Kind)
	}
}

// Move はチェックボックス行をファイル間で移動する。
// かんばんのドラッグも /morning のタスク配置も、この 1 つの操作に帰着する。
//
// 楽観ロック: line 行目のテキストが expectText と一致しない場合は ErrLineMismatch。
// （Claude / VSCode が同じファイルを先に編集したケースの検出）
func Move(ws *workspace.Workspace, file string, line int, expectText string, dest Dest) (Task, error) {
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
		File: destFile, Line: newLine, Project: dest.Project, Section: destSection,
	}, nil
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

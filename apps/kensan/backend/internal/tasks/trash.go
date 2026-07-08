package tasks

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/yu-min3/kensan-lab/apps/kensan/backend/internal/workspace"
)

// TrashFile は app で削除したタスク行の退避先。
// ドット始まりディレクトリのため workspace.Scan / 全文検索 / タグ集計には現れない
// （普段は目立たず、app の /trash からだけ見える）。
const TrashFile = ".kensan/trash.md"

var (
	fromRe    = regexp.MustCompile(`@from\(([^)#]*)#([^)]*)\)`)
	deletedRe = regexp.MustCompile(`@deleted\((\d{4}-\d{2}-\d{2})\)`)
)

// TrashEntry は trash.md の 1 行。Line + Raw が復元・完全削除の locator（楽観ロック）。
type TrashEntry struct {
	Text    string `json:"text"`              // 復元される本文（@due 等の行内タグ込み）
	Display string `json:"display"`           // 表示用（行内タグ除去）
	State   string `json:"state"`             // todo | done | skipped
	From    string `json:"from,omitempty"`    // 元ファイル（workspace 相対）
	Section string `json:"section,omitempty"` // 元セクション
	Deleted string `json:"deleted,omitempty"` // YYYY-MM-DD
	Line    int    `json:"line"`              // trash.md 内の行番号（1-based）
	Raw     string `json:"raw"`               // trash 行の本文（照合用）
}

// stripTrashTags は @from / @deleted を除いた本文を返す（@due 等は残す）。
func stripTrashTags(raw string) string {
	t := fromRe.ReplaceAllString(raw, "")
	t = deletedRe.ReplaceAllString(t, "")
	return strings.TrimSpace(multiSpace.ReplaceAllString(t, " "))
}

// appendTrashEntry は trash.md 末尾に 1 行追記する。ファイルが無ければ骨組みごと作る。
func appendTrashEntry(ws *workspace.Workspace, mark, body, fromFile, fromSection string, now time.Time) error {
	d := now.Format("2006-01-02")
	entry := "- [" + mark + "] " + body + " @from(" + fromFile + "#" + fromSection + ") @deleted(" + d + ")"
	return ws.Mutate(TrashFile, func(content []byte, exists bool) ([]byte, error) {
		text := string(content)
		if !exists {
			text = fmt.Sprintf(`---
type: trash
created: %s
updated: %s
---

# ゴミ箱

app で削除したタスクの退避先。復元・完全削除は app の /trash から。
`, d, d)
		}
		return []byte(workspaceTouch(strings.TrimRight(text, "\n") + "\n" + entry + "\n")), nil
	})
}

// TrashList は trash.md のエントリを新しい順（= 追記の逆順）で返す。ファイルが無ければ空。
func TrashList(ws *workspace.Workspace) ([]TrashEntry, error) {
	_, content, err := ws.Read(TrashFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []TrashEntry{}, nil
		}
		return nil, err
	}
	var out []TrashEntry
	for i, l := range strings.Split(string(content), "\n") {
		m := checkboxRe.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		raw := strings.TrimSpace(m[2])
		e := TrashEntry{State: stateOf(m[1]), Line: i + 1, Raw: raw}
		if fm := fromRe.FindStringSubmatch(raw); fm != nil {
			e.From = strings.TrimSpace(fm[1])
			e.Section = strings.TrimSpace(fm[2])
		}
		if dm := deletedRe.FindStringSubmatch(raw); dm != nil {
			e.Deleted = dm[1]
		}
		e.Text = stripTrashTags(raw)
		e.Display = parseInline(e.Text).Display
		out = append(out, e)
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

// removeTrashLine は trash.md から line 行目を取り除き、その行の (mark, raw) を返す（楽観ロック付き）。
func removeTrashLine(ws *workspace.Workspace, line int, expectRaw string) (mark, raw string, err error) {
	err = ws.Mutate(TrashFile, func(content []byte, exists bool) ([]byte, error) {
		if !exists {
			return nil, fmt.Errorf("trash not found: %s", TrashFile)
		}
		lines := strings.Split(string(content), "\n")
		if line < 1 || line > len(lines) {
			return nil, fmt.Errorf("%w: line %d out of range", ErrLineMismatch, line)
		}
		m := checkboxRe.FindStringSubmatch(lines[line-1])
		if m == nil || strings.TrimSpace(m[2]) != strings.TrimSpace(expectRaw) {
			return nil, fmt.Errorf("%w: %s:%d", ErrLineMismatch, TrashFile, line)
		}
		mark, raw = m[1], strings.TrimSpace(m[2])
		out := append(lines[:line-1:line-1], lines[line:]...)
		return []byte(workspaceTouch(strings.Join(out, "\n"))), nil
	})
	return mark, raw, err
}

// RestoreFromTrash は trash.md のエントリを元のファイル・セクションへ戻す。
// 元ファイルが消えている（プロジェクトの archive 等）・セクション情報が無い場合は
// todo.md ## Now へフォールバックする。
func RestoreFromTrash(ws *workspace.Workspace, line int, expectRaw string) (Task, error) {
	mark, raw, err := removeTrashLine(ws, line, expectRaw)
	if err != nil {
		return Task{}, err
	}

	body := stripTrashTags(raw)
	destFile, destSection := "todo.md", "Now"
	if fm := fromRe.FindStringSubmatch(raw); fm != nil {
		file, section := strings.TrimSpace(fm[1]), strings.TrimSpace(fm[2])
		if file != "" && section != "" {
			if abs, aerr := ws.Abs(file); aerr == nil {
				if _, serr := os.Stat(abs); serr == nil {
					destFile, destSection = file, section
				}
			}
		}
	}

	newLineText := "- [" + mark + "] " + body
	var newLine int
	err = ws.Mutate(destFile, func(content []byte, exists bool) ([]byte, error) {
		if !exists {
			return nil, fmt.Errorf("restore destination not found: %s", destFile)
		}
		text, ln := insertIntoSection(string(content), destSection, newLineText)
		newLine = ln
		return []byte(workspaceTouch(text)), nil
	})
	if err != nil {
		// 挿入失敗時はゴミ箱へ戻す（行消失を防ぐ）
		_ = ws.Mutate(TrashFile, func(content []byte, exists bool) ([]byte, error) {
			if !exists {
				return nil, nil
			}
			return []byte(strings.TrimRight(string(content), "\n") + "\n- [" + mark + "] " + raw + "\n"), nil
		})
		return Task{}, err
	}
	return taskFromBody(mark, body, destFile, newLine, projectOf(destFile)), nil
}

// PurgeTrashEntry は trash.md からエントリを完全に削除する（復元不可・楽観ロック付き）。
func PurgeTrashEntry(ws *workspace.Workspace, line int, expectRaw string) error {
	_, _, err := removeTrashLine(ws, line, expectRaw)
	return err
}

// projectOf は projects/<name>/README.md から project 名を得る（それ以外は空）。
func projectOf(file string) string {
	parts := strings.Split(file, "/")
	if len(parts) == 3 && parts[0] == "projects" && parts[2] == "README.md" {
		return parts[1]
	}
	return ""
}

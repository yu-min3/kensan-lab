// Package projects は projects/<name>/README.md を構造化して読む。
//
// 生の README をそのまま出すのではなく、frontmatter（status/deadline/repo）と
// セクション（## 目標 / ## マイルストーン / ## タスク / ## ログ / ## 関連ノート）を
// パースして、一覧サマリと詳細を組み立てる（List·Detail UI 用）。
package projects

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/yu-min3/kensan-lab/apps/kensan/backend/internal/tasks"
	"github.com/yu-min3/kensan-lab/apps/kensan/backend/internal/workspace"
)

var nameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

var headingRe = regexp.MustCompile(`^(#{1,6})\s+(.+?)\s*$`)
var logLineRe = regexp.MustCompile(`^-\s*(\d{4}-\d{2}-\d{2})\s*[:：]?\s*(.*)$`)
var wikiRe = regexp.MustCompile(`\[\[([^\]|]+)(?:\|([^\]]+))?\]\]`)

// Summary は一覧用の 1 プロジェクト分。
type Summary struct {
	Name            string `json:"name"`
	Status          string `json:"status"`
	Deadline        string `json:"deadline,omitempty"`
	Goal            string `json:"goal"`
	MilestonesDone  int    `json:"milestonesDone"`
	MilestonesTotal int    `json:"milestonesTotal"`
	OpenTasks       int    `json:"openTasks"`
}

// LogEntry は ## ログ の 1 エントリ（日付 + 本文）。
type LogEntry struct {
	Date string `json:"date,omitempty"`
	Text string `json:"text"`
}

// NoteRef は ## 関連ノート の 1 リンク。Target は app で開けるパス（.md）、無ければ外部テキスト。
type NoteRef struct {
	Group  string `json:"group,omitempty"`
	Target string `json:"target,omitempty"`
	Label  string `json:"label"`
	Desc   string `json:"desc,omitempty"`
}

// Detail は詳細パネル用。
type Detail struct {
	Name       string       `json:"name"`
	Status     string       `json:"status"`
	Deadline   string       `json:"deadline,omitempty"`
	Repo       string       `json:"repo,omitempty"`
	Overview   string       `json:"overview"`
	Goal       string       `json:"goal"`
	Milestones []tasks.Task `json:"milestones"`
	Tasks      []tasks.Task `json:"tasks"`
	Log        []LogEntry   `json:"log"`
	Notes      []NoteRef    `json:"notes"`
}

func readme(root, name string) (string, error) {
	// name は GET 詳細でユーザ入力になり得るため検証（path traversal の defense-in-depth）
	if !nameRe.MatchString(name) {
		return "", fmt.Errorf("invalid project name: %q", name)
	}
	b, err := os.ReadFile(filepath.Join(root, "projects", name, "README.md"))
	return string(b), err
}

// Summaries は全アクティブ含む全プロジェクトのサマリを返す。
// 並び: active 優先 → 締切が近い順（無しは後ろ）→ 名前。
func Summaries(root string) []Summary {
	var out []Summary
	for _, name := range tasks.Projects(root) {
		content, err := readme(root, name)
		if err != nil {
			continue
		}
		fm := frontmatter(content)
		s := Summary{Name: name, Status: fm["status"], Deadline: fm["deadline"], Goal: firstLine(section(content, "目標"))}
		for _, t := range tasks.ExtractLines(content, "") {
			switch t.Section {
			case "マイルストーン":
				s.MilestonesTotal++
				if t.State == "done" {
					s.MilestonesDone++
				}
			case "タスク":
				if t.State == "todo" {
					s.OpenTasks++
				}
			}
		}
		out = append(out, s)
	}
	sort.SliceStable(out, func(i, j int) bool {
		ai, aj := out[i].Status == "active", out[j].Status == "active"
		if ai != aj {
			return ai
		}
		di, dj := out[i].Deadline, out[j].Deadline
		if (di == "") != (dj == "") {
			return di != "" // 締切ありを前に
		}
		if di != dj {
			return di < dj
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// Load は 1 プロジェクトの詳細を返す。
func Load(root, name string) (Detail, error) {
	content, err := readme(root, name)
	if err != nil {
		return Detail{}, err
	}
	fm := frontmatter(content)
	d := Detail{
		Name: name, Status: fm["status"], Deadline: fm["deadline"], Repo: fm["repo"],
		Overview: strings.TrimSpace(section(content, "概要")),
		Goal:     strings.TrimSpace(section(content, "目標")),
		Log:      parseLog(section(content, "ログ")),
		Notes:    parseNotes(sectionPrefix(content, "関連ノート")),
	}
	rel := filepath.ToSlash(filepath.Join("projects", name, "README.md"))
	for _, t := range tasks.ExtractLines(content, rel) {
		t.Project = name
		switch t.Section {
		case "マイルストーン":
			d.Milestones = append(d.Milestones, t)
		case "タスク":
			d.Tasks = append(d.Tasks, t)
		}
	}
	return d, nil
}

// Update は status / deadline（frontmatter）と目標（## 目標）を更新する。
// deadline が空文字なら frontmatter から削除する。
func Update(ws *workspace.Workspace, name, status, deadline, goal string) error {
	file := filepath.ToSlash(filepath.Join("projects", name, "README.md"))
	return ws.Mutate(file, func(content []byte, exists bool) ([]byte, error) {
		if !exists {
			return nil, fmt.Errorf("project not found: %s", name)
		}
		s := string(content)
		s = setFrontmatterField(s, "status", status)
		s = setFrontmatterField(s, "deadline", deadline)
		s = replaceSection(s, "目標", goal)
		return []byte(s), nil
	})
}

// Create はテンプレート付きで projects/<name>/README.md を新規作成する。
func Create(ws *workspace.Workspace, name string, now time.Time) error {
	if !nameRe.MatchString(name) {
		return fmt.Errorf("invalid project name: %q（英小文字・数字・ハイフン）", name)
	}
	file := filepath.ToSlash(filepath.Join("projects", name, "README.md"))
	d := now.Format("2006-01-02")
	tmpl := fmt.Sprintf(`---
type: project
status: active
created: %s
updated: %s
---

## 概要

## 目標

## マイルストーン

## タスク

## ログ

## 関連ノート・リソース
`, d, d)
	return ws.Create(file, []byte(tmpl))
}

// setFrontmatterField は frontmatter の key を value に設定する（value 空で削除）。
func setFrontmatterField(content, key, value string) string {
	if !strings.HasPrefix(content, "---\n") {
		return content
	}
	rest := content[4:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return content
	}
	after := rest[end:] // "\n---..." 以降
	var out []string
	found := false
	for _, l := range strings.Split(rest[:end], "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), key+":") {
			found = true
			if value != "" {
				out = append(out, key+": "+value)
			}
			continue
		}
		out = append(out, l)
	}
	if !found && value != "" {
		out = append(out, key+": "+value)
	}
	return "---\n" + strings.Join(out, "\n") + after
}

// replaceSection は ## heading の本文を body に差し替える（見出しは保持。無ければ末尾に追加）。
func replaceSection(content, heading, body string) string {
	lines := strings.Split(content, "\n")
	start, level := -1, 0
	for i, l := range lines {
		if h := headingRe.FindStringSubmatch(l); h != nil && h[2] == heading {
			start, level = i, len(h[1])
			break
		}
	}
	body = strings.TrimSpace(body)
	if start == -1 {
		return strings.TrimRight(content, "\n") + "\n\n## " + heading + "\n\n" + body + "\n"
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if h := headingRe.FindStringSubmatch(lines[i]); h != nil && len(h[1]) <= level {
			end = i
			break
		}
	}
	out := append([]string{}, lines[:start+1]...)
	out = append(out, "", body, "")
	out = append(out, lines[end:]...)
	return strings.Join(out, "\n")
}

// --- パースヘルパー ---

// sectionBy は match に合致する見出しの本文を返す（同レベル以上の見出しで終端、深い見出しは含む）。
func sectionBy(content string, match func(string) bool) string {
	lines := strings.Split(content, "\n")
	start, startLevel := -1, 0
	for i, l := range lines {
		if h := headingRe.FindStringSubmatch(l); h != nil && match(h[2]) {
			start, startLevel = i, len(h[1])
			break
		}
	}
	if start == -1 {
		return ""
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if h := headingRe.FindStringSubmatch(lines[i]); h != nil && len(h[1]) <= startLevel {
			end = i
			break
		}
	}
	return strings.Join(lines[start+1:end], "\n")
}

func section(content, heading string) string {
	return sectionBy(content, func(h string) bool { return h == heading })
}

func sectionPrefix(content, prefix string) string {
	return sectionBy(content, func(h string) bool { return strings.HasPrefix(h, prefix) })
}

func frontmatter(content string) map[string]string {
	m := map[string]string{}
	if !strings.HasPrefix(content, "---") {
		return m
	}
	rest := content[3:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return m
	}
	for _, line := range strings.Split(rest[:end], "\n") {
		if i := strings.Index(line, ":"); i > 0 {
			k := strings.TrimSpace(line[:i])
			v := strings.Trim(strings.TrimSpace(line[i+1:]), `"`)
			if k != "" {
				m[k] = v
			}
		}
	}
	return m
}

func firstLine(s string) string {
	for _, l := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(l); t != "" {
			return t
		}
	}
	return ""
}

// parseLog は ## ログ を「- YYYY-MM-DD: 本文」のエントリ単位に分け、日付降順で返す。
func parseLog(s string) []LogEntry {
	var out []LogEntry
	for _, line := range strings.Split(s, "\n") {
		t := strings.TrimSpace(line)
		if m := logLineRe.FindStringSubmatch(t); m != nil {
			out = append(out, LogEntry{Date: m[1], Text: strings.TrimSpace(m[2])})
		} else if strings.HasPrefix(t, "- ") {
			out = append(out, LogEntry{Text: strings.TrimSpace(t[2:])})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if (out[i].Date == "") != (out[j].Date == "") {
			return out[i].Date != "" // 日付ありを前に
		}
		return out[i].Date > out[j].Date // 新しい順
	})
	return out
}

// parseNotes は ## 関連ノート の箇条書きを NoteRef に分解する。### は Group として保持。
// [[target|label]] — desc を解釈し、target は app で開ける .md パスにする。
func parseNotes(s string) []NoteRef {
	var out []NoteRef
	group := ""
	for _, line := range strings.Split(s, "\n") {
		if h := headingRe.FindStringSubmatch(line); h != nil {
			group = h[2]
			continue
		}
		t := strings.TrimSpace(line)
		if !strings.HasPrefix(t, "- ") && !strings.HasPrefix(t, "* ") {
			continue
		}
		item := strings.TrimSpace(t[2:])
		ref := NoteRef{Group: group}
		if m := wikiRe.FindStringSubmatch(item); m != nil {
			ref.Target = m[1]
			if !strings.HasSuffix(ref.Target, ".md") {
				ref.Target += ".md"
			}
			ref.Label = m[2]
			if ref.Label == "" {
				ref.Label = m[1]
			}
			if idx := strings.Index(item, "]]"); idx >= 0 {
				ref.Desc = strings.TrimLeft(strings.TrimSpace(item[idx+2:]), "—- ")
			}
		} else {
			ref.Label = item
		}
		out = append(out, ref)
	}
	return out
}

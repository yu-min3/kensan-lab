// Package tasks は Markdown チェックボックス行をタスクとして抽出する。
//
// ファイル契約（unification-plan.md / conventions.md）:
//
//	projects/<name>/README.md  ## タスク        ← ストック（project 紐付き）
//	projects/<name>/README.md  ## マイルストーン ← マイルストーン
//	projects/<name>/README.md  ## いつかやる     ← someday
//	todo.md                    ## Now           ← 今日やる
//
// タスクの単位は「行」。app のかんばん操作も Claude の /morning も
// 同じ「チェックボックス行のファイル間移動」として表現される。
package tasks

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	checkboxRe = regexp.MustCompile(`^\s*- \[([ x-])\] (.+)$`)
	headingRe  = regexp.MustCompile(`^(#{1,6})\s+(.+?)\s*$`)
)

// Task は 1 つのチェックボックス行。File + Line で SSoT 上の位置を一意に指す。
type Task struct {
	Text    string `json:"text"`
	State   string `json:"state"` // todo | done | skipped
	File    string `json:"file"`  // workspace 相対パス
	Line    int    `json:"line"`  // 1-based
	Project string `json:"project,omitempty"`
	Section string `json:"section,omitempty"`
}

// Board はかんばんの 1 画面分。Today と Stock が中核（ストック / 今日やる の分離）。
type Board struct {
	Today      []Task `json:"today"`
	Stock      []Task `json:"stock"`
	Someday    []Task `json:"someday"`
	Milestones []Task `json:"milestones"`
}

// ExtractLines は content からチェックボックス行を抽出する。直近の見出しを Section に入れる。
func ExtractLines(content string, file string) []Task {
	var out []Task
	section := ""
	for i, line := range strings.Split(content, "\n") {
		if h := headingRe.FindStringSubmatch(line); h != nil {
			section = h[2]
			continue
		}
		m := checkboxRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		out = append(out, Task{
			Text:    strings.TrimSpace(m[2]),
			State:   stateOf(m[1]),
			File:    file,
			Line:    i + 1,
			Section: section,
		})
	}
	return out
}

func stateOf(mark string) string {
	switch mark {
	case "x":
		return "done"
	case "-":
		return "skipped"
	default:
		return "todo"
	}
}

// Collect は workspace ルートからかんばん Board を組み立てる。
func Collect(root string) (Board, error) {
	b := Board{}

	// 今日やる: todo.md ## Now
	if content, err := os.ReadFile(filepath.Join(root, "todo.md")); err == nil {
		for _, t := range ExtractLines(string(content), "todo.md") {
			if t.Section == "Now" {
				b.Today = append(b.Today, t)
			}
		}
	}

	// ストック等: projects/<name>/README.md（_archive は除外）
	entries, err := os.ReadDir(filepath.Join(root, "projects"))
	if err != nil {
		return b, nil // projects/ が無くてもエラーにしない
	}
	var projects []string
	for _, e := range entries {
		if e.IsDir() && e.Name() != "_archive" && !strings.HasPrefix(e.Name(), ".") {
			projects = append(projects, e.Name())
		}
	}
	sort.Strings(projects)
	for _, p := range projects {
		rel := filepath.ToSlash(filepath.Join("projects", p, "README.md"))
		content, err := os.ReadFile(filepath.Join(root, "projects", p, "README.md"))
		if err != nil {
			continue
		}
		for _, t := range ExtractLines(string(content), rel) {
			t.Project = p
			switch t.Section {
			case "タスク":
				b.Stock = append(b.Stock, t)
			case "マイルストーン":
				b.Milestones = append(b.Milestones, t)
			case "いつかやる":
				b.Someday = append(b.Someday, t)
			}
			// ## ルーティン の行は `- [毎日]` 等の記法でチェックボックスに
			// マッチしないため自然に除外される
		}
	}
	return b, nil
}

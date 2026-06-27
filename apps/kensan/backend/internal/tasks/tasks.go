// Package tasks は Markdown チェックボックス行をタスクとして抽出する。
//
// ファイル契約（unification-plan.md / conventions.md）:
//
//	projects/<name>/README.md  ## タスク        ← タスク（project 紐付き）
//	projects/<name>/README.md  ## マイルストーン ← マイルストーン
//	projects/<name>/README.md  ## いつかやる     ← someday
//	todo.md                    ## Now           ← project に属さない即席 today
//
// タスクは project に住む。「今日やる / ストック」は場所ではなく行内タグのビュー:
//
//	@today          ← 今日やるレーンに浮上
//	@due(YYYY-MM-DD) ← 期限。今日以前なら今日やるに自動浮上
//	@ms(slug)       ← project 内マイルストーンへの紐付け（バッジ表示）
//	@p(N)           ← 優先度（整数・小さいほど上）。ストックの並び順。ドラッグで中間値に書換
//
// タスクの単位は「行」。ダッシュボードでチェック / @today 切替すると、その行を
// 持つ project ファイルが書き換わる（ダッシュボード完結 × project 紐付きの両立）。
package tasks

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	checkboxRe = regexp.MustCompile(`^\s*- \[([ x-])\] (.+)$`)
	headingRe  = regexp.MustCompile(`^(#{1,6})\s+(.+?)\s*$`)
	todayRe    = regexp.MustCompile(`@today\b`)
	dueRe      = regexp.MustCompile(`@due\(([^)]+)\)`)
	msRe       = regexp.MustCompile(`@ms\(([^)]+)\)`)
	pRe        = regexp.MustCompile(`@p\((\d+)\)`)
	multiSpace = regexp.MustCompile(`\s{2,}`)
)

// Task は 1 つのチェックボックス行。File + Line で SSoT 上の位置を一意に指す。
// Text は行内タグ込みの生テキスト（move / state 照合に使う）。Display は表示用。
type Task struct {
	Text      string `json:"text"`
	Display   string `json:"display"`
	State     string `json:"state"` // todo | done | skipped
	File      string `json:"file"`  // workspace 相対パス
	Line      int    `json:"line"`  // 1-based
	Project   string `json:"project,omitempty"`
	Section   string `json:"section,omitempty"`
	Today     bool   `json:"today"`               // @today
	Due       string `json:"due,omitempty"`       // @due(YYYY-MM-DD)
	Milestone string `json:"milestone,omitempty"` // @ms(slug)
	Priority  int    `json:"priority,omitempty"`  // @p(N)（0 = 未設定）
}

// Board はかんばんの 1 画面分。
// Today = @today / @due≤今日 のタスク + todo.md ## Now（即席）。
// Stock = project の未完了タスクのうち today でないもの。
type Board struct {
	Today      []Task `json:"today"`
	Stock      []Task `json:"stock"`
	Someday    []Task `json:"someday"`
	Milestones []Task `json:"milestones"`
}

// inlineTags は行テキストから抽出した行内タグ。
type inlineTags struct {
	Display   string
	Today     bool
	Due       string
	Milestone string
	Priority  int
}

// parseInline は行テキストから @today / @due / @ms / @p を抽出し、タグを除いた表示用テキストを返す。
func parseInline(text string) inlineTags {
	t := inlineTags{Today: todayRe.MatchString(text)}
	if m := dueRe.FindStringSubmatch(text); m != nil {
		t.Due = strings.TrimSpace(m[1])
	}
	if m := msRe.FindStringSubmatch(text); m != nil {
		t.Milestone = strings.TrimSpace(m[1])
	}
	if m := pRe.FindStringSubmatch(text); m != nil {
		t.Priority, _ = strconv.Atoi(m[1])
	}
	d := todayRe.ReplaceAllString(text, "")
	d = dueRe.ReplaceAllString(d, "")
	d = msRe.ReplaceAllString(d, "")
	d = pRe.ReplaceAllString(d, "")
	t.Display = strings.TrimSpace(multiSpace.ReplaceAllString(d, " "))
	return t
}

// ExtractLines は content からチェックボックス行を抽出する。直近の見出しを Section に入れ、
// 行内タグ（@today / @due / @ms）を解析する。
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
		raw := strings.TrimSpace(m[2])
		tg := parseInline(raw)
		out = append(out, Task{
			Text:      raw,
			Display:   tg.Display,
			State:     stateOf(m[1]),
			File:      file,
			Line:      i + 1,
			Section:   section,
			Today:     tg.Today,
			Due:       tg.Due,
			Milestone: tg.Milestone,
			Priority:  tg.Priority,
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

// isToday は @today か、@due が today 以前かを判定する。
func isToday(t Task, today string) bool {
	return t.Today || (t.Due != "" && t.Due <= today)
}

// Projects は projects/ 直下のプロジェクト名一覧を返す（_archive・ドット始まりは除外）。
func Projects(root string) []string {
	entries, err := os.ReadDir(filepath.Join(root, "projects"))
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() && e.Name() != "_archive" && !strings.HasPrefix(e.Name(), ".") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}

// Collect は workspace ルートからかんばん Board を組み立てる。
func Collect(root string) (Board, error) {
	return collect(root, time.Now().Format("2006-01-02"))
}

// collect は today（YYYY-MM-DD）を注入できるテスト用の内部実装。
func collect(root, today string) (Board, error) {
	b := Board{}

	// todo.md ## Now: project に属さない即席の「今日やる」
	if content, err := os.ReadFile(filepath.Join(root, "todo.md")); err == nil {
		for _, t := range ExtractLines(string(content), "todo.md") {
			if t.Section == "Now" {
				b.Today = append(b.Today, t)
			}
		}
	}

	// projects/<name>/README.md（_archive は除外）
	for _, p := range Projects(root) {
		rel := filepath.ToSlash(filepath.Join("projects", p, "README.md"))
		content, err := os.ReadFile(filepath.Join(root, "projects", p, "README.md"))
		if err != nil {
			continue
		}
		for _, t := range ExtractLines(string(content), rel) {
			t.Project = p
			switch t.Section {
			case "タスク":
				switch {
				case isToday(t, today):
					b.Today = append(b.Today, t) // @today / @due≤今日 は今日やるへ
				case t.State == "todo":
					b.Stock = append(b.Stock, t) // 未完了かつ today でない = ストック
				}
			case "マイルストーン":
				b.Milestones = append(b.Milestones, t)
			case "いつかやる":
				b.Someday = append(b.Someday, t)
			}
			// ## ルーティン の行は `- [毎日]` 等でチェックボックスにマッチせず除外される
		}
	}

	// ストックは @p(N) 昇順（小さいほど上）。未設定(0)は後ろに、元の並び順を保つ。
	sort.SliceStable(b.Stock, func(i, j int) bool {
		pi, pj := b.Stock[i].Priority, b.Stock[j].Priority
		switch {
		case pi == 0 && pj == 0:
			return false
		case pi == 0:
			return false // i 未設定 → 後ろ
		case pj == 0:
			return true // j 未設定 → i が前
		default:
			return pi < pj
		}
	})
	return b, nil
}

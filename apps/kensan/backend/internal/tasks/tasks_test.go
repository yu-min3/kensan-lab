package tasks

import (
	"os"
	"path/filepath"
	"testing"
)

const sampleReadme = `---
type: project
status: active
---

## 概要

テストプロジェクト。

## マイルストーン

- [x] CFP 提出
- [ ] スライド完成

## タスク

- [ ] 原稿レビュー依頼
- [x] アブストラクト確定
- [-] 没ネタ調査

## ルーティン

- [毎日] 英語 30 分
- [火,木] ジム

## いつかやる

- [ ] デモ環境の自動化
`

const sampleTodo = `---
type: todo
---
## Now

- [ ] 原稿レビュー依頼
- [x] 英語 30 分

## メモ

- [ ] Now の外にあるタスクは today に含めない
`

func TestExtractLines(t *testing.T) {
	got := ExtractLines(sampleReadme, "projects/demo/README.md")
	if len(got) != 6 {
		t.Fatalf("want 6 tasks (routines excluded), got %d: %+v", len(got), got)
	}
	// ルーティン記法 `- [毎日]` はチェックボックスとして抽出されない
	for _, task := range got {
		if task.Section == "ルーティン" {
			t.Errorf("routine line leaked into tasks: %+v", task)
		}
	}
	if got[0].Section != "マイルストーン" || got[0].State != "done" {
		t.Errorf("unexpected first task: %+v", got[0])
	}
	if got[4].State != "skipped" {
		t.Errorf("want skipped for '- [-]', got %+v", got[4])
	}
	if got[2].Line == 0 || got[2].File != "projects/demo/README.md" {
		t.Errorf("file/line missing: %+v", got[2])
	}
}

func TestCollect(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "todo.md"), sampleTodo)
	mustWrite(t, filepath.Join(root, "projects", "demo", "README.md"), sampleReadme)
	mustWrite(t, filepath.Join(root, "projects", "_archive", "old", "README.md"), sampleReadme)

	b, err := Collect(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Today) != 2 {
		t.Errorf("today: want 2 (todo.md ## Now), got %d: %+v", len(b.Today), b.Today)
	}
	// いつか(Later) = バンドタグ無しの未完了。## タスク の「原稿レビュー依頼」＋
	// ## いつかやる の「デモ環境の自動化」の 2 件（done/skipped は除外、_archive も除外）
	if len(b.Later) != 2 {
		t.Errorf("later: want 2 (## タスク + ## いつかやる の未完了), got %d: %+v", len(b.Later), b.Later)
	}
	if len(b.Milestones) != 2 {
		t.Errorf("milestones: got %d", len(b.Milestones))
	}
	for _, task := range b.Later {
		if task.Project != "demo" {
			t.Errorf("later task must carry project, got %+v", task)
		}
	}
}

func TestParseInline(t *testing.T) {
	tg := parseInline("README を英語化 @today @due(2026-06-14) @ms(v1-public) @p(1500)")
	if tg.Display != "README を英語化" {
		t.Errorf("display should strip tags: %q", tg.Display)
	}
	if !tg.Today {
		t.Error("@today not detected")
	}
	if tg.Due != "2026-06-14" {
		t.Errorf("due: %q", tg.Due)
	}
	if tg.Milestone != "v1-public" {
		t.Errorf("ms: %q", tg.Milestone)
	}
	if tg.Priority != 1500 {
		t.Errorf("priority: %d", tg.Priority)
	}

	// タグ無しはそのまま
	p := parseInline("ただのタスク")
	if p.Display != "ただのタスク" || p.Today || p.Due != "" || p.Milestone != "" || p.Priority != 0 {
		t.Errorf("plain task misparsed: %+v", p)
	}
}

// @due が今日以前なら today 扱い、未来なら stock 扱い
func TestCollectDueSurfacing(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "projects", "demo", "README.md"), `---
type: project
---
## タスク

- [ ] 期限切れ @due(2026-01-01)
- [ ] まだ先 @due(2099-12-31)
`)
	b, _ := collect(root, "2026-06-14")
	if len(b.Today) != 1 || b.Today[0].Display != "期限切れ" {
		t.Errorf("due<=today should surface in today: %+v", b.Today)
	}
	if len(b.Later) != 1 || b.Later[0].Display != "まだ先" {
		t.Errorf("future due should stay in stock: %+v", b.Later)
	}
}

// 時間軸バンド: @week / @month の手動タグと @due による自動昇格の振り分け。
// today=2026-07-25(土)。今週末(日)=2026-07-26、月末=2026-07-31。
func TestCollectBands(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "projects", "demo", "README.md"), `---
type: project
---
## タスク

- [ ] 今週やる @week
- [ ] 今月やる @month
- [ ] 中期のタスク
- [ ] 締切が今週日曜 @due(2026-07-26)
- [ ] 締切が月内 @due(2026-07-31)
- [ ] 締切ずっと先だが今週やる @week @due(2099-01-01)
- [x] 完了は出さない @week
`)
	b, _ := collect(root, "2026-07-25")
	disp := func(ts []Task) []string {
		out := []string{}
		for _, t := range ts {
			out = append(out, t.Display)
		}
		return out
	}
	// @week + @due≤今週末 + (@week@due先→week が勝つ)
	if got := disp(b.Week); len(got) != 3 {
		t.Errorf("week: want 3, got %v", got)
	}
	// @month + @due≤月末（今週末より後）
	if got := disp(b.Month); len(got) != 2 {
		t.Errorf("month: want 2, got %v", got)
	}
	// バンドタグ無し・締切無し
	if got := disp(b.Later); len(got) != 1 || got[0] != "中期のタスク" {
		t.Errorf("later: want [中期のタスク], got %v", got)
	}
	if len(b.Today) != 0 {
		t.Errorf("today: want 0, got %v", disp(b.Today))
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

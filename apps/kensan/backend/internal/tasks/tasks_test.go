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
	// ストック = project の ## タスク のうち未完了かつ today でないもの
	// （原稿レビュー依頼 のみ。アブストラクト確定=done / 没ネタ調査=skipped は除外、_archive も除外）
	if len(b.Stock) != 1 {
		t.Errorf("stock: want 1 (未完了の ## タスク のみ), got %d: %+v", len(b.Stock), b.Stock)
	}
	if len(b.Milestones) != 2 || len(b.Someday) != 1 {
		t.Errorf("milestones/someday: got %d/%d", len(b.Milestones), len(b.Someday))
	}
	for _, task := range b.Stock {
		if task.Project != "demo" {
			t.Errorf("stock task must carry project, got %+v", task)
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
	if len(b.Stock) != 1 || b.Stock[0].Display != "まだ先" {
		t.Errorf("future due should stay in stock: %+v", b.Stock)
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

package projects

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yu-min3/kensan-lab/apps/kensan/backend/internal/workspace"
)

const sample = `---
type: project
status: active
deadline: 2026-03-31
repo: "https://github.com/yu-min3/kensan-lab"
---

## 概要

ホームラボ。

## 目標

ホームラボを公開する

## マイルストーン

- [x] v1 公開
- [ ] Zenn 告知

## タスク

- [ ] README英語化 @today
- [x] 完了済み
- [ ] 図を描く

## ログ

- 2026-06-10: やったこと B
- 2026-06-13: やったこと A

## 関連ノート・リソース

### 設計

- [[notes/foo|フー]] — 説明テキスト
- https://example.com
`

func write(t *testing.T, root, name, content string) {
	t.Helper()
	p := filepath.Join(root, "projects", name, "README.md")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSummaries(t *testing.T) {
	root := t.TempDir()
	write(t, root, "kensan-lab", sample)
	write(t, root, "no-deadline", "---\ntype: project\nstatus: active\n---\n## 目標\n後回し\n")

	ss := Summaries(root)
	if len(ss) != 2 {
		t.Fatalf("want 2, got %d", len(ss))
	}
	// 締切ありが先
	if ss[0].Name != "kensan-lab" {
		t.Errorf("deadline project should sort first: %+v", ss)
	}
	k := ss[0]
	if k.Status != "active" || k.Deadline != "2026-03-31" || k.Goal != "ホームラボを公開する" {
		t.Errorf("summary fields: %+v", k)
	}
	if k.MilestonesDone != 1 || k.MilestonesTotal != 2 || k.OpenTasks != 2 {
		t.Errorf("progress wrong: %+v", k)
	}
}

func TestLoad(t *testing.T) {
	root := t.TempDir()
	write(t, root, "kensan-lab", sample)

	d, err := Load(root, "kensan-lab")
	if err != nil {
		t.Fatal(err)
	}
	if d.Repo == "" || d.Goal != "ホームラボを公開する" || d.Overview != "ホームラボ。" {
		t.Errorf("meta/goal/overview: %+v", d)
	}
	if len(d.Milestones) != 2 || len(d.Tasks) != 3 {
		t.Errorf("milestones/tasks: %d/%d", len(d.Milestones), len(d.Tasks))
	}
	// ログは日付降順、エントリ分割
	if len(d.Log) != 2 || d.Log[0].Date != "2026-06-13" || d.Log[0].Text != "やったこと A" {
		t.Errorf("log entries: %+v", d.Log)
	}
	// ノートは wikilink パース + group + desc
	if len(d.Notes) != 2 {
		t.Fatalf("notes count: %+v", d.Notes)
	}
	if d.Notes[0].Target != "notes/foo.md" || d.Notes[0].Label != "フー" || d.Notes[0].Desc != "説明テキスト" || d.Notes[0].Group != "設計" {
		t.Errorf("note ref: %+v", d.Notes[0])
	}
	if d.Notes[1].Label != "https://example.com" || d.Notes[1].Target != "" {
		t.Errorf("plain note: %+v", d.Notes[1])
	}
	// タスクの行内タグが効いている
	if d.Tasks[0].Display != "README英語化" || !d.Tasks[0].Today {
		t.Errorf("task tag parse: %+v", d.Tasks[0])
	}
}

func TestLoadMissing(t *testing.T) {
	if _, err := Load(t.TempDir(), "nope"); !os.IsNotExist(err) {
		t.Errorf("want not-exist error, got %v", err)
	}
}

func TestUpdate(t *testing.T) {
	root := t.TempDir()
	write(t, root, "kensan-lab", sample)
	ws := workspace.New(root)

	if err := Update(ws, "kensan-lab", "paused", "", "新しい目標"); err != nil {
		t.Fatal(err)
	}
	d, _ := Load(root, "kensan-lab")
	if d.Status != "paused" {
		t.Errorf("status not updated: %q", d.Status)
	}
	if d.Deadline != "" {
		t.Errorf("deadline should be removed: %q", d.Deadline)
	}
	if d.Goal != "新しい目標" {
		t.Errorf("goal not replaced: %q", d.Goal)
	}
	// 他セクションは残っている
	if len(d.Milestones) != 2 {
		t.Errorf("milestones lost: %+v", d.Milestones)
	}
}

func TestCreate(t *testing.T) {
	root := t.TempDir()
	ws := workspace.New(root)
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.Local)

	if err := Create(ws, "new-proj", now); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(root, "projects", "new-proj", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(content)
	if !strings.Contains(s, "status: active") || !strings.Contains(s, "created: 2026-06-15") ||
		!strings.Contains(s, "## マイルストーン") {
		t.Errorf("template wrong:\n%s", s)
	}
	// 二重作成は失敗
	if err := Create(ws, "new-proj", now); err == nil {
		t.Error("duplicate create should fail")
	}
	// 不正名
	if err := Create(ws, "Bad Name", now); err == nil {
		t.Error("invalid name should fail")
	}
}

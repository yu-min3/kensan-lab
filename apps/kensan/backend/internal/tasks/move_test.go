package tasks

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yu-min3/kensan-lab/apps/kensan/backend/internal/workspace"
)

func setupBoard(t *testing.T) (*workspace.Workspace, string) {
	t.Helper()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "todo.md"), sampleTodo)
	mustWrite(t, filepath.Join(root, "projects", "demo", "README.md"), sampleReadme)
	return workspace.New(root), root
}

// ストック → 今日（@today 付与）→ 完了 → daily の一巡
func TestTodayToggleAndArchive(t *testing.T) {
	ws, root := setupBoard(t)

	board, _ := Collect(root)
	// 「原稿レビュー依頼」は demo の ## タスク にある未完了 = ストック
	var src Task
	for _, task := range board.Stock {
		if task.Display == "原稿レビュー依頼" {
			src = task
		}
	}
	if src.Text == "" {
		t.Fatalf("fixture stock task not found: %+v", board.Stock)
	}

	// ストック → 今日（@today タグを付与。ファイルは project のまま）
	moved, err := SetToday(ws, src.File, src.Line, src.Text, true)
	if err != nil {
		t.Fatal(err)
	}
	if !moved.Today || moved.File != src.File {
		t.Fatalf("want @today on same file, got %+v", moved)
	}
	board, _ = Collect(root)
	inToday := false
	for _, task := range board.Today {
		if task.Display == "原稿レビュー依頼" && task.Project == "demo" {
			inToday = true
		}
	}
	if !inToday {
		t.Errorf("task should surface in today: %+v", board.Today)
	}

	// 完了にして daily へ退避
	done, err := SetState(ws, moved.File, moved.Line, moved.Text, "done")
	if err != nil {
		t.Fatal(err)
	}
	date := time.Date(2026, 6, 6, 0, 0, 0, 0, time.Local)
	final, err := Move(ws, done.File, done.Line, done.Text, Dest{Kind: "daily", Date: date})
	if err != nil {
		t.Fatal(err)
	}
	if final.File != "daily/2026/06/06.md" {
		t.Fatalf("unexpected daily path: %+v", final)
	}
	content, _ := os.ReadFile(filepath.Join(root, "daily", "2026", "06", "06.md"))
	if !strings.Contains(string(content), "### 完了タスク") || !strings.Contains(string(content), "原稿レビュー依頼") {
		t.Errorf("daily missing archived task:\n%s", content)
	}
}

// @today の付け外しが行内タグの付与/除去になっている
func TestSetTodayToggle(t *testing.T) {
	ws, root := setupBoard(t)
	board, _ := Collect(root)
	src := board.Stock[0]

	on, err := SetToday(ws, src.File, src.Line, src.Text, true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(on.Text, "@today") {
		t.Errorf("@today not appended: %q", on.Text)
	}
	off, err := SetToday(ws, on.File, on.Line, on.Text, false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(off.Text, "@today") || off.Today {
		t.Errorf("@today not removed: %q", off.Text)
	}
	if off.Display != src.Display {
		t.Errorf("display changed across toggle: %q vs %q", off.Display, src.Display)
	}
}

// 行内容の不一致（他クライアントが先に編集）は ErrLineMismatch で拒否し、ファイルを変更しない
func TestMoveLineMismatch(t *testing.T) {
	ws, root := setupBoard(t)
	before, _ := os.ReadFile(filepath.Join(root, "todo.md"))

	_, err := Move(ws, "todo.md", 5, "存在しないテキスト", Dest{Kind: "daily"})
	if !errors.Is(err, ErrLineMismatch) {
		t.Fatalf("want ErrLineMismatch, got %v", err)
	}
	after, _ := os.ReadFile(filepath.Join(root, "todo.md"))
	if string(before) != string(after) {
		t.Error("source file modified despite mismatch")
	}
}

// @p(N) の設定とストックの優先度ソート
func TestSetPriorityAndSort(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "projects", "demo", "README.md"), `---
type: project
---
## タスク

- [ ] あ
- [ ] い
- [ ] う
`)
	ws := workspace.New(root)
	board, _ := Collect(root)
	if len(board.Stock) != 3 {
		t.Fatalf("want 3 stock, got %d", len(board.Stock))
	}

	// 「う」に @p(10)、「あ」に @p(20) を付ける → う, あ, (い 未設定) の順
	var a, u Task
	for _, t2 := range board.Stock {
		switch t2.Display {
		case "あ":
			a = t2
		case "う":
			u = t2
		}
	}
	if _, err := SetPriority(ws, u.File, u.Line, u.Text, 10); err != nil {
		t.Fatal(err)
	}
	if _, err := SetPriority(ws, a.File, a.Line, a.Text, 20); err != nil {
		t.Fatal(err)
	}
	board, _ = Collect(root)
	order := []string{board.Stock[0].Display, board.Stock[1].Display, board.Stock[2].Display}
	if order[0] != "う" || order[1] != "あ" || order[2] != "い" {
		t.Errorf("priority sort wrong: %v", order)
	}
	if board.Stock[0].Priority != 10 {
		t.Errorf("priority not parsed: %+v", board.Stock[0])
	}

	// @p を 0 で除去すると未設定に戻る
	off, err := SetPriority(ws, board.Stock[0].File, board.Stock[0].Line, board.Stock[0].Text, 0)
	if err != nil {
		t.Fatal(err)
	}
	if off.Priority != 0 || strings.Contains(off.Text, "@p(") {
		t.Errorf("@p not removed: %+v", off)
	}
}

// 作成: project の ## タスク に追加（タグ付き）
func TestCreateTask(t *testing.T) {
	ws, root := setupBoard(t)
	out, err := CreateTask(ws, "demo", "新規タスク", true, "2026-06-20", "v1")
	if err != nil {
		t.Fatal(err)
	}
	if out.File != "projects/demo/README.md" || !out.Today || out.Due != "2026-06-20" || out.Milestone != "v1" {
		t.Fatalf("unexpected: %+v", out)
	}
	content, _ := os.ReadFile(filepath.Join(root, "projects", "demo", "README.md"))
	if !strings.Contains(string(content), "- [ ] 新規タスク @today @due(2026-06-20) @ms(v1)") {
		t.Errorf("line not created:\n%s", content)
	}
	// project 空 → todo.md ## Now
	out2, err := CreateTask(ws, "", "即席タスク", false, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if out2.File != "todo.md" {
		t.Fatalf("empty project should go to todo.md: %+v", out2)
	}
}

// 編集でプロジェクトを変えるとファイル間移動になり、@p は引き継がれる
func TestEditTaskReproject(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "projects", "a", "README.md"), "---\ntype: project\n---\n## タスク\n\n- [ ] 移動するタスク @p(1500)\n")
	mustWrite(t, filepath.Join(root, "projects", "b", "README.md"), "---\ntype: project\n---\n## タスク\n\n- [ ] 既存\n")
	ws := workspace.New(root)
	board, _ := Collect(root)
	var src Task
	for _, x := range board.Stock {
		if x.Display == "移動するタスク" {
			src = x
		}
	}
	out, err := EditTask(ws, src.File, src.Line, src.Text, "b", "移動するタスク", false, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if out.File != "projects/b/README.md" || out.Priority != 1500 {
		t.Fatalf("reproject/keep-priority failed: %+v", out)
	}
	a, _ := os.ReadFile(filepath.Join(root, "projects", "a", "README.md"))
	if strings.Contains(string(a), "移動するタスク") {
		t.Errorf("not removed from source:\n%s", a)
	}
	bb, _ := os.ReadFile(filepath.Join(root, "projects", "b", "README.md"))
	if !strings.Contains(string(bb), "- [ ] 移動するタスク @p(1500)") {
		t.Errorf("not inserted into dest:\n%s", bb)
	}
}

// インライン編集は本文だけ差し替え、行内タグを維持する
func TestSetText(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "projects", "demo", "README.md"), `---
type: project
---
## タスク

- [ ] 元のタスク @today @due(2026-06-20) @p(1000)
`)
	ws := workspace.New(root)
	board, _ := Collect(root)
	src := board.Today[0]
	if src.Display != "元のタスク" {
		t.Fatalf("unexpected fixture: %+v", src)
	}

	out, err := SetText(ws, src.File, src.Line, src.Text, "新しい本文")
	if err != nil {
		t.Fatal(err)
	}
	if out.Display != "新しい本文" || !out.Today || out.Due != "2026-06-20" || out.Priority != 1000 {
		t.Errorf("tags not preserved across edit: %+v", out)
	}
	content, _ := os.ReadFile(filepath.Join(root, src.File))
	if !strings.Contains(string(content), "- [ ] 新しい本文 @today @due(2026-06-20) @p(1000)") {
		t.Errorf("line not rewritten as expected:\n%s", content)
	}
}

func TestDeleteLine(t *testing.T) {
	ws, root := setupBoard(t)
	board, _ := Collect(root)
	src := board.Stock[0]

	if err := DeleteLine(ws, src.File, src.Line, src.Text); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(filepath.Join(root, src.File))
	if strings.Contains(string(content), src.Text) {
		t.Errorf("task line not removed:\n%s", content)
	}
	// 不一致は ErrLineMismatch
	if err := DeleteLine(ws, src.File, src.Line, "存在しない"); !errors.Is(err, ErrLineMismatch) {
		t.Errorf("want ErrLineMismatch, got %v", err)
	}
}

func TestSetStateMarks(t *testing.T) {
	ws, root := setupBoard(t)
	board, _ := Collect(root)
	src := board.Today[0]

	for _, state := range []string{"done", "skipped", "todo"} {
		got, err := SetState(ws, src.File, src.Line, src.Text, state)
		if err != nil {
			t.Fatal(err)
		}
		if got.State != state {
			t.Errorf("want %s, got %+v", state, got)
		}
	}
	if _, err := SetState(ws, src.File, src.Line, src.Text, "bogus"); err == nil {
		t.Error("unknown state should fail")
	}
	board, _ = Collect(root)
	if len(board.Today) != 2 {
		t.Errorf("today count changed by state ops: %d", len(board.Today))
	}
}

// CLAUDE.md の /reflection 日付判定: 0:00〜6:00 は前日扱い
func TestReflectionDate(t *testing.T) {
	cases := []struct {
		now  string
		want string
	}{
		{"2026-06-07T02:30:00", "2026-06-06"}, // 深夜 = 前日
		{"2026-06-07T05:59:59", "2026-06-06"},
		{"2026-06-07T06:00:00", "2026-06-07"}, // 6時以降 = 当日
		{"2026-06-07T23:00:00", "2026-06-07"},
	}
	for _, c := range cases {
		now, err := time.ParseInLocation("2006-01-02T15:04:05", c.now, time.Local)
		if err != nil {
			t.Fatal(err)
		}
		if got := ReflectionDate(now).Format("2006-01-02"); got != c.want {
			t.Errorf("ReflectionDate(%s) = %s, want %s", c.now, got, c.want)
		}
	}
}

// 深夜（0〜6時）に date 未指定で daily へ移動するとき、
// 移動先パスと新規作成される骨組みの日付が必ず一致する（codex review の指摘）
func TestNormalizedDestSkeletonConsistency(t *testing.T) {
	night := time.Date(2026, 6, 7, 2, 30, 0, 0, time.Local)
	d := Dest{Kind: "daily"}.normalized(night)

	file, _, err := d.resolve()
	if err != nil {
		t.Fatal(err)
	}
	if file != "daily/2026/06/06.md" {
		t.Fatalf("path should be previous day, got %s", file)
	}
	skeleton := newDailySkeleton(d.Date)
	if !strings.Contains(skeleton, "# 2026-06-06") || !strings.Contains(skeleton, "created: 2026-06-06") {
		t.Errorf("skeleton date mismatches path:\n%s", skeleton)
	}

	// 明示指定はそのまま尊重される
	explicit := Dest{Kind: "daily", Date: time.Date(2026, 6, 1, 0, 0, 0, 0, time.Local)}.normalized(night)
	if f, _, _ := explicit.resolve(); f != "daily/2026/06/01.md" {
		t.Errorf("explicit date overridden: %s", f)
	}
}

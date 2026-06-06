package tasks

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yu-min3/kensan-lab/apps/kensan-next/backend/internal/workspace"
)

func setupBoard(t *testing.T) (*workspace.Workspace, string) {
	t.Helper()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "todo.md"), sampleTodo)
	mustWrite(t, filepath.Join(root, "projects", "demo", "README.md"), sampleReadme)
	return workspace.New(root), root
}

// ストック → 今日 → daily の一巡（/morning と /reflection の操作に相当）
func TestMoveRoundTrip(t *testing.T) {
	ws, root := setupBoard(t)

	board, _ := Collect(root)
	// 「アブストラクト確定」は README にのみ存在するテキスト（todo.md との重複なし）
	var src Task
	for _, task := range board.Stock {
		if task.Text == "アブストラクト確定" {
			src = task
		}
	}
	if src.Text == "" {
		t.Fatal("fixture task not found")
	}

	// ストック → 今日
	moved, err := Move(ws, src.File, src.Line, src.Text, Dest{Kind: "today"})
	if err != nil {
		t.Fatal(err)
	}
	if moved.File != "todo.md" {
		t.Fatalf("want todo.md, got %+v", moved)
	}
	board, _ = Collect(root)
	if len(board.Stock) != 2 {
		t.Errorf("stock should shrink to 2, got %d", len(board.Stock))
	}
	if len(board.Today) != 3 {
		t.Errorf("today should grow to 3, got %d: %+v", len(board.Today), board.Today)
	}

	// 完了にして daily へ
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
	// daily は無かったので骨組みごと作られている
	content, err := os.ReadFile(filepath.Join(root, "daily", "2026", "06", "06.md"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(content)
	if !strings.Contains(s, "type: daily") || !strings.Contains(s, "### 完了タスク") ||
		!strings.Contains(s, "- [x] アブストラクト確定") {
		t.Errorf("daily skeleton missing parts:\n%s", s)
	}

	// 元の todo.md からは消えている
	todoContent, _ := os.ReadFile(filepath.Join(root, "todo.md"))
	if strings.Contains(string(todoContent), "アブストラクト確定") {
		t.Error("task not removed from todo.md")
	}
}

// 行内容の不一致（他クライアントが先に編集）は ErrLineMismatch で拒否し、ファイルを変更しない
func TestMoveLineMismatch(t *testing.T) {
	ws, root := setupBoard(t)
	before, _ := os.ReadFile(filepath.Join(root, "todo.md"))

	_, err := Move(ws, "todo.md", 5, "存在しないテキスト", Dest{Kind: "stock", Project: "demo"})
	if !errors.Is(err, ErrLineMismatch) {
		t.Fatalf("want ErrLineMismatch, got %v", err)
	}
	after, _ := os.ReadFile(filepath.Join(root, "todo.md"))
	if string(before) != string(after) {
		t.Error("source file modified despite mismatch")
	}
}

// 今日 → ストック（project 必須）
func TestMoveTodayToStock(t *testing.T) {
	ws, root := setupBoard(t)
	board, _ := Collect(root)
	src := board.Today[0]

	if _, err := Move(ws, src.File, src.Line, src.Text, Dest{Kind: "stock"}); err == nil {
		t.Fatal("stock without project should fail")
	}
	moved, err := Move(ws, src.File, src.Line, src.Text, Dest{Kind: "stock", Project: "demo"})
	if err != nil {
		t.Fatal(err)
	}
	if moved.Project != "demo" || moved.Section != "タスク" {
		t.Fatalf("unexpected move result: %+v", moved)
	}
	// ## タスク セクションの末尾に入っている（## ルーティン より前）
	content, _ := os.ReadFile(filepath.Join(root, "projects", "demo", "README.md"))
	s := string(content)
	if strings.Index(s, src.Text) > strings.Index(s, "## ルーティン") {
		t.Errorf("inserted outside ## タスク section:\n%s", s)
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

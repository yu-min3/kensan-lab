package tasks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 削除 → trash 退避 → 復元（元セクションへ）の一巡
func TestDeleteLineMovesToTrashAndRestore(t *testing.T) {
	ws, root := setupBoard(t)

	board, _ := Collect(root)
	var src Task
	for _, task := range board.Stock {
		if task.Display == "原稿レビュー依頼" {
			src = task
		}
	}
	if src.Text == "" {
		t.Fatalf("fixture stock task not found: %+v", board.Stock)
	}

	if err := DeleteLine(ws, src.File, src.Line, src.Text); err != nil {
		t.Fatal(err)
	}

	// 元ファイルから消えている
	content, _ := os.ReadFile(filepath.Join(root, src.File))
	if strings.Contains(string(content), "原稿レビュー依頼") {
		t.Errorf("task should be removed from source:\n%s", content)
	}

	// trash に @from / @deleted 付きで積まれている
	items, err := TrashList(ws)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("want 1 trash entry, got %d: %+v", len(items), items)
	}
	e := items[0]
	if e.Display != "原稿レビュー依頼" || e.From != src.File || e.Section != "タスク" || e.Deleted == "" {
		t.Fatalf("unexpected trash entry: %+v", e)
	}

	// 復元: 元ファイルの ## タスク に戻り、trash は空になる
	restored, err := RestoreFromTrash(ws, e.Line, e.Raw)
	if err != nil {
		t.Fatal(err)
	}
	if restored.File != src.File || restored.Display != "原稿レビュー依頼" || restored.Project != "demo" {
		t.Fatalf("unexpected restored task: %+v", restored)
	}
	content, _ = os.ReadFile(filepath.Join(root, src.File))
	if !strings.Contains(string(content), "- [ ] 原稿レビュー依頼") {
		t.Errorf("restored line missing in source:\n%s", content)
	}
	items, _ = TrashList(ws)
	if len(items) != 0 {
		t.Errorf("trash should be empty after restore: %+v", items)
	}
}

// 元ファイルが消えている場合は todo.md ## Now へフォールバック
func TestTrashRestoreFallbackToTodo(t *testing.T) {
	ws, root := setupBoard(t)

	board, _ := Collect(root)
	var src Task
	for _, task := range board.Stock {
		if task.Display == "原稿レビュー依頼" {
			src = task
		}
	}
	if err := DeleteLine(ws, src.File, src.Line, src.Text); err != nil {
		t.Fatal(err)
	}
	// 元プロジェクトごと削除（archive 相当）
	if err := os.RemoveAll(filepath.Join(root, "projects", "demo")); err != nil {
		t.Fatal(err)
	}

	items, _ := TrashList(ws)
	restored, err := RestoreFromTrash(ws, items[0].Line, items[0].Raw)
	if err != nil {
		t.Fatal(err)
	}
	if restored.File != "todo.md" || restored.Project != "" {
		t.Fatalf("want fallback to todo.md (no project), got %+v", restored)
	}
	content, _ := os.ReadFile(filepath.Join(root, "todo.md"))
	todoSection := string(content)[strings.Index(string(content), "## Now"):]
	if !strings.Contains(todoSection[:strings.Index(todoSection, "## メモ")], "原稿レビュー依頼") {
		t.Errorf("restored line should be under ## Now:\n%s", content)
	}
}

// 完全削除は復元不可・行内タグ（@due）は復元まで保持
func TestTrashPurgeAndTagPreserved(t *testing.T) {
	ws, root := setupBoard(t)

	// @due 付きタスクを作って消す
	created, err := CreateTask(ws, "demo", "期限付きタスク", false, "2026-12-31", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := DeleteLine(ws, created.File, created.Line, created.Text); err != nil {
		t.Fatal(err)
	}
	items, _ := TrashList(ws)
	if len(items) != 1 || items[0].Text != "期限付きタスク @due(2026-12-31)" {
		t.Fatalf("inline tag should be preserved in trash: %+v", items)
	}

	// 楽観ロック: text 不一致は ErrLineMismatch
	if err := PurgeTrashEntry(ws, items[0].Line, "別のテキスト"); err == nil {
		t.Fatal("purge with mismatched text should fail")
	}

	if err := PurgeTrashEntry(ws, items[0].Line, items[0].Raw); err != nil {
		t.Fatal(err)
	}
	items, _ = TrashList(ws)
	if len(items) != 0 {
		t.Errorf("trash should be empty after purge: %+v", items)
	}
	// 完全削除後、元ファイルにも戻っていない
	content, _ := os.ReadFile(filepath.Join(root, created.File))
	if strings.Contains(string(content), "期限付きタスク") {
		t.Errorf("purged task must not reappear:\n%s", content)
	}
}

// trash.md は Scan に現れない（ドットディレクトリ除外）
func TestTrashHiddenFromScan(t *testing.T) {
	ws, root := setupBoard(t)
	board, _ := Collect(root)
	if err := DeleteLine(ws, board.Stock[0].File, board.Stock[0].Line, board.Stock[0].Text); err != nil {
		t.Fatal(err)
	}
	docs, _ := ws.Scan()
	for _, d := range docs {
		if strings.HasPrefix(d.Path, ".kensan/") {
			t.Errorf("trash leaked into scan: %+v", d)
		}
	}
	_ = root
}

package today

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSetHabitKeepsDiaryAndOtherSections(t *testing.T) {
	src := "# 2026-10-10\n\n## 日記\n\n- 22:41 書いた\n\n## みのりちゃんへ\n\nおめでとう\n"
	line := habitLine("life", "英語 30分")
	added := string(setHabit(src, line, true))
	want := src + "\n## 習慣\n\n- [x] 英語 30分 @routine(life)\n"
	if added != want {
		t.Fatalf("add:\n%q\nwant:\n%q", added, want)
	}
	second := string(setHabit(added, habitLine("life", "ジム"), true))
	if second != want+"- [x] ジム @routine(life)\n" {
		t.Fatalf("second: %q", second)
	}
	if setHabit(second, line, true) != nil {
		t.Fatal("retry must not change the file")
	}
	removed := string(setHabit(string(setHabit(second, line, false)), habitLine("life", "ジム"), false))
	if removed != src {
		t.Fatalf("remove:\n%q\nwant:\n%q", removed, src)
	}
}

func TestSetHabitInsertsBeforeFollowingSection(t *testing.T) {
	src := "## 習慣\n\n- [x] A @routine(p)\n\n## 学習\n\nメモ\n"
	got := string(setHabit(src, habitLine("p", "B"), true))
	want := "## 習慣\n\n- [x] A @routine(p)\n- [x] B @routine(p)\n\n## 学習\n\nメモ\n"
	if got != want {
		t.Fatalf("got %q", got)
	}
}

func TestReadHistoryFromMarkdown(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("projects/life/README.md", "## タスク\n- [x] 終えた @done(2026-10-09)\n- [x] 日付なし\n")
	write("todo.md", "## Now\n- [x] 片付けた @done(2026-10-10)\n")
	write("daily/2026/10/10.md", "## 日記\n\n- [x] 日記の中の行\n\n### 完了タスク\n\n- [x] 移した @done(2026-10-08)\n\n## 習慣\n\n- [x] 英語 30分 @routine(life)\n- [x] タグなし\n")
	h, err := ReadHistory(root)
	if err != nil {
		t.Fatal(err)
	}
	if h.Done["2026-10-09"] != 1 || h.Done["2026-10-10"] != 1 || h.Done["2026-10-08"] != 1 || len(h.Done) != 3 {
		t.Fatalf("done: %+v", h.Done)
	}
	if !h.Routines[routineKey("life", "英語 30分")]["2026-10-10"] || len(h.Routines) != 1 {
		t.Fatalf("routines: %+v", h.Routines)
	}
}

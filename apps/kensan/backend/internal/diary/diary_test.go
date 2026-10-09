package diary

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yu-min3/kensan-lab/apps/kensan/backend/internal/workspace"
)

var jst = time.FixedZone("JST", 9*60*60)

func TestAppendLineKeepsOtherSections(t *testing.T) {
	src := "---\ntype: daily\n---\n\n# 2026-10-09\n\n## 日記\n\n朝は雨。\n\n## みのりちゃんへ\n\nおめでとう\n"
	got := AppendLine(src, time.Date(2026, 10, 9, 22, 41, 0, 0, jst), "ジムに行けた")
	want := "---\ntype: daily\n---\n\n# 2026-10-09\n\n## 日記\n\n朝は雨。\n- 22:41 ジムに行けた\n\n## みのりちゃんへ\n\nおめでとう\n"
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestAppendLineToSkeletonAndMissingHeading(t *testing.T) {
	at := time.Date(2026, 10, 9, 7, 5, 0, 0, jst)
	got := AppendLine(workspace.DailySkeleton(at), at, "早起き")
	if !strings.HasSuffix(got, "## 日記\n\n- 07:05 早起き\n") {
		t.Fatalf("skeleton: %q", got)
	}
	got = AppendLine("# 2026-10-09\n", at, "見出しなし")
	if got != "# 2026-10-09\n\n## 日記\n\n- 07:05 見出しなし\n" {
		t.Fatalf("missing heading: %q", got)
	}
}

func TestNormalize(t *testing.T) {
	if _, err := Normalize("  \n "); err != ErrEmpty {
		t.Fatalf("empty: %v", err)
	}
	if got, _ := Normalize("一行目\n二行目"); got != "一行目 二行目" {
		t.Fatalf("newline: %q", got)
	}
	if _, err := Normalize(strings.Repeat("あ", MaxLine+1)); err == nil {
		t.Fatal("too long must fail")
	}
}

func TestAddCreatesDailyAndRecentCountsOnlyEntries(t *testing.T) {
	root := t.TempDir()
	ws := workspace.New(root)
	at := time.Date(2026, 10, 9, 22, 0, 0, 0, jst)
	rel, err := Add(ws, at, at, "書いた")
	if err != nil || rel != "daily/2026/10/09.md" {
		t.Fatalf("add: %s %v", rel, err)
	}
	// 骨組みだけの日は記入に数えない。
	skeleton := filepath.Join(root, "daily/2026/10/08.md")
	if err := os.WriteFile(skeleton, []byte(workspace.DailySkeleton(at.AddDate(0, 0, -1))), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Recent(root, at, 3)
	if err != nil {
		t.Fatal(err)
	}
	if s.Last != "2026-10-09" || len(s.Days) != 3 || s.Days[1].Written || !s.Days[2].Written {
		t.Fatalf("recent: %+v", s)
	}
	content, _ := os.ReadFile(filepath.Join(root, rel))
	if !strings.Contains(string(content), "type: daily") || !strings.Contains(string(content), "- 22:00 書いた") {
		t.Fatalf("content: %s", content)
	}
}

func TestRecentWithoutDailyDir(t *testing.T) {
	s, err := Recent(t.TempDir(), time.Date(2026, 10, 9, 0, 0, 0, 0, jst), 2)
	if err != nil || s.Last != "" || len(s.Days) != 2 {
		t.Fatalf("%+v %v", s, err)
	}
}

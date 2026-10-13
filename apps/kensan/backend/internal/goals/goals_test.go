package goals

import "testing"

const sampleGoals = `---
type: goal
---

## North Star

「小さく作って早く出す」を続けるエンジニア

## 今期のフォーカス（2026 Q1-Q2）

1. **読書を習慣にする** — 月2冊読んで感想を書く
2. **自宅サーバーを整える** — バックアップを自動化する
3. 体力をつける — 週3回の運動

## 今月のゴール（6月）

- [ ] ダッシュボードには出さない月次ゴール
`

func TestParse(t *testing.T) {
	g := Parse(sampleGoals)

	// 行全体が引用符で囲まれていない場合はそのまま（「」は文中の一部）
	if g.NorthStar != "「小さく作って早く出す」を続けるエンジニア" {
		t.Errorf("northStar mismatch: %q", g.NorthStar)
	}

	if len(g.Focus) != 3 {
		t.Fatalf("focus: want 3 (月次は除外), got %d: %+v", len(g.Focus), g.Focus)
	}
	if g.Focus[0].Title != "読書を習慣にする" {
		t.Errorf("focus[0].Title (太字剥がし) mismatch: %q", g.Focus[0].Title)
	}
	if g.Focus[0].Detail != "月2冊読んで感想を書く" {
		t.Errorf("focus[0].Detail (em dash 区切り) mismatch: %q", g.Focus[0].Detail)
	}
	// 太字なし項目も title/detail に分かれる
	if g.Focus[2].Title != "体力をつける" || g.Focus[2].Detail != "週3回の運動" {
		t.Errorf("focus[2] mismatch: %+v", g.Focus[2])
	}
}

func TestParseEmpty(t *testing.T) {
	g := Parse("# nothing here\n")
	if g.NorthStar != "" || len(g.Focus) != 0 {
		t.Errorf("want empty goals, got %+v", g)
	}
}

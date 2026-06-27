package goals

import "testing"

const sampleGoals = `---
type: goal
---

## North Star

「インフラ × AI × 事業」を掛け合わせ、組織の技術的意思決定をリードできるエンジニア

## 今期のフォーカス（2026 Q1-Q2）

1. **対外発信の第一歩を踏み出す** — KubeCon CFP 提出、技術ブログ月1投稿
2. **ホームラボを公開する** — 技術的クレデンシャルの土台
3. 転職価値を上げる — 履歴書にかける業務成果

## 今月のゴール（6月）

- [ ] ダッシュボードには出さない月次ゴール
`

func TestParse(t *testing.T) {
	g := Parse(sampleGoals)

	// 行全体が引用符で囲まれていない場合はそのまま（「」は文中の一部）
	if g.NorthStar != "「インフラ × AI × 事業」を掛け合わせ、組織の技術的意思決定をリードできるエンジニア" {
		t.Errorf("northStar mismatch: %q", g.NorthStar)
	}

	if len(g.Focus) != 3 {
		t.Fatalf("focus: want 3 (月次は除外), got %d: %+v", len(g.Focus), g.Focus)
	}
	if g.Focus[0].Title != "対外発信の第一歩を踏み出す" {
		t.Errorf("focus[0].Title (太字剥がし) mismatch: %q", g.Focus[0].Title)
	}
	if g.Focus[0].Detail != "KubeCon CFP 提出、技術ブログ月1投稿" {
		t.Errorf("focus[0].Detail (em dash 区切り) mismatch: %q", g.Focus[0].Detail)
	}
	// 太字なし項目も title/detail に分かれる
	if g.Focus[2].Title != "転職価値を上げる" || g.Focus[2].Detail != "履歴書にかける業務成果" {
		t.Errorf("focus[2] mismatch: %+v", g.Focus[2])
	}
}

func TestParseEmpty(t *testing.T) {
	g := Parse("# nothing here\n")
	if g.NorthStar != "" || len(g.Focus) != 0 {
		t.Errorf("want empty goals, got %+v", g)
	}
}

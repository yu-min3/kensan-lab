package goals

import "testing"

const sampleGoals = `---
type: goal
---

## North Star

「インフラ × AI × 事業」を掛け合わせ、組織の技術的意思決定をリードできるエンジニア

## 今期のフォーカス（Q3: 2026-07〜09）

North Star に向けて、今期どのプロジェクトを優先するか。

- **projects/kubecon-2026** → 対外発信の第一歩。7/29-30 本番でいい経験を作り切る
- **projects/kensan-lab** → ホームラボの技術的クレデンシャルを固める
- 転職価値を上げる — 履歴書にかける業務成果

## 過去の月次ゴール（アーカイブ、〜6月）

- [ ] ダッシュボードには出さない月次ゴール
`

func TestParse(t *testing.T) {
	g := Parse(sampleGoals)

	// 行全体が引用符で囲まれていない場合はそのまま（「」は文中の一部）
	if g.NorthStar != "「インフラ × AI × 事業」を掛け合わせ、組織の技術的意思決定をリードできるエンジニア" {
		t.Errorf("northStar mismatch: %q", g.NorthStar)
	}

	if len(g.Focus) != 3 {
		t.Fatalf("focus: want 3 (アーカイブ節は除外), got %d: %+v", len(g.Focus), g.Focus)
	}
	if g.Focus[0].Title != "projects/kubecon-2026" {
		t.Errorf("focus[0].Title (太字剥がし) mismatch: %q", g.Focus[0].Title)
	}
	if g.Focus[0].Detail != "対外発信の第一歩。7/29-30 本番でいい経験を作り切る" {
		t.Errorf("focus[0].Detail (→ 区切り) mismatch: %q", g.Focus[0].Detail)
	}
	// 太字なし・em dash 区切りの旧フォーマット項目も title/detail に分かれる（寛容なパース）
	if g.Focus[2].Title != "転職価値を上げる" || g.Focus[2].Detail != "履歴書にかける業務成果" {
		t.Errorf("focus[2] mismatch: %+v", g.Focus[2])
	}
}

func TestParseNumberedLegacyFormat(t *testing.T) {
	// 旧フォーマット（番号付きリスト）も引き続きパースできる（寛容なパース原則）
	g := Parse("## 今期のフォーカス（2026 Q1-Q2）\n\n1. **対外発信の第一歩を踏み出す** — KubeCon CFP 提出\n2. ホームラボを公開する\n")
	if len(g.Focus) != 2 {
		t.Fatalf("focus: want 2, got %d: %+v", len(g.Focus), g.Focus)
	}
	if g.Focus[0].Title != "対外発信の第一歩を踏み出す" || g.Focus[0].Detail != "KubeCon CFP 提出" {
		t.Errorf("focus[0] mismatch: %+v", g.Focus[0])
	}
}

func TestParseEmpty(t *testing.T) {
	g := Parse("# nothing here\n")
	if g.NorthStar != "" || len(g.Focus) != 0 {
		t.Errorf("want empty goals, got %+v", g)
	}
}

package portfolio

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yu-min3/kensan-lab/apps/kensan/backend/internal/workspace"
)

var jst = time.FixedZone("JST", 9*60*60)

const portfolioFixture = `---
type: goal
---

説明の地の文。

## 事業に効かせる × インフラ

（空けておく）

## 事業に効かせる × AI

- 社内 経営向け AI エージェント — 判断材料づくり → 社内のみ

## 投資を決める × インフラ

- 2026-05-04 NAS を見送り Longhorn へ — 単一ノード依存 → 約 ¥53,000 を節約 @public @project(kensan-lab)

## 技術を選ぶ × インフラ

- 2026-07-30 KubeCon で基調講演と LT — 採択 → 英語で登壇 @public @project(kubecon-2026)

## 年報の章

- 2026-Q3 外へ出る
`

func write(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, File, portfolioFixture)
	write(t, root, "goals.md", "## North Star\n\nインフラ × AI × 事業\n\n## 配分\n\n- kensan-lab 30\n- outbound 25\n")
	write(t, root, "projects/kensan-lab/README.md", "---\ntype: project\nstatus: active\naims: [技術を選ぶ×AI]\n---\n## マイルストーン\n- [x] kind explore を出荷（試食導線） ✅ 2026-08-09\n- [ ] Longhorn Phase 2\n## タスク\n- [x] 片付けた @done(2026-10-08)\n- [x] 古い完了 @done(2026-08-01)\n## ログ\n- 2026-10-09: 何かした\n- 2026-10-02 (2): 別の日\n- 2026-07-01: 古い\n")
	write(t, root, "projects/outbound/README.md", "---\ntype: project\nstatus: active\naims: [事業に効かせる×AI]\n---\n## マイルストーン\n- [ ] AI ガバナンスの記事を公開 @due(2026-10-31)\n")
	write(t, root, "projects/_archive/kubecon-2026/README.md", "---\ntype: project\nstatus: completed\n---\n## マイルストーン\n- [x] KubeCon Japan 2026 で登壇 ✅ 2026-07-30\n")
	return root
}

func cell(v View, rung, domain string) Cell {
	for _, c := range v.Cells {
		if c.Rung == rung && c.Domain == domain {
			return c
		}
	}
	return Cell{}
}

func TestStatusComesFromPublicTags(t *testing.T) {
	v, err := Load(fixture(t), time.Date(2026, 10, 10, 12, 0, 0, 0, jst))
	if err != nil {
		t.Fatal(err)
	}
	if v.PublicCells != 2 || len(v.Cells) != 9 {
		t.Fatalf("public cells: %d / %d", v.PublicCells, len(v.Cells))
	}
	if c := cell(v, "impact", "インフラ"); c.Status != Empty || c.Note != "空けておく" {
		t.Fatalf("empty cell: %+v", c)
	}
	if c := cell(v, "impact", "AI"); c.Status != Internal || len(c.Aims) != 1 || c.Aims[0].Project != "outbound" || c.Aims[0].Next != "AI ガバナンスの記事を公開" {
		t.Fatalf("internal cell with aim: %+v", c)
	}
	e := cell(v, "invest", "インフラ").Evidence[0]
	if e.Date != "2026-05-04" || e.Title != "NAS を見送り Longhorn へ" || e.Story != "単一ノード依存" || e.Impact != "約 ¥53,000 を節約" || !e.Public || e.Project != "kensan-lab" {
		t.Fatalf("evidence: %+v", e)
	}
	if c := cell(v, "choose", "AI"); c.Status != Empty || c.Aims[0].Project != "kensan-lab" || c.Aims[0].Next != "Longhorn Phase 2" {
		t.Fatalf("aim on empty cell: %+v", c)
	}
}

func TestCompassCountsLast28Days(t *testing.T) {
	v, _ := Load(fixture(t), time.Date(2026, 10, 10, 12, 0, 0, 0, jst))
	if v.Compass.From != "2026-09-13" || v.Compass.Total != 3 {
		t.Fatalf("window: %+v", v.Compass)
	}
	var kl, ob Alloc
	for _, a := range v.Compass.Allocs {
		switch a.Project {
		case "kensan-lab":
			kl = a
		case "outbound":
			ob = a
		}
	}
	// 10/08 の @done・10/09 と 10/02 のログ。8/1 と 7/1 は窓の外
	if kl.Count != 3 || kl.Actual != 100 || kl.Target != 30 || kl.Weeks[3] != 2 || kl.Weeks[2] != 1 {
		t.Fatalf("kensan-lab: %+v", kl)
	}
	if ob.Count != 0 || ob.Target != 25 || ob.Actual != 0 {
		t.Fatalf("outbound: %+v", ob)
	}
	// kensan-lab も outbound も空き（非 public）のマスを担当している
	if v.GapShare != 100 {
		t.Fatalf("gap share: %d", v.GapShare)
	}
}

func TestAnnualPrefersMilestoneOnSameDay(t *testing.T) {
	v, _ := Load(fixture(t), time.Date(2026, 10, 10, 12, 0, 0, 0, jst))
	// マイルストーン 2（kind explore・KubeCon）+ 証拠の NAS。KubeCon の証拠は同日同プロジェクトで除く
	if v.Annual.Total != 3 || v.Annual.Month != 0 {
		t.Fatalf("annual: %+v", v.Annual)
	}
	if len(v.Annual.Chapters) != 2 || v.Annual.Chapters[1].Quarter != "2026-Q3" || v.Annual.Chapters[1].Name != "外へ出る" {
		t.Fatalf("chapters: %+v", v.Annual.Chapters)
	}
	if got := v.Annual.Chapters[1].Items[0].Title; got != "KubeCon Japan 2026 で登壇" {
		t.Fatalf("milestone title: %q", got)
	}
	if got := v.Annual.Chapters[1].Items[1].Title; got != "kind explore を出荷" {
		t.Fatalf("milestone title strip: %q", got)
	}
}

func TestMissingPortfolioStillRenders(t *testing.T) {
	root := t.TempDir()
	v, err := Load(root, time.Now())
	if err != nil || !v.Missing || len(v.Cells) != 9 || v.Compass.Allocs == nil || v.Proposals == nil {
		t.Fatalf("%+v %v", v, err)
	}
}

const inbox = `---
type: memo
---

## 提案

- [ ] 投資を決める × AI | 2026-10-10 AI 基盤の投資判断を説明 — 面談で比較 → 伝わった @project(job-search) @source(daily/2026/10/10.md)
- [ ] 事業に効かせる × インフラ | 2026-10-10 却下する候補 — x → y
- [ ] 存在しない × AI | 2026-10-10 壊れた行 — x → y
`

func TestAcceptAppendsOnceAndMarksInbox(t *testing.T) {
	root := fixture(t)
	write(t, root, "reflect/inbox/2026-10-11.md", inbox)
	ws := workspace.New(root)
	ps := Proposals(root)
	if len(ps) != 2 || ps[0].Rung != "invest" || ps[0].Source != "daily/2026/10/10.md" || ps[0].Body.Project != "job-search" {
		t.Fatalf("proposals: %+v", ps)
	}
	now := time.Date(2026, 10, 11, 8, 0, 0, 0, jst)
	for i := 0; i < 2; i++ {
		if _, err := Decide(ws, ps[0].File, ps[0].Line, ps[0].Text, true, now); err != nil {
			t.Fatalf("accept %d: %v", i, err)
		}
	}
	b, _ := os.ReadFile(filepath.Join(root, File))
	if n := strings.Count(string(b), "AI 基盤の投資判断を説明"); n != 1 {
		t.Fatalf("appended %d times:\n%s", n, b)
	}
	if !strings.Contains(string(b), "## 投資を決める × AI\n\n- 2026-10-10 AI 基盤の投資判断を説明 — 面談で比較 → 伝わった @project(job-search)\n") {
		t.Fatalf("new section:\n%s", b)
	}
	if strings.Contains(string(b), "@source") || strings.Contains(string(b), "AI 基盤の投資判断を説明 — 面談で比較 → 伝わった @public") {
		t.Fatal("source or public leaked into portfolio")
	}
	ib, _ := os.ReadFile(filepath.Join(root, "reflect/inbox/2026-10-11.md"))
	if !strings.Contains(string(ib), "- [x] 投資を決める × AI") {
		t.Fatalf("inbox not marked:\n%s", ib)
	}
	if _, err := Decide(ws, ps[0].File, ps[0].Line, ps[0].Text, false, now); !errors.Is(err, ErrMismatch) {
		t.Fatalf("reject after accept must conflict: %v", err)
	}
	if _, err := Decide(ws, ps[1].File, ps[1].Line, ps[1].Text, false, now); err != nil {
		t.Fatal(err)
	}
	b2, _ := os.ReadFile(filepath.Join(root, File))
	if strings.Contains(string(b2), "却下する候補") {
		t.Fatal("rejected proposal reached portfolio")
	}
	if left := Proposals(root); len(left) != 0 {
		t.Fatalf("decided proposals still listed: %+v", left)
	}
}

func TestAcceptIntoCellWithNote(t *testing.T) {
	root := fixture(t)
	write(t, root, "reflect/inbox/2026-10-11.md", "## 提案\n\n- [ ] 事業に効かせる × インフラ | 2026-10-10 初めての証拠 — x → y\n")
	ws := workspace.New(root)
	p := Proposals(root)[0]
	if _, err := Decide(ws, p.File, p.Line, p.Text, true, time.Now()); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(root, File))
	if !strings.Contains(string(b), "（空けておく）\n- 2026-10-10 初めての証拠 — x → y\n\n## 事業に効かせる × AI") {
		t.Fatalf("insert position:\n%s", b)
	}
	v, _ := Load(root, time.Now())
	if c := cell(v, "impact", "インフラ"); c.Status != Internal || len(c.Evidence) != 1 {
		t.Fatalf("cell after accept: %+v", c)
	}
}

func TestDecideRejectsStaleAndForeignFiles(t *testing.T) {
	root := fixture(t)
	write(t, root, "reflect/inbox/2026-10-11.md", inbox)
	ws := workspace.New(root)
	p := Proposals(root)[0]
	if _, err := Decide(ws, p.File, p.Line, p.Text+" 改変", true, time.Now()); !errors.Is(err, ErrMismatch) {
		t.Fatalf("stale text accepted: %v", err)
	}
	if _, err := Decide(ws, "portfolio.md", 1, "x", true, time.Now()); err == nil {
		t.Fatal("non-inbox file accepted")
	}
}

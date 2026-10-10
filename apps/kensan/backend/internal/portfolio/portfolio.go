// Package portfolio はダッシュボード（地図・羅針盤・年報）をファイルから組み立てる。
// AI は使わない。証拠は portfolio.md、担当は README の aims、狙う配分は goals.md の ## 配分、
// 時間の向きは @done と ## ログ の日付、年報は完了マイルストーンと証拠から数える。
// 契約: kensan-workspace projects/kensan-workspace/docs/portfolio-dashboard.md
package portfolio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/yu-min3/kensan-lab/apps/kensan/backend/internal/goals"
	"github.com/yu-min3/kensan-lab/apps/kensan/backend/internal/tasks"
)

const File = "portfolio.md"

// 段（判断の高さ）と領域。並びは画面の上から下・左から右。
var (
	Rungs   = []Rung{{"impact", "事業に効かせる", "いくら効いたか"}, {"invest", "投資を決める", "何に賭けるか"}, {"choose", "技術を選ぶ", "何で作るか"}}
	Domains = []string{"インフラ", "AI", "事業"}
)

type Rung struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Question string `json:"question"`
}

// Status はマスの判定。手で書かず、証拠の @public から決まる。
type Status string

const (
	Public   Status = "public"   // 外から見える証拠がある
	Internal Status = "internal" // 証拠はあるが社内だけ
	Empty    Status = "empty"    // 証拠なし
)

type Evidence struct {
	When    string `json:"when"` // YYYY-MM-DD か「社内」「取得済み」
	Date    string `json:"date,omitempty"`
	Title   string `json:"title"`
	Story   string `json:"story"`  // 状況と判断
	Impact  string `json:"impact"` // 影響
	Public  bool   `json:"public"`
	Project string `json:"project,omitempty"`
	Raw     string `json:"raw"`
}

type Aim struct {
	Project string `json:"project"`
	Next    string `json:"next,omitempty"` // 次の未完了マイルストーン
}

type Cell struct {
	Rung     string     `json:"rung"`
	Domain   string     `json:"domain"`
	Status   Status     `json:"status"`
	Note     string     `json:"note,omitempty"` // 証拠以外の地の文（例: 空けておく理由）
	Evidence []Evidence `json:"evidence"`
	Aims     []Aim      `json:"aims"`
}

type Alloc struct {
	Project string   `json:"project"`
	Target  int      `json:"target"` // 狙う割合（%）
	Actual  int      `json:"actual"` // 直近 28 日の割合（%）
	Count   int      `json:"count"`
	Weeks   []int    `json:"weeks"` // 古い順の 4 週の件数
	Aims    []string `json:"aims"`
}

type Compass struct {
	From   string  `json:"from"`
	To     string  `json:"to"`
	Total  int     `json:"total"`
	Allocs []Alloc `json:"allocs"`
}

type Milestone struct {
	Date    string `json:"date"`
	Title   string `json:"title"`
	Project string `json:"project,omitempty"`
	Kind    string `json:"kind"` // milestone | evidence
}

type Chapter struct {
	Quarter string      `json:"quarter"` // 2026-Q3
	Name    string      `json:"name"`
	Items   []Milestone `json:"items"`
}

type Annual struct {
	Year     int       `json:"year"`
	Total    int       `json:"total"`
	Month    int       `json:"month"`
	Chapters []Chapter `json:"chapters"`
}

type Proposal struct {
	File   string   `json:"file"`
	Line   int      `json:"line"`
	Text   string   `json:"text"` // チェックボックス以降の生テキスト（楽観ロック用）
	State  string   `json:"state"`
	Rung   string   `json:"rung"`
	Domain string   `json:"domain"`
	Body   Evidence `json:"evidence"`
	Source string   `json:"source,omitempty"`
}

// ReflectState は夜の証拠拾い（reflect/state.json）。書き手はジョブだけ。
type ReflectState struct {
	LastRunAt     string `json:"lastRunAt,omitempty"`
	LastSuccessAt string `json:"lastSuccessAt,omitempty"`
	Status        string `json:"status,omitempty"`
	Reason        string `json:"reason,omitempty"`
	Proposals     int    `json:"proposals,omitempty"`
	InputDate     string `json:"inputDate,omitempty"`
}

type View struct {
	Date        string        `json:"date"`
	NorthStar   string        `json:"northStar"`
	Rungs       []Rung        `json:"rungs"`
	Domains     []string      `json:"domains"`
	Cells       []Cell        `json:"cells"`
	PublicCells int           `json:"publicCells"`
	Compass     Compass       `json:"compass"`
	GapShare    int           `json:"gapShare"` // 空きを埋めに行く取り組みへ向いた時間の割合（%）
	Annual      Annual        `json:"annual"`
	Proposals   []Proposal    `json:"proposals"`
	Reflect     *ReflectState `json:"reflect"`
	Missing     bool          `json:"missing"` // portfolio.md が無い
}

var (
	headingRe    = regexp.MustCompile(`^##\s+(.+?)\s*$`)
	cellHeadRe   = regexp.MustCompile(`^(.+?)\s*[×x]\s*(.+)$`)
	evidenceRe   = regexp.MustCompile(`^-\s+(\S+)\s+(.+)$`)
	publicRe     = regexp.MustCompile(`\s*@public\b`)
	projectTagRe = regexp.MustCompile(`\s*@project\(([^)]+)\)`)
	sourceTagRe  = regexp.MustCompile(`\s*@source\(([^)]+)\)`)
	dateRe       = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	checkMarkRe  = regexp.MustCompile(`✅\s*(\d{4}-\d{2}-\d{2})`)
	logLineRe    = regexp.MustCompile(`^-\s+(\d{4}-\d{2}-\d{2})`)
	allocLineRe  = regexp.MustCompile(`^-\s+([a-z0-9][a-z0-9-]*)\s+(\d{1,3})\s*$`)
	chapterRe    = regexp.MustCompile(`^-\s+(\d{4}-Q[1-4])\s+(.+)$`)
	proposalRe   = regexp.MustCompile(`^\s*- \[([ x-])\] (.+?)\s*\|\s*(.+)$`)
)

func rungID(name string) string {
	for _, r := range Rungs {
		if r.Name == strings.TrimSpace(name) {
			return r.ID
		}
	}
	return ""
}

func domainOK(d string) bool {
	for _, x := range Domains {
		if x == strings.TrimSpace(d) {
			return true
		}
	}
	return false
}

// CellKey は「段×領域」の表記（aims や見出しと同じ言葉）。
func CellKey(rungName, domain string) string { return rungName + "×" + domain }

// ParseEvidence は 1 行を証拠に分解する。`- 日付 タイトル — 状況と判断 → 影響 @public @project(x)`
func ParseEvidence(line string) (Evidence, bool) {
	m := evidenceRe.FindStringSubmatch(strings.TrimSpace(line))
	if m == nil {
		return Evidence{}, false
	}
	return parseBody(m[1], m[2], line), true
}

func parseBody(when, rest, raw string) Evidence {
	e := Evidence{When: when, Raw: strings.TrimSpace(raw)}
	if dateRe.MatchString(when) {
		e.Date = when
	}
	e.Public = publicRe.MatchString(rest)
	if m := projectTagRe.FindStringSubmatch(rest); m != nil {
		e.Project = strings.TrimSpace(m[1])
	}
	rest = sourceTagRe.ReplaceAllString(projectTagRe.ReplaceAllString(publicRe.ReplaceAllString(rest, ""), ""), "")
	rest = strings.TrimSpace(rest)
	title, story, _ := strings.Cut(rest, " — ")
	e.Title = strings.TrimSpace(title)
	if story != "" {
		s, impact, _ := strings.Cut(story, " → ")
		e.Story, e.Impact = strings.TrimSpace(s), strings.TrimSpace(impact)
	}
	return e
}

// Parsed は portfolio.md の中身。
type Parsed struct {
	Cells    map[string]*Cell // key: rungID/domain
	Chapters map[string]string
}

func key(rung, domain string) string { return rung + "/" + domain }

func Parse(content string) Parsed {
	p := Parsed{Cells: map[string]*Cell{}, Chapters: map[string]string{}}
	for _, r := range Rungs {
		for _, d := range Domains {
			p.Cells[key(r.ID, d)] = &Cell{Rung: r.ID, Domain: d, Evidence: []Evidence{}, Aims: []Aim{}}
		}
	}
	var cur *Cell
	inChapters := false
	for _, line := range strings.Split(content, "\n") {
		if h := headingRe.FindStringSubmatch(line); h != nil {
			cur, inChapters = nil, h[1] == "年報の章"
			if m := cellHeadRe.FindStringSubmatch(h[1]); m != nil {
				if id := rungID(m[1]); id != "" && domainOK(m[2]) {
					cur = p.Cells[key(id, strings.TrimSpace(m[2]))]
				}
			}
			continue
		}
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		if inChapters {
			if m := chapterRe.FindStringSubmatch(t); m != nil {
				p.Chapters[m[1]] = strings.TrimSpace(m[2])
			}
			continue
		}
		if cur == nil {
			continue
		}
		if e, ok := ParseEvidence(t); ok {
			cur.Evidence = append(cur.Evidence, e)
		} else if cur.Note == "" {
			cur.Note = strings.Trim(t, "（）() ")
		}
	}
	for _, c := range p.Cells {
		c.Status = Empty
		for _, e := range c.Evidence {
			if e.Public {
				c.Status = Public
				break
			}
			c.Status = Internal
		}
	}
	return p
}

type projectInfo struct {
	name, file, status string
	aims               []string
	nextMilestone      string
	done               []string // @done の日付（タスク・マイルストーン）
	logs               []string // ## ログ の日付
	milestones         []Milestone
}

func readProject(root, rel, name string) (projectInfo, bool) {
	b, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		return projectInfo{}, false
	}
	content := string(b)
	pi := projectInfo{name: name, file: rel}
	fm := frontmatter(content)
	pi.status = fm["status"]
	if a := strings.Trim(fm["aims"], "[] "); a != "" {
		for _, x := range strings.Split(a, ",") {
			if x = strings.TrimSpace(strings.Trim(x, `"'`)); x != "" {
				pi.aims = append(pi.aims, strings.ReplaceAll(x, " ", ""))
			}
		}
	}
	for _, t := range tasks.ExtractLines(content, rel) {
		if t.State == "done" && t.Done != "" {
			pi.done = append(pi.done, t.Done)
		}
		if t.Section != "マイルストーン" {
			continue
		}
		if t.State == "todo" && pi.nextMilestone == "" {
			pi.nextMilestone = strings.ReplaceAll(t.Display, "**", "")
		}
		if t.State == "done" {
			date := t.Done
			if m := checkMarkRe.FindStringSubmatch(t.Text); m != nil {
				date = m[1]
			}
			if date != "" {
				title := checkMarkRe.ReplaceAllString(strings.ReplaceAll(t.Display, "**", ""), "")
				if i := strings.IndexAny(title, "（("); i > 0 {
					title = title[:i]
				}
				pi.milestones = append(pi.milestones, Milestone{Date: date, Title: strings.TrimSpace(title), Project: name, Kind: "milestone"})
			}
		}
	}
	inLog := false
	for _, line := range strings.Split(content, "\n") {
		if h := headingRe.FindStringSubmatch(line); h != nil {
			inLog = h[1] == "ログ"
			continue
		}
		if inLog {
			if m := logLineRe.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
				pi.logs = append(pi.logs, m[1])
			}
		}
	}
	return pi, true
}

func projects(root string) []projectInfo {
	var out []projectInfo
	for _, n := range tasks.Projects(root) {
		if pi, ok := readProject(root, "projects/"+n+"/README.md", n); ok {
			out = append(out, pi)
		}
	}
	if entries, err := os.ReadDir(filepath.Join(root, "projects", "_archive")); err == nil {
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
				if pi, ok := readProject(root, "projects/_archive/"+e.Name()+"/README.md", e.Name()); ok {
					pi.status = "archived"
					out = append(out, pi)
				}
			}
		}
	}
	return out
}

func frontmatter(content string) map[string]string {
	m := map[string]string{}
	if !strings.HasPrefix(content, "---") {
		return m
	}
	rest := content[3:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return m
	}
	for _, line := range strings.Split(rest[:end], "\n") {
		if i := strings.Index(line, ":"); i > 0 {
			m[strings.TrimSpace(line[:i])] = strings.Trim(strings.TrimSpace(line[i+1:]), `"`)
		}
	}
	return m
}

// targets は goals.md の ## 配分（- name 数字）。
func targets(root string) ([]string, map[string]int) {
	b, err := os.ReadFile(filepath.Join(root, "goals.md"))
	if err != nil {
		return nil, map[string]int{}
	}
	order, out := []string{}, map[string]int{}
	in := false
	for _, line := range strings.Split(string(b), "\n") {
		if h := headingRe.FindStringSubmatch(line); h != nil {
			in = h[1] == "配分"
			continue
		}
		if in {
			if m := allocLineRe.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
				n, _ := strconv.Atoi(m[2])
				order = append(order, m[1])
				out[m[1]] = n
			}
		}
	}
	return order, out
}

func pct(n, total int) int {
	if total == 0 {
		return 0
	}
	return (n*100 + total/2) / total
}

// Load は今日の日付（JST）でダッシュボードを組み立てる。
func Load(root string, now time.Time) (View, error) {
	jst := time.FixedZone("JST", 9*60*60)
	now = now.In(jst)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, jst)
	v := View{Date: today.Format("2006-01-02"), Rungs: Rungs, Domains: Domains, Cells: []Cell{}, Proposals: []Proposal{}}
	g, err := goals.Load(root)
	if err != nil {
		return v, err
	}
	v.NorthStar = g.NorthStar

	b, err := os.ReadFile(filepath.Join(root, File))
	if os.IsNotExist(err) {
		v.Missing = true
	} else if err != nil {
		return v, err
	}
	parsed := Parse(string(b))
	projs := projects(root)

	// 担当: aims を持つ active なプロジェクト
	for _, pi := range projs {
		if pi.status != "active" {
			continue
		}
		for _, a := range pi.aims {
			rn, dom, ok := strings.Cut(a, "×")
			if !ok {
				continue
			}
			if c := parsed.Cells[key(rungID(rn), strings.TrimSpace(dom))]; c != nil {
				c.Aims = append(c.Aims, Aim{Project: pi.name, Next: pi.nextMilestone})
			}
		}
	}
	for _, r := range Rungs {
		for _, d := range Domains {
			c := *parsed.Cells[key(r.ID, d)]
			if c.Status == Public {
				v.PublicCells++
			}
			v.Cells = append(v.Cells, c)
		}
	}

	// 羅針盤: 直近 28 日の @done と ## ログ の日付。古い順に 7 日ずつ 4 本
	from := today.AddDate(0, 0, -27)
	v.Compass.From, v.Compass.To = from.Format("2006-01-02"), v.Date
	order, tgt := targets(root)
	counts := map[string][]int{}
	for _, pi := range projs {
		if pi.status == "archived" {
			continue
		}
		weeks := make([]int, 4)
		for _, d := range append(append([]string{}, pi.done...), pi.logs...) {
			t, err := time.ParseInLocation("2006-01-02", d, jst)
			if err != nil || t.Before(from) || t.After(today) {
				continue
			}
			weeks[int(t.Sub(from).Hours()/24)/7]++
		}
		counts[pi.name] = weeks
	}
	names := append([]string{}, order...)
	for n, w := range counts {
		if _, ok := tgt[n]; !ok && sum(w) > 0 {
			names = append(names, n)
		}
	}
	gap := map[string]bool{}
	aimsOf := map[string][]string{}
	for _, pi := range projs {
		aimsOf[pi.name] = pi.aims
		for _, a := range pi.aims {
			rn, dom, _ := strings.Cut(a, "×")
			if c := parsed.Cells[key(rungID(rn), strings.TrimSpace(dom))]; c != nil && c.Status != Public {
				gap[pi.name] = true
			}
		}
	}
	gapCount := 0
	for _, n := range names {
		w := counts[n]
		if w == nil {
			w = make([]int, 4)
		}
		a := Alloc{Project: n, Target: tgt[n], Count: sum(w), Weeks: w, Aims: aimsOf[n]}
		if a.Aims == nil {
			a.Aims = []string{}
		}
		v.Compass.Total += a.Count
		if gap[n] {
			gapCount += a.Count
		}
		v.Compass.Allocs = append(v.Compass.Allocs, a)
	}
	for i := range v.Compass.Allocs {
		v.Compass.Allocs[i].Actual = pct(v.Compass.Allocs[i].Count, v.Compass.Total)
	}
	if v.Compass.Allocs == nil {
		v.Compass.Allocs = []Alloc{}
	}
	v.GapShare = pct(gapCount, v.Compass.Total)

	// 年報: 今年の完了マイルストーンと日付のある証拠。同じ日・同じプロジェクトはマイルストーンを優先
	year := today.Year()
	var items []Milestone
	seen := map[string]bool{}
	for _, pi := range projs {
		for _, m := range pi.milestones {
			if strings.HasPrefix(m.Date, strconv.Itoa(year)) {
				items = append(items, m)
				seen[m.Date+"/"+m.Project] = true
			}
		}
	}
	for _, c := range parsed.Cells {
		for _, e := range c.Evidence {
			if e.Date != "" && strings.HasPrefix(e.Date, strconv.Itoa(year)) && !seen[e.Date+"/"+e.Project] {
				items = append(items, Milestone{Date: e.Date, Title: e.Title, Project: e.Project, Kind: "evidence"})
			}
		}
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Date < items[j].Date })
	v.Annual = Annual{Year: year, Chapters: []Chapter{}}
	byQ := map[string]*Chapter{}
	month := today.Format("2006-01")
	for _, it := range items {
		v.Annual.Total++
		if strings.HasPrefix(it.Date, month) {
			v.Annual.Month++
		}
		mo, _ := strconv.Atoi(it.Date[5:7])
		q := it.Date[:4] + "-Q" + strconv.Itoa((mo-1)/3+1)
		ch := byQ[q]
		if ch == nil {
			v.Annual.Chapters = append(v.Annual.Chapters, Chapter{Quarter: q, Name: parsed.Chapters[q], Items: []Milestone{}})
			ch = &v.Annual.Chapters[len(v.Annual.Chapters)-1]
			byQ[q] = ch
		}
		ch.Items = append(ch.Items, it)
	}

	v.Proposals = Proposals(root)
	if b, err := os.ReadFile(filepath.Join(root, "reflect", "state.json")); err == nil {
		var st ReflectState
		if json.Unmarshal(b, &st) == nil {
			v.Reflect = &st
		}
	}
	return v, nil
}

func sum(w []int) int {
	n := 0
	for _, x := range w {
		n += x
	}
	return n
}

// Proposals は reflect/inbox/*.md の「## 提案」の行を新しい順に返す（未判定だけ）。
func Proposals(root string) []Proposal {
	out := []Proposal{}
	dir := filepath.Join(root, "reflect", "inbox")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return out
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() > entries[j].Name() })
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		rel := "reflect/inbox/" + e.Name()
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		for i, line := range strings.Split(string(b), "\n") {
			if p, ok := parseProposal(line, rel, i+1); ok && p.State == "todo" {
				out = append(out, p)
			}
		}
	}
	return out
}

func parseProposal(line, file string, n int) (Proposal, bool) {
	m := proposalRe.FindStringSubmatch(line)
	if m == nil {
		return Proposal{}, false
	}
	cm := cellHeadRe.FindStringSubmatch(strings.TrimSpace(m[2]))
	if cm == nil {
		return Proposal{}, false
	}
	rid, dom := rungID(cm[1]), strings.TrimSpace(cm[2])
	if rid == "" || !domainOK(dom) {
		return Proposal{}, false
	}
	body, ok := ParseEvidence("- " + strings.TrimSpace(m[3]))
	if !ok {
		return Proposal{}, false
	}
	state := map[string]string{" ": "todo", "x": "accepted", "-": "rejected"}[m[1]]
	p := Proposal{File: file, Line: n, Text: strings.TrimSpace(strings.SplitN(line, "] ", 2)[1]), State: state, Rung: rid, Domain: dom, Body: body}
	if s := sourceTagRe.FindStringSubmatch(m[3]); s != nil {
		p.Source = s[1]
	}
	return p, true
}

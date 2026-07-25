package projects

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/yu-min3/kensan-lab/apps/kensan/backend/internal/metrics"
	"github.com/yu-min3/kensan-lab/apps/kensan/backend/internal/tasks"
)

// State は project の状態判定。決定的なルールだけで決め、表示経路に LLM を入れない。
// 色だけで状態を表さないため、ラベルと必ず根拠（Why）を一緒に返す。
type State struct {
	Label    string `json:"label"`
	Tone     string `json:"tone"` // success | warn | muted
	Why      string `json:"why"`  // 一覧・Hero バッジ横の一行
	Sentence string `json:"sentence"`
	// LastActivity は最後に前進した日（ログ / 完了マイルストーン / メトリクス更新の最大）。
	LastActivity string `json:"lastActivity,omitempty"`
}

// MetricBrief は一覧に出す主指標の要約。詳細は /metrics が返す。
type MetricBrief struct {
	Label   string   `json:"label"`
	Unit    string   `json:"unit"`
	Current *float64 `json:"current,omitempty"`
	Target  *float64 `json:"target,omitempty"`
	Display string   `json:"display"`
}

var doneDateRe = regexp.MustCompile(`✅\s*(\d{4}-\d{2}-\d{2})`)

const dateFmt = "2006-01-02"

func daysFrom(today time.Time, iso string) (int, bool) {
	t, err := time.Parse(dateFmt, iso)
	if err != nil {
		return 0, false
	}
	return int(t.Sub(today).Hours() / 24), true
}

// lastActivity は README ログの日付・完了マイルストーンの ✅ 日付・メトリクス更新日の最大値。
func lastActivity(log []LogEntry, milestones []tasks.Task, views []metrics.View) string {
	latest := ""
	keep := func(d string) {
		if d != "" && d > latest {
			latest = d
		}
	}
	for _, e := range log {
		keep(e.Date)
	}
	for _, m := range milestones {
		if hit := doneDateRe.FindStringSubmatch(m.Text); hit != nil {
			keep(hit[1])
		}
	}
	for _, v := range views {
		if len(v.UpdatedAt) >= 10 {
			keep(v.UpdatedAt[:10])
		}
	}
	return latest
}

// ComputeState は状態を決める。判定順は「締切 → 滞留 → 前進 → 停滞 → 記録なし」。
// 先に来るものほど手当てが要る状態で、後ろは情報が足りないだけの状態。
func ComputeState(deadline string, milestones, tasksList []tasks.Task, log []LogEntry, views []metrics.View, today time.Time) State {
	openMs, overdue := 0, 0
	for _, m := range milestones {
		if m.State != "done" {
			openMs++
		}
	}
	for _, t := range tasksList {
		if t.State != "todo" || t.Due == "" {
			continue
		}
		if d, ok := daysFrom(today, t.Due); ok && d < 0 {
			overdue++
		}
	}

	if deadline != "" && openMs > 0 {
		if d, ok := daysFrom(today, deadline); ok && d >= 0 && d <= 14 {
			return State{
				Label: "仕上げの時期", Tone: "warn",
				Why:      fmt.Sprintf("締切まで %d 日・未完マイルストーン %d", d, openMs),
				Sentence: fmt.Sprintf("締切まで %d 日。未完のマイルストーンが %d 件残っています。", d, openMs),
			}
		}
	}
	if overdue > 0 {
		return State{
			Label: "要整理", Tone: "warn",
			Why:      fmt.Sprintf("期限切れタスク %d 件", overdue),
			Sentence: fmt.Sprintf("期限切れのタスクが %d 件。まず滞留を片付けると動きが戻ります。", overdue),
		}
	}

	activity := lastActivity(log, milestones, views)
	if activity == "" {
		return State{
			Label: "観測を開始", Tone: "muted",
			Why:      "ログもメトリクスもまだありません",
			Sentence: "ログもメトリクスもまだありません。まず記録を 1 つ残すところから。",
		}
	}
	since := 0
	if d, ok := daysFrom(today, activity); ok {
		since = -d
	}
	if since <= 30 {
		s := State{Label: "前進中", Tone: "success", Why: activity + " に前進", LastActivity: activity}
		if since == 0 {
			s.Sentence = "今日前進しました。"
		} else {
			s.Sentence = fmt.Sprintf("%d 日前に前進しています。", since)
		}
		return s
	}
	if len(milestones) == 0 {
		return State{
			Label: "安定運用", Tone: "muted",
			Why:          "直近の記録は " + activity,
			Sentence:     "終点を置かない project です。直近の記録は " + activity + "。",
			LastActivity: activity,
		}
	}
	return State{
		Label: "再開待ち", Tone: "muted",
		Why:          fmt.Sprintf("%d 日動きなし", since),
		Sentence:     fmt.Sprintf("%d 日動きがありません。小さく再開できる一手を選びます。", since),
		LastActivity: activity,
	}
}

// briefOf は主指標（先頭）を一覧用に要約する。
func briefOf(views []metrics.View) *MetricBrief {
	if len(views) == 0 {
		return nil
	}
	v := views[0]
	return &MetricBrief{Label: v.Label, Unit: v.Unit, Current: v.Current, Target: v.Target, Display: v.Display}
}

// FormatMetric は "4 / 100 stars" の形に整える（一覧のバッジ用）。
func (b MetricBrief) Format() string {
	if b.Current == nil {
		return "— " + b.Unit
	}
	f := func(v float64) string {
		if b.Display == "integer" {
			return fmt.Sprintf("%.0f", v)
		}
		return strings.TrimSuffix(fmt.Sprintf("%.1f", v), ".0")
	}
	if b.Target != nil {
		return fmt.Sprintf("%s / %s %s", f(*b.Current), f(*b.Target), b.Unit)
	}
	return fmt.Sprintf("%s %s", f(*b.Current), b.Unit)
}

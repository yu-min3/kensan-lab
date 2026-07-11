package api

import (
	"io/fs"
	"net/http"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// レビュー成果物（/weekly-review, /reflection が生成する HTML）の一覧と配信。
// ファイル契約: reviews/YYYY/WNN.html（週次）, reviews/daily/*.html（日次）,
// reviews/YYYY/MM-monthly.*（月次）。md も後方互換で列挙する。

type reviewEntry struct {
	Path  string    `json:"path"` // workspace 相対
	Name  string    `json:"name"`
	Kind  string    `json:"kind"` // weekly | daily | monthly | other
	MTime time.Time `json:"mtime"`
	Size  int64     `json:"size"`
}

var weeklyRe = regexp.MustCompile(`^W\d+`)

var (
	// 正規の reviews/daily/YYYY/MM/DD.html と、過渡期の flat な YYYY-MM-DD.html の両方を拾う
	dailyDateRe   = regexp.MustCompile(`(\d{4})[-/](\d{2})[-/](\d{2})\.[^.]+$`)
	weeklyDateRe  = regexp.MustCompile(`(\d{4})/W0*(\d{1,2})\.[^.]+$`)
	monthlyDateRe = regexp.MustCompile(`(\d{4})/0*(\d{1,2})-monthly\.[^.]+$`)
)

// reviewDate はレビューの「期間終端」をパス規約から導く。
// mtime は再生成・rsync で簡単に狂い、月次が日次の列に割り込む並びになるため使わない。
// daily = その日 / weekly = ISO 週の日曜 / monthly = 月末。導けないものは mtime に fallback。
func reviewDate(rel string, kind string, mtime time.Time) time.Time {
	switch kind {
	case "daily":
		if m := dailyDateRe.FindStringSubmatch(rel); m != nil {
			if t, err := time.Parse("2006-01-02", m[1]+"-"+m[2]+"-"+m[3]); err == nil {
				return t
			}
		}
	case "weekly":
		if m := weeklyDateRe.FindStringSubmatch(rel); m != nil {
			year := atoi(m[1])
			week := atoi(m[2])
			return isoWeekSunday(year, week)
		}
	case "monthly":
		if m := monthlyDateRe.FindStringSubmatch(rel); m != nil {
			year := atoi(m[1])
			month := atoi(m[2])
			// 翌月 1 日 - 1 日 = 月末
			return time.Date(year, time.Month(month)+1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, -1)
		}
	}
	return mtime
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		n = n*10 + int(c-'0')
	}
	return n
}

// isoWeekSunday は ISO 8601 週番号の週の日曜（期間終端）を返す。
func isoWeekSunday(year, week int) time.Time {
	// ISO 週 1 は 1/4 を含む週。その週の月曜を起点に week-1 週 + 6 日進める
	jan4 := time.Date(year, 1, 4, 0, 0, 0, 0, time.UTC)
	wd := int(jan4.Weekday())
	if wd == 0 {
		wd = 7 // Sunday を 7 に（ISO は月曜始まり）
	}
	week1Monday := jan4.AddDate(0, 0, -(wd - 1))
	return week1Monday.AddDate(0, 0, (week-1)*7+6)
}

// GET /api/v1/reviews
func (s *Server) handleReviews(w http.ResponseWriter, _ *http.Request) {
	root := filepath.Join(s.ws.Root, "reviews")
	var out []reviewEntry
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			// workspace.Scan と同じ意味論: 隠しディレクトリは見ない
			if path != root && strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return nil // symlink は辿らない（workspace.Scan と同じ）
		}
		if !strings.HasSuffix(name, ".html") && !strings.HasSuffix(name, ".md") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(s.ws.Root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		kind := "other"
		switch {
		case strings.Contains(rel, "/daily/"):
			kind = "daily"
		case weeklyRe.MatchString(name):
			kind = "weekly"
		case strings.Contains(name, "monthly"):
			kind = "monthly"
		}
		out = append(out, reviewEntry{Path: rel, Name: name, Kind: kind, MTime: info.ModTime(), Size: info.Size()})
		return nil
	})
	out = dedupeReviews(out)
	// 期間終端の新しい順（同日なら daily を上に）。mtime 順だと再生成した月次が
	// 今週の日次の間に割り込む（実データで 4 月月次が 7/7 と 7/6 の間に出た）
	sort.SliceStable(out, func(i, j int) bool {
		di := reviewDate(out[i].Path, out[i].Kind, out[i].MTime)
		dj := reviewDate(out[j].Path, out[j].Kind, out[j].MTime)
		if !di.Equal(dj) {
			return di.After(dj)
		}
		return out[i].Path < out[j].Path
	})
	writeJSON(w, http.StatusOK, map[string]any{"reviews": out, "total": len(out)})
}

// dedupeReviews は同じ期間のレビューが .md（過渡期の旧形式）と .html の両方で
// 存在するとき .html を採用する（W19 が 2 件並ぶ実データの解消）。
func dedupeReviews(in []reviewEntry) []reviewEntry {
	stem := func(e reviewEntry) string { return strings.TrimSuffix(e.Path, filepath.Ext(e.Path)) }
	htmlStems := make(map[string]bool, len(in))
	for _, e := range in {
		if strings.HasSuffix(e.Path, ".html") {
			htmlStems[stem(e)] = true
		}
	}
	out := in[:0]
	for _, e := range in {
		if strings.HasSuffix(e.Path, ".md") && htmlStems[stem(e)] {
			continue
		}
		out = append(out, e)
	}
	return out
}

// GET /api/v1/reviews/{path...} — レビューファイルを生のまま配信（iframe 用）。
// reviews/ サブツリー限定。Content-Type は拡張子から（.html → text/html）。
func (s *Server) handleReviewContent(w http.ResponseWriter, r *http.Request) {
	rel := "reviews/" + r.PathValue("path")
	abs, err := s.ws.Abs(rel)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}
	// Abs は workspace 外を拒否するが、reviews/ 外への .. 移動も明示的に塞ぐ
	cleanRel, err := filepath.Rel(s.ws.Root, filepath.Clean(abs))
	if err != nil || !strings.HasPrefix(filepath.ToSlash(cleanRel), "reviews/") {
		writeError(w, http.StatusBadRequest, "path must be under reviews/")
		return
	}
	// http.ServeFile は symlink を辿るため、解決後の実体も reviews/ 配下であることを検証する
	// （reviews/evil -> /etc のような symlink 経由の workspace 外配信を防ぐ）
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	realRoot, err := filepath.EvalSymlinks(filepath.Join(s.ws.Root, "reviews"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if resolved != realRoot && !strings.HasPrefix(resolved, realRoot+string(filepath.Separator)) {
		writeError(w, http.StatusBadRequest, "symlink escapes reviews/")
		return
	}
	http.ServeFile(w, r, abs)
}

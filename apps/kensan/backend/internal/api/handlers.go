package api

import (
	"fmt"
	"net/http"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/yu-min3/kensan-lab/apps/kensan/backend/internal/tasks"
	"github.com/yu-min3/kensan-lab/apps/kensan/backend/internal/workspace"
)

// GET /api/v1/files?type=&tag=&status=&q=
func (s *Server) handleFiles(w http.ResponseWriter, r *http.Request) {
	docs, err := s.ws.Scan()
	if err != nil {
		s.log.Warn("scan finished with errors", "err", err)
	}
	q := r.URL.Query()
	typ := q.Get("type")
	tag := q.Get("tag")
	statusFilter := q.Get("status")
	text := strings.ToLower(q.Get("q"))

	out := docs[:0:0]
	for _, d := range docs {
		if typ != "" && d.Meta.Type != typ {
			continue
		}
		if statusFilter != "" && d.Meta.Status != statusFilter {
			continue
		}
		if tag != "" && !slices.Contains(d.Meta.Tags, tag) {
			continue
		}
		if text != "" && !strings.Contains(strings.ToLower(d.Path), text) &&
			!strings.Contains(strings.ToLower(d.Meta.Title), text) {
			continue
		}
		out = append(out, d)
	}
	writeJSON(w, http.StatusOK, map[string]any{"files": out, "total": len(out)})
}

// GET /api/v1/files/{path...}
func (s *Server) handleFileDetail(w http.ResponseWriter, r *http.Request) {
	rel := r.PathValue("path")
	doc, content, err := s.ws.Read(rel)
	if err != nil {
		if os.IsNotExist(err) {
			writeError(w, http.StatusNotFound, "file not found: "+rel)
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"doc": doc, "content": string(content)})
}

// GET /api/v1/daily?date=YYYY-MM-DD（省略時は今日）/ ?limit=N（直近一覧）
func (s *Server) handleDaily(w http.ResponseWriter, r *http.Request) {
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit <= 0 {
			writeError(w, http.StatusBadRequest, "invalid limit")
			return
		}
		s.handleDailyList(w, limit)
		return
	}
	date := r.URL.Query().Get("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid date: "+date)
		return
	}
	rel := fmt.Sprintf("daily/%04d/%02d/%02d.md", t.Year(), t.Month(), t.Day())
	doc, content, err := s.ws.Read(rel)
	if err != nil {
		if os.IsNotExist(err) {
			writeError(w, http.StatusNotFound, "no daily for "+date)
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"doc": doc, "content": string(content)})
}

func (s *Server) handleDailyList(w http.ResponseWriter, limit int) {
	docs, _ := s.ws.Scan()
	var dailies []workspace.Doc
	for _, d := range docs {
		if strings.HasPrefix(d.Path, "daily/") {
			dailies = append(dailies, d)
		}
	}
	// daily/YYYY/MM/DD.md はパス順 = 日付順
	sort.Slice(dailies, func(i, j int) bool { return dailies[i].Path > dailies[j].Path })
	if len(dailies) > limit {
		dailies = dailies[:limit]
	}
	writeJSON(w, http.StatusOK, map[string]any{"files": dailies, "total": len(dailies)})
}

// GET /api/v1/tasks — かんばん Board（today / stock / someday / milestones）
func (s *Server) handleTasks(w http.ResponseWriter, _ *http.Request) {
	board, err := tasks.Collect(s.ws.Root)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, board)
}

// GET /api/v1/tags — タグ集計
func (s *Server) handleTags(w http.ResponseWriter, _ *http.Request) {
	docs, _ := s.ws.Scan()
	counts := map[string]int{}
	for _, d := range docs {
		for _, t := range d.Meta.Tags {
			counts[t]++
		}
	}
	type tagCount struct {
		Tag   string `json:"tag"`
		Count int    `json:"count"`
	}
	out := make([]tagCount, 0, len(counts))
	for t, c := range counts {
		out = append(out, tagCount{t, c})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Tag < out[j].Tag
	})
	writeJSON(w, http.StatusOK, map[string]any{"tags": out})
}

// GET /api/v1/stats — type 別件数・タスク数・今月の daily 数
func (s *Server) handleStats(w http.ResponseWriter, _ *http.Request) {
	docs, _ := s.ws.Scan()
	byType := map[string]int{}
	uncategorized := 0
	dailyThisMonth := 0
	monthPrefix := "daily/" + time.Now().Format("2006/01") + "/"
	for _, d := range docs {
		if d.Uncategorized() {
			uncategorized++
			byType["未分類"]++
		} else {
			byType[d.Meta.Type]++
		}
		if strings.HasPrefix(d.Path, monthPrefix) {
			dailyThisMonth++
		}
	}
	board, _ := tasks.Collect(s.ws.Root)
	countState := func(ts []tasks.Task, state string) int {
		n := 0
		for _, t := range ts {
			if t.State == state {
				n++
			}
		}
		return n
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"totalDocs":      len(docs),
		"byType":         byType,
		"uncategorized":  uncategorized,
		"dailyThisMonth": dailyThisMonth,
		"tasks": map[string]any{
			"today":     map[string]int{"total": len(board.Today), "done": countState(board.Today, "done")},
			"stock":     map[string]int{"total": len(board.Stock), "done": countState(board.Stock, "done")},
			"someday":   len(board.Someday),
			"milestone": map[string]int{"total": len(board.Milestones), "done": countState(board.Milestones, "done")},
		},
	})
}

// searchSizeLimit を超えるファイルは全文検索の対象外（notes/sessions/ の巨大ログ等）。
const searchSizeLimit = 1 << 20 // 1MB

// GET /api/v1/search?q=&type=
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeError(w, http.StatusBadRequest, "q is required")
		return
	}
	typ := r.URL.Query().Get("type")
	lower := strings.ToLower(q)

	type hit struct {
		Path    string `json:"path"`
		Line    int    `json:"line"`
		Snippet string `json:"snippet"`
	}
	const maxHits = 100
	var hits []hit
	skipped := 0

	docs, _ := s.ws.Scan()
	for _, d := range docs {
		if typ != "" && d.Meta.Type != typ {
			continue
		}
		if d.Size > searchSizeLimit {
			skipped++
			continue
		}
		_, content, err := s.ws.Read(d.Path)
		if err != nil {
			continue
		}
		for i, line := range strings.Split(string(content), "\n") {
			if strings.Contains(strings.ToLower(line), lower) {
				hits = append(hits, hit{Path: d.Path, Line: i + 1, Snippet: snippet(line)})
				if len(hits) >= maxHits {
					break
				}
			}
		}
		if len(hits) >= maxHits {
			break
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"hits": hits, "total": len(hits), "truncated": len(hits) >= maxHits, "skippedLargeFiles": skipped,
	})
}

func snippet(line string) string {
	line = strings.TrimSpace(line)
	const max = 200
	if len(line) > max {
		// rune 境界を壊さないように切る
		r := []rune(line)
		if len(r) > max/3 {
			return string(r[:max/3]) + "…"
		}
	}
	return line
}

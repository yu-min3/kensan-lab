package api

import (
	"net/http"
	"regexp"

	"github.com/yu-min3/kensan-lab/apps/kensan/backend/internal/history"
)

// revRe は rev クエリを git のオブジェクト名（短縮〜完全 SHA-1）に限定する。
// 任意の revspec（HEAD~3 や branch 名）を git へ渡さないための入力検証。
var revRe = regexp.MustCompile(`^[0-9a-fA-F]{7,40}$`)

// GET /api/v1/history/{path...}        → コミット一覧（新しい順）
// GET /api/v1/history/{path...}?rev=H  → その版のファイル内容
//
// アプリは git に書き込まない（commit は通常の作業フロー）。ここは読むだけ。
func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	rel := r.PathValue("path")
	if _, err := s.ws.Abs(rel); err != nil {
		writeError(w, http.StatusBadRequest, "invalid path: "+rel)
		return
	}

	if rev := r.URL.Query().Get("rev"); rev != "" {
		if !revRe.MatchString(rev) {
			writeError(w, http.StatusBadRequest, "invalid rev")
			return
		}
		content, err := history.Show(r.Context(), s.ws.Root, rel, rev)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"rev": rev, "content": content})
		return
	}

	commits, err := history.Log(r.Context(), s.ws.Root, rel)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if commits == nil {
		commits = []history.Commit{} // git repo でない等 → 空配列で返す
	}
	writeJSON(w, http.StatusOK, map[string]any{"commits": commits})
}

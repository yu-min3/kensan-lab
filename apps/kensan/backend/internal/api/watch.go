package api

import (
	"github.com/yu-min3/kensan-lab/apps/kensan/backend/internal/projectwatch"
	"github.com/yu-min3/kensan-lab/apps/kensan/backend/internal/watchmetrics"
	"net/http"
	"time"
)

func (s *Server) handleWatch(w http.ResponseWriter, r *http.Request) {
	v, e := projectwatch.Read(s.ws.Root)
	if e != nil {
		writeError(w, http.StatusInternalServerError, "常設表示の記録を読み取れません")
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) handleWatchMetrics(w http.ResponseWriter, r *http.Request) {
	v, e := watchmetrics.Read(s.ws.Root, time.Now())
	if e != nil {
		writeError(w, http.StatusInternalServerError, "指標の観測値を読み取れません")
		return
	}
	writeJSON(w, http.StatusOK, v)
}

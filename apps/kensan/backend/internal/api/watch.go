package api

import (
	"github.com/yu-min3/kensan-lab/apps/kensan/backend/internal/projectwatch"
	"net/http"
)

func (s *Server) handleWatch(w http.ResponseWriter, r *http.Request) {
	v, e := projectwatch.Read(s.ws.Root)
	if e != nil {
		writeError(w, http.StatusInternalServerError, "常設表示の記録を読み取れません")
		return
	}
	writeJSON(w, http.StatusOK, v)
}

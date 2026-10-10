package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/yu-min3/kensan-lab/apps/kensan/backend/internal/tasks"
	"github.com/yu-min3/kensan-lab/apps/kensan/backend/internal/today"
)

func (s *Server) handleToday(w http.ResponseWriter, r *http.Request) {
	v, err := today.Load(s.ws, time.Now())
	if err != nil {
		writeOpError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}
func (s *Server) handleRoutineState(w http.ResponseWriter, r *http.Request) {
	var req struct {
		File string `json:"file"`
		ID   string `json:"id"`
		Date string `json:"date"`
		Done bool   `json:"done"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.File == "" || req.ID == "" {
		writeError(w, 400, "file, id and date are required")
		return
	}
	now := time.Now().In(today.JST)
	if req.Date != now.Format("2006-01-02") {
		writeError(w, 409, "日付が変わりました。再読込してください。")
		return
	}
	if err := today.SetRoutine(s.ws, req.File, req.ID, req.Date, req.Done, now); err != nil {
		writeOpError(w, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"done": req.Done})
}
func (s *Server) handleTaskDefer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		File string `json:"file"`
		Line int    `json:"line"`
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.File == "" || req.Line < 1 {
		writeError(w, 400, "file, line, text are required")
		return
	}
	t, err := tasks.ReviewLater(s.ws, req.File, req.Line, req.Text)
	if err != nil {
		writeOpError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"task": t})
}

func (s *Server) handleTaskTriage(w http.ResponseWriter, r *http.Request) {
	var req struct {
		File   string `json:"file"`
		Line   int    `json:"line"`
		Text   string `json:"text"`
		Action string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.File == "" || req.Line < 1 {
		writeError(w, 400, "file, line, text, action are required")
		return
	}
	if req.Action != "today" && req.Action != "later" && req.Action != "skip" {
		writeError(w, 400, "invalid triage action")
		return
	}
	t, err := tasks.Triage(s.ws, req.File, req.Line, req.Text, req.Action)
	if err != nil {
		writeOpError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"task": t})
}

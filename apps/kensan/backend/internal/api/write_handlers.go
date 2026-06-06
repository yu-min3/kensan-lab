package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/yu-min3/kensan-lab/apps/kensan/backend/internal/tasks"
	"github.com/yu-min3/kensan-lab/apps/kensan/backend/internal/workspace"
)

// POST /api/v1/files {path, content}
func (s *Server) handleFileCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Path == "" {
		writeError(w, http.StatusBadRequest, "path and content are required")
		return
	}
	if err := s.ws.Create(req.Path, []byte(req.Content)); err != nil {
		writeOpError(w, err)
		return
	}
	doc, _, err := s.ws.Read(req.Path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"doc": doc})
}

// PUT /api/v1/files/{path...} {content, baseMtime}
// baseMtime は直前の GET で得た mtime（RFC3339Nano）。省略時はロックなし上書き。
func (s *Server) handleFileUpdate(w http.ResponseWriter, r *http.Request) {
	rel := r.PathValue("path")
	var req struct {
		Content   string    `json:"content"`
		BaseMtime time.Time `json:"baseMtime"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.ws.Write(rel, []byte(req.Content), req.BaseMtime); err != nil {
		writeOpError(w, err)
		return
	}
	doc, _, err := s.ws.Read(rel)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"doc": doc})
}

// DELETE /api/v1/files/{path...}
func (s *Server) handleFileDelete(w http.ResponseWriter, r *http.Request) {
	if err := s.ws.Delete(r.PathValue("path")); err != nil {
		writeOpError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// POST /api/v1/tasks/move {file, line, text, to, project?, date?}
// かんばんのドラッグ = この 1 操作。to: today | stock | daily
func (s *Server) handleTaskMove(w http.ResponseWriter, r *http.Request) {
	var req struct {
		File    string `json:"file"`
		Line    int    `json:"line"`
		Text    string `json:"text"`
		To      string `json:"to"`
		Project string `json:"project,omitempty"`
		Date    string `json:"date,omitempty"` // YYYY-MM-DD（to=daily 用、省略時は今日）
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.File == "" || req.Line < 1 || req.To == "" {
		writeError(w, http.StatusBadRequest, "file, line, text, to are required")
		return
	}
	dest := tasks.Dest{Kind: req.To, Project: req.Project}
	if req.Date != "" {
		t, err := time.Parse("2006-01-02", req.Date)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid date: "+req.Date)
			return
		}
		dest.Date = t
	}
	moved, err := tasks.Move(s.ws, req.File, req.Line, req.Text, dest)
	if err != nil {
		writeOpError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"task": moved})
}

// PATCH /api/v1/tasks {file, line, text, state}
func (s *Server) handleTaskState(w http.ResponseWriter, r *http.Request) {
	var req struct {
		File  string `json:"file"`
		Line  int    `json:"line"`
		Text  string `json:"text"`
		State string `json:"state"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.File == "" || req.Line < 1 {
		writeError(w, http.StatusBadRequest, "file, line, text, state are required")
		return
	}
	updated, err := tasks.SetState(s.ws, req.File, req.Line, req.Text, req.State)
	if err != nil {
		writeOpError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"task": updated})
}

// writeOpError はドメインエラーを HTTP ステータスに写像する。
func writeOpError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, workspace.ErrConflict), errors.Is(err, tasks.ErrLineMismatch):
		writeError(w, http.StatusConflict, err.Error())
	case os.IsNotExist(err):
		writeError(w, http.StatusNotFound, err.Error())
	default:
		writeError(w, http.StatusBadRequest, err.Error())
	}
}

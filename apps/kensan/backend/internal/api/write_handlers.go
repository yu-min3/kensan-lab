package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/yu-min3/kensan-lab/apps/kensan/backend/internal/projects"
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
	// frontmatter の updated を自動更新（ユーザーは触らない方針）
	content := workspace.TouchUpdated([]byte(req.Content), time.Now())
	if err := s.ws.Write(rel, content, req.BaseMtime); err != nil {
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

// POST /api/v1/tasks/move {file, line, text, date?}
// 完了タスクの daily 退避（/reflection 相当）。今日やる ⇄ ストックは /tasks/today で行う。
func (s *Server) handleTaskMove(w http.ResponseWriter, r *http.Request) {
	var req struct {
		File string `json:"file"`
		Line int    `json:"line"`
		Text string `json:"text"`
		Date string `json:"date,omitempty"` // YYYY-MM-DD（省略時は今日）
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.File == "" || req.Line < 1 {
		writeError(w, http.StatusBadRequest, "file, line, text are required")
		return
	}
	dest := tasks.Dest{Kind: "daily"}
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

// POST /api/v1/tasks/today {file, line, text, on}
// @today タグの付け外し = 今日やる ⇄ ストックの切替。行はその場で書き換わる。
func (s *Server) handleTaskToday(w http.ResponseWriter, r *http.Request) {
	var req struct {
		File string `json:"file"`
		Line int    `json:"line"`
		Text string `json:"text"`
		On   bool   `json:"on"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.File == "" || req.Line < 1 {
		writeError(w, http.StatusBadRequest, "file, line, text are required")
		return
	}
	updated, err := tasks.SetToday(s.ws, req.File, req.Line, req.Text, req.On)
	if err != nil {
		writeOpError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"task": updated})
}

// POST /api/v1/tasks/priority {file, line, text, priority}
// ストックの 1 タスクに @p(N) を設定（ドラッグ並べ替えの 1 手）。
func (s *Server) handleTaskPriority(w http.ResponseWriter, r *http.Request) {
	var req struct {
		File     string `json:"file"`
		Line     int    `json:"line"`
		Text     string `json:"text"`
		Priority int    `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.File == "" || req.Line < 1 {
		writeError(w, http.StatusBadRequest, "file, line, text are required")
		return
	}
	updated, err := tasks.SetPriority(s.ws, req.File, req.Line, req.Text, req.Priority)
	if err != nil {
		writeOpError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"task": updated})
}

// POST /api/v1/tasks/reorder {items:[{file,line,text,priority}]}
// ストック全体の @p(N) 一括設定（中間値の隙間が尽きたときの再採番フォールバック）。
func (s *Server) handleTaskReorder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Items []struct {
			File     string `json:"file"`
			Line     int    `json:"line"`
			Text     string `json:"text"`
			Priority int    `json:"priority"`
		} `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Items) == 0 {
		writeError(w, http.StatusBadRequest, "items are required")
		return
	}
	for _, it := range req.Items {
		if _, err := tasks.SetPriority(s.ws, it.File, it.Line, it.Text, it.Priority); err != nil {
			writeOpError(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "count": len(req.Items)})
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

// POST /api/v1/tasks/save — タスクの作成・編集の統一エンドポイント。
// locator（file/line/text）があれば編集（project 変更はファイル間移動）、無ければ作成。
func (s *Server) handleTaskSave(w http.ResponseWriter, r *http.Request) {
	var req struct {
		File      string `json:"file"`  // 編集時の locator（作成時は空）
		Line      int    `json:"line"`  //
		Text      string `json:"text"`  // 現在の生テキスト（楽観ロック）
		Project   string `json:"project"`
		Display   string `json:"display"`
		Today     bool   `json:"today"`
		Due       string `json:"due"`
		Milestone string `json:"milestone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Display == "" {
		writeError(w, http.StatusBadRequest, "display is required")
		return
	}
	var out tasks.Task
	var err error
	if req.File != "" && req.Line > 0 {
		out, err = tasks.EditTask(s.ws, req.File, req.Line, req.Text, req.Project, req.Display, req.Today, req.Due, req.Milestone)
	} else {
		out, err = tasks.CreateTask(s.ws, req.Project, req.Display, req.Today, req.Due, req.Milestone)
	}
	if err != nil {
		writeOpError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"task": out})
}

// POST /api/v1/tasks/due {file, line, text, due}
// 行の @due(YYYY-MM-DD) を設定（空文字で除去）。マイルストーン/タスクの期限。
func (s *Server) handleTaskDue(w http.ResponseWriter, r *http.Request) {
	var req struct {
		File string `json:"file"`
		Line int    `json:"line"`
		Text string `json:"text"`
		Due  string `json:"due"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.File == "" || req.Line < 1 {
		writeError(w, http.StatusBadRequest, "file, line, text are required")
		return
	}
	out, err := tasks.SetDue(s.ws, req.File, req.Line, req.Text, req.Due)
	if err != nil {
		writeOpError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"task": out})
}

// POST /api/v1/tasks/add {file, section, display}
// 指定セクション末尾にチェックボックス行を追加（マイルストーン追加など）。
func (s *Server) handleTaskAdd(w http.ResponseWriter, r *http.Request) {
	var req struct {
		File    string `json:"file"`
		Section string `json:"section"`
		Display string `json:"display"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.File == "" || req.Section == "" || req.Display == "" {
		writeError(w, http.StatusBadRequest, "file, section, display are required")
		return
	}
	out, err := tasks.AddLine(s.ws, req.File, req.Section, req.Display)
	if err != nil {
		writeOpError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"task": out})
}

// PATCH /api/v1/projects/{name} {status, deadline, goal}
func (s *Server) handleProjectUpdate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Status   string `json:"status"`
		Deadline string `json:"deadline"`
		Goal     string `json:"goal"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := projects.Update(s.ws, r.PathValue("name"), req.Status, req.Deadline, req.Goal); err != nil {
		writeOpError(w, err)
		return
	}
	d, err := projects.Load(s.ws.Root, r.PathValue("name"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, d)
}

// POST /api/v1/projects {name}
func (s *Server) handleProjectCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if err := projects.Create(s.ws, req.Name, time.Now()); err != nil {
		writeOpError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"name": req.Name})
}

// POST /api/v1/tasks/text {file, line, text, display}
// タスクの本文をインライン編集（行内タグは維持）。
func (s *Server) handleTaskText(w http.ResponseWriter, r *http.Request) {
	var req struct {
		File    string `json:"file"`
		Line    int    `json:"line"`
		Text    string `json:"text"`
		Display string `json:"display"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.File == "" || req.Line < 1 {
		writeError(w, http.StatusBadRequest, "file, line, text, display are required")
		return
	}
	updated, err := tasks.SetText(s.ws, req.File, req.Line, req.Text, req.Display)
	if err != nil {
		writeOpError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"task": updated})
}

// POST /api/v1/tasks/delete {file, line, text}
// タスク行を 1 行削除する（楽観ロック）。
func (s *Server) handleTaskDelete(w http.ResponseWriter, r *http.Request) {
	var req struct {
		File string `json:"file"`
		Line int    `json:"line"`
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.File == "" || req.Line < 1 {
		writeError(w, http.StatusBadRequest, "file, line, text are required")
		return
	}
	if err := tasks.DeleteLine(s.ws, req.File, req.Line, req.Text); err != nil {
		writeOpError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
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

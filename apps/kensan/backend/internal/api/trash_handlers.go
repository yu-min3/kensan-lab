package api

import (
	"encoding/json"
	"net/http"

	"github.com/yu-min3/kensan-lab/apps/kensan/backend/internal/tasks"
)

// ゴミ箱（.kensan/trash.md）。app でのタスク削除は即消しでなくここへ退避される。
// 一覧 / 復元 / 完全削除の 3 操作のみ（普段は目立たない場所、が要件）。

// GET /api/v1/trash — 削除タスクの一覧（新しい順）
func (s *Server) handleTrashList(w http.ResponseWriter, _ *http.Request) {
	items, err := tasks.TrashList(s.ws)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

// POST /api/v1/trash/restore {line, text}
// 元のファイル・セクションへ戻す（元が無ければ todo.md ## Now）。text は Raw（楽観ロック）。
func (s *Server) handleTrashRestore(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Line int    `json:"line"`
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Line < 1 || req.Text == "" {
		writeError(w, http.StatusBadRequest, "line and text are required")
		return
	}
	task, err := tasks.RestoreFromTrash(s.ws, req.Line, req.Text)
	if err != nil {
		writeOpError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"task": task})
}

// POST /api/v1/trash/delete {line, text} — 完全削除（復元不可）
func (s *Server) handleTrashPurge(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Line int    `json:"line"`
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Line < 1 || req.Text == "" {
		writeError(w, http.StatusBadRequest, "line and text are required")
		return
	}
	if err := tasks.PurgeTrashEntry(s.ws, req.Line, req.Text); err != nil {
		writeOpError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "purged"})
}

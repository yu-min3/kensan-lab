// Package api は kensan の読み取り REST API を提供する。
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/yu-min3/kensan-lab/apps/kensan/backend/internal/workspace"
)

type Server struct {
	ws  *workspace.Workspace
	log *slog.Logger
}

func New(root string, log *slog.Logger) *Server {
	return &Server{ws: workspace.New(root), log: log}
}

// Handler は全ルートを登録した http.Handler を返す。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("GET /api/v1/files", s.handleFiles)
	mux.HandleFunc("POST /api/v1/files", s.handleFileCreate)
	mux.HandleFunc("GET /api/v1/files/{path...}", s.handleFileDetail)
	mux.HandleFunc("PUT /api/v1/files/{path...}", s.handleFileUpdate)
	mux.HandleFunc("DELETE /api/v1/files/{path...}", s.handleFileDelete)
	mux.HandleFunc("GET /api/v1/history/{path...}", s.handleHistory)
	mux.HandleFunc("GET /api/v1/daily", s.handleDaily)
	mux.HandleFunc("GET /api/v1/tasks", s.handleTasks)
	mux.HandleFunc("POST /api/v1/tasks/move", s.handleTaskMove)
	mux.HandleFunc("POST /api/v1/tasks/today", s.handleTaskToday)
	mux.HandleFunc("POST /api/v1/tasks/add", s.handleTaskAdd)
	mux.HandleFunc("POST /api/v1/tasks/due", s.handleTaskDue)
	mux.HandleFunc("POST /api/v1/tasks/priority", s.handleTaskPriority)
	mux.HandleFunc("POST /api/v1/tasks/reorder", s.handleTaskReorder)
	mux.HandleFunc("POST /api/v1/tasks/text", s.handleTaskText)
	mux.HandleFunc("POST /api/v1/tasks/delete", s.handleTaskDelete)
	mux.HandleFunc("PATCH /api/v1/tasks", s.handleTaskState)
	mux.HandleFunc("GET /api/v1/goals", s.handleGoals)
	mux.HandleFunc("GET /api/v1/projects", s.handleProjects)
	mux.HandleFunc("POST /api/v1/projects", s.handleProjectCreate)
	mux.HandleFunc("GET /api/v1/projects/{name}", s.handleProjectDetail)
	mux.HandleFunc("PATCH /api/v1/projects/{name}", s.handleProjectUpdate)
	mux.HandleFunc("POST /api/v1/tasks/save", s.handleTaskSave)
	mux.HandleFunc("GET /api/v1/tags", s.handleTags)
	mux.HandleFunc("GET /api/v1/reviews", s.handleReviews)
	mux.HandleFunc("GET /api/v1/reviews/{path...}", s.handleReviewContent)
	mux.HandleFunc("GET /api/v1/stats", s.handleStats)
	mux.HandleFunc("GET /api/v1/search", s.handleSearch)
	return s.middleware(mux)
}

// middleware はアクセスログと開発用 CORS を提供する。
func (s *Server) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		// Vite dev server (localhost:5173) からのアクセスを許可
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, traceparent")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
		s.log.Info("request", "method", r.Method, "path", r.URL.Path, "dur", time.Since(start).String())
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

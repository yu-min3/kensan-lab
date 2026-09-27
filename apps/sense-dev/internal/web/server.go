package web

import (
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"html/template"
	"io"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/core"
)

//go:embed static/*
var static embed.FS

type Server struct {
	store      *core.Store
	token      []byte
	tokensCSS  string
	workerMode string
	allowedNow func(time.Time) bool
	mu         sync.Mutex
	sessions   map[string]session
}

type session struct {
	csrf    string
	expires time.Time
}

type agentView struct {
	Agent      core.Agent
	WaitReason string
}

func agentOrder(role string) int {
	switch role {
	case "requirements", "feedback":
		return 0
	case "design_review", "app_acceptance":
		return 1
	case "implementation":
		return 2
	case "implementation_review":
		return 3
	case "release_gate":
		return 4
	default:
		return 5
	}
}

func agentWaitReason(st core.State, a core.Agent, workerMode string, allowed bool, now time.Time) string {
	if st.Stopped {
		return "全体停止"
	}
	if st.PausedUntil != nil && now.Before(*st.PausedUntil) {
		return "Mac 優先"
	}
	task := st.Tasks[a.TaskID]
	if task.Team == core.App && task.Kind == "acceptance" && task.SourceTaskID != "" {
		if st.Tasks[task.SourceTaskID].Status == "decision_wait" {
			return "Yu の判断待ち"
		}
		if task.Status == "revision_wait" {
			return "Platform 修正待ち"
		}
		if task.HeadSHA != "" && task.BaseSHA != "" && task.BaseSHA != task.HeadSHA {
			return "App 作業ツリー更新待ち"
		}
	}
	if a.Status != "ready" {
		return a.Status
	}
	for _, depID := range a.DependsOn {
		if dep, ok := st.Agents[depID]; !ok || dep.Status != "completed" {
			if ok {
				return "依存待ち: " + dep.Role
			}
			return "依存関係が不明"
		}
	}
	for _, depID := range st.Tasks[a.TaskID].DependsOn {
		if dep, ok := st.Tasks[depID]; !ok || dep.Status != "done" {
			return "案件依存待ち"
		}
	}
	if task.Team == core.App && task.Kind == "acceptance" && task.SourceTaskID != "" {
		source := st.Tasks[task.SourceTaskID]
		if task.HeadSHA == "" {
			return "Platform の合格成果物待ち"
		}
		if source.HeadSHA != task.HeadSHA {
			return "Platform 版不一致・要確認"
		}
	}
	if workerMode == "off" {
		return "実行未設定"
	}
	if workerMode == "isolated" && !allowed {
		return "運転時間外"
	}
	if workerMode == "isolated" && st.Tasks[a.TaskID].BaseSHA == "" {
		return "作業ツリー準備待ち"
	}
	return "配車待ち"
}

func New(store *core.Store, tokenFile, tokensCSS, workerMode string, allowedNow func(time.Time) bool) (*Server, error) {
	if workerMode != "off" && workerMode != "mock" && workerMode != "isolated" {
		return nil, errors.New("unknown worker mode")
	}
	if workerMode == "isolated" && allowedNow == nil {
		return nil, errors.New("isolated worker requires a run window")
	}
	info, err := os.Stat(tokenFile)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("admin token file must be regular and mode 0600 or stricter")
	}
	b, err := os.ReadFile(tokenFile)
	if err != nil {
		return nil, err
	}
	b = []byte(strings.TrimSpace(string(b)))
	if len(b) < 32 {
		return nil, errors.New("admin token must be at least 32 characters")
	}
	if info, err := os.Stat(tokensCSS); err != nil || !info.Mode().IsRegular() {
		return nil, errors.New("design tokens CSS file is required")
	}
	return &Server{store: store, token: b, tokensCSS: tokensCSS, workerMode: workerMode, allowedNow: allowedNow, sessions: map[string]session{}}, nil
}

func LoopbackOnly(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return errors.New("private service may listen only on numeric loopback")
	}
	return nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /login", s.loginPage)
	mux.HandleFunc("POST /login", s.login)
	mux.HandleFunc("POST /logout", s.auth(s.logout, true))
	mux.HandleFunc("GET /", s.auth(s.home, false))
	mux.HandleFunc("GET /static/app.css", s.staticCSS)
	mux.HandleFunc("GET /static/tokens.css", s.tokens)
	mux.HandleFunc("GET /static/app.js", s.staticJS)
	mux.HandleFunc("GET /api/state", s.auth(s.state, false))
	mux.HandleFunc("POST /api/tasks", s.auth(s.createTask, true))
	mux.HandleFunc("POST /api/questions/{id}/answer", s.auth(s.answerQuestion, true))
	mux.HandleFunc("POST /api/approvals/{id}/decide", s.auth(s.decideApproval, true))
	mux.HandleFunc("POST /api/reports/preview", s.auth(s.previewReport, true))
	mux.HandleFunc("POST /api/mac-priority", s.auth(s.macPriority, true))
	mux.HandleFunc("POST /api/mac-priority/clear", s.auth(s.macPriorityClear, true))
	mux.HandleFunc("POST /api/stop", s.auth(s.stop, true))
	mux.HandleFunc("POST /api/resume", s.auth(s.resume, true))
	return securityHeaders(mux)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.Host)
		if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
			http.Error(w, "loopback host required", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'self'; script-src 'self'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) auth(fn http.HandlerFunc, write bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("sense_session")
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		s.mu.Lock()
		value, ok := s.sessions[cookie.Value]
		if ok && time.Now().After(value.expires) {
			delete(s.sessions, cookie.Value)
			ok = false
		}
		s.mu.Unlock()
		if !ok {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		if write {
			if r.Header.Get("Origin") != "" && r.Header.Get("Origin") != "http://"+r.Host {
				http.Error(w, "origin mismatch", http.StatusForbidden)
				return
			}
			if subtle.ConstantTimeCompare([]byte(r.FormValue("csrf")), []byte(value.csrf)) != 1 {
				http.Error(w, "CSRF check failed", http.StatusForbidden)
				return
			}
		}
		fn(w, r)
	}
}

func randomHex() (string, error) {
	b := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *Server) loginPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	b, _ := static.ReadFile("static/login.html")
	_, _ = w.Write(b)
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || subtle.ConstantTimeCompare([]byte(r.Form.Get("token")), s.token) != 1 {
		http.Error(w, "login failed", http.StatusUnauthorized)
		return
	}
	id, err := randomHex()
	if err != nil {
		http.Error(w, "session unavailable", http.StatusInternalServerError)
		return
	}
	csrf, err := randomHex()
	if err != nil {
		http.Error(w, "session unavailable", http.StatusInternalServerError)
		return
	}
	s.mu.Lock()
	s.sessions[id] = session{csrf: csrf, expires: time.Now().Add(12 * time.Hour)}
	s.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "sense_session", Value: id, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: 43200})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	cookie, _ := r.Cookie("sense_session")
	s.mu.Lock()
	delete(s.sessions, cookie.Value)
	s.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "sense_session", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (s *Server) csrf(r *http.Request) string {
	cookie, _ := r.Cookie("sense_session")
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessions[cookie.Value].csrf
}

func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	b, _ := static.ReadFile("static/index.html")
	page, err := template.New("home").Parse(string(b))
	if err != nil {
		http.Error(w, "template unavailable", http.StatusInternalServerError)
		return
	}
	st := s.store.Snapshot()
	tasks := make([]core.Task, 0, len(st.Tasks))
	for _, t := range st.Tasks {
		tasks = append(tasks, t)
	}
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].CreatedAt.After(tasks[j].CreatedAt) })
	messages := make([]core.Message, 0, len(st.Messages))
	for _, m := range st.Messages {
		messages = append(messages, m)
	}
	sort.Slice(messages, func(i, j int) bool { return messages[i].CreatedAt.After(messages[j].CreatedAt) })
	agents := make([]agentView, 0, len(st.Agents))
	for _, a := range st.Agents {
		allowed := s.allowedNow == nil || s.allowedNow(time.Now())
		agents = append(agents, agentView{Agent: a, WaitReason: agentWaitReason(st, a, s.workerMode, allowed, time.Now())})
	}
	sort.Slice(agents, func(i, j int) bool {
		ai, aj := agents[i].Agent, agents[j].Agent
		if ai.TaskID != aj.TaskID {
			return st.Tasks[ai.TaskID].CreatedAt.Before(st.Tasks[aj.TaskID].CreatedAt)
		}
		if agentOrder(ai.Role) != agentOrder(aj.Role) {
			return agentOrder(ai.Role) < agentOrder(aj.Role)
		}
		return ai.ID < aj.ID
	})
	questions := make([]core.Question, 0, len(st.Questions))
	for _, q := range st.Questions {
		questions = append(questions, q)
	}
	sort.Slice(questions, func(i, j int) bool { return questions[i].CreatedAt.After(questions[j].CreatedAt) })
	approvals := make([]core.ApprovalRequest, 0, len(st.Approvals))
	for _, a := range st.Approvals {
		approvals = append(approvals, a)
	}
	sort.Slice(approvals, func(i, j int) bool { return approvals[i].CreatedAt.After(approvals[j].CreatedAt) })
	reports := make([]core.DailyReport, 0, len(st.Reports))
	for _, report := range st.Reports {
		reports = append(reports, report)
	}
	sort.Slice(reports, func(i, j int) bool { return reports[i].Date > reports[j].Date })
	outbox := make([]core.ReportOutboxEntry, 0, len(st.ReportOutbox))
	for _, entry := range st.ReportOutbox {
		outbox = append(outbox, entry)
	}
	sort.Slice(outbox, func(i, j int) bool { return outbox[i].Date > outbox[j].Date })
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = page.Execute(w, struct {
		CSRF        string
		Tasks       []core.Task
		Messages    []core.Message
		Agents      []agentView
		Questions   []core.Question
		Approvals   []core.ApprovalRequest
		Reports     []core.DailyReport
		Outbox      []core.ReportOutboxEntry
		Stopped     bool
		PausedUntil *time.Time
		WorkerMode  string
		WindowOpen  bool
	}{s.csrf(r), tasks, messages, agents, questions, approvals, reports, outbox, st.Stopped, st.PausedUntil, s.workerMode, s.allowedNow == nil || s.allowedNow(time.Now())})
}

func (s *Server) staticCSS(w http.ResponseWriter, r *http.Request) {
	b, _ := static.ReadFile("static/app.css")
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	_, _ = w.Write(b)
}

func (s *Server) staticJS(w http.ResponseWriter, r *http.Request) {
	b, _ := static.ReadFile("static/app.js")
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	_, _ = w.Write(b)
}

func (s *Server) tokens(w http.ResponseWriter, r *http.Request) {
	b, err := os.ReadFile(s.tokensCSS)
	if err != nil {
		http.Error(w, "design tokens unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	_, _ = w.Write(b)
}

func (s *Server) state(w http.ResponseWriter, r *http.Request) {
	st := s.store.Snapshot()
	// Session ids and private memo content are deliberately absent from the UI.
	for id, a := range st.Agents {
		a.SessionID, a.InputHash = "", ""
		st.Agents[id] = a
	}
	st.Decisions = nil
	st.Intents = nil
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(st)
}

func (s *Server) createTask(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	kind := r.Form.Get("kind")
	if kind == "feedback" {
		kind = "analysis"
	}
	var err error
	if source := strings.TrimSpace(r.Form.Get("source_task")); source != "" {
		if core.Team(r.Form.Get("team")) != core.App || kind != "acceptance" {
			http.Error(w, "source task is only valid for App acceptance", http.StatusBadRequest)
			return
		}
		platform := s.store.Snapshot().Tasks[source]
		if platform.ID == "" || platform.MissionID != r.Form.Get("mission") || platform.ContractVersion != r.Form.Get("contract") {
			http.Error(w, "source task mission or contract mismatch", http.StatusBadRequest)
			return
		}
		_, _, err = s.store.CreateLinkedAcceptanceTask(source, r.Form.Get("title"))
	} else {
		_, _, err = s.store.CreatePlannedTask(r.Form.Get("mission"), core.Team(r.Form.Get("team")), kind, r.Form.Get("title"), r.Form.Get("contract"))
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) answerQuestion(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	_, err := s.store.AnswerQuestion(r.PathValue("id"), r.Form.Get("action_id"), r.Form.Get("answer"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	http.Redirect(w, r, "/#question-"+r.PathValue("id"), http.StatusSeeOther)
}

func (s *Server) decideApproval(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	_, err := s.store.DecideApproval(r.PathValue("id"), r.Form.Get("action_id"), r.Form.Get("operation"), r.Form.Get("sha"), r.Form.Get("verdict"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	http.Redirect(w, r, "/#approval-"+r.PathValue("id"), http.StatusSeeOther)
}

func (s *Server) previewReport(w http.ResponseWriter, r *http.Request) {
	if _, err := s.store.PreviewDailyReport(time.Now()); err != nil {
		http.Error(w, "report preview unavailable", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/#reports", http.StatusSeeOther)
}

func (s *Server) macPriority(w http.ResponseWriter, r *http.Request) {
	if err := s.store.SetMacPriority(time.Now().Add(2 * time.Hour)); err != nil {
		http.Error(w, "state write failed", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) macPriorityClear(w http.ResponseWriter, r *http.Request) {
	if err := s.store.SetMacPriority(time.Time{}); err != nil {
		http.Error(w, "state write failed", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) stop(w http.ResponseWriter, r *http.Request) {
	if err := s.store.SetStopped(true); err != nil {
		http.Error(w, "state write failed", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) resume(w http.ResponseWriter, r *http.Request) {
	if err := s.store.SetStopped(false); err != nil {
		http.Error(w, "state write failed", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

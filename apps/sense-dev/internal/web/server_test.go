package web

import (
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/core"
)

func TestIsolatedWorkerWaitReasonOutsideWindow(t *testing.T) {
	state := core.State{Tasks: map[string]core.Task{"task": {ID: "task", Status: "ready"}}, Agents: map[string]core.Agent{}}
	agent := core.Agent{TaskID: "task", Status: "ready"}
	if got := agentWaitReason(state, agent, "isolated", false, time.Now()); got != "運転時間外" {
		t.Fatalf("outside-window reason: %s", got)
	}
	if got := agentWaitReason(state, agent, "isolated", true, time.Now()); got != "作業ツリー準備待ち" {
		t.Fatalf("inside-window unpinned reason: %s", got)
	}
	task := state.Tasks["task"]
	task.BaseSHA = strings.Repeat("a", 40)
	state.Tasks["task"] = task
	if got := agentWaitReason(state, agent, "isolated", true, time.Now()); got != "配車待ち" {
		t.Fatalf("inside-window pinned reason: %s", got)
	}
}

func TestLinkedAppWaitReason(t *testing.T) {
	state := core.State{Tasks: map[string]core.Task{
		"platform": {ID: "platform", Team: core.Platform, Kind: "change", Status: "publish_wait", HeadSHA: strings.Repeat("a", 40)},
		"app":      {ID: "app", Team: core.App, Kind: "acceptance", Status: "ready", SourceTaskID: "platform"},
	}, Agents: map[string]core.Agent{}}
	agent := core.Agent{TaskID: "app", Team: core.App, Status: "ready"}
	if got := agentWaitReason(state, agent, "mock", true, time.Now()); got != "Platform の合格成果物待ち" {
		t.Fatalf("unreceived handoff: %s", got)
	}
	app := state.Tasks["app"]
	app.HeadSHA = strings.Repeat("b", 40)
	state.Tasks["app"] = app
	if got := agentWaitReason(state, agent, "mock", true, time.Now()); got != "Platform 版不一致・要確認" {
		t.Fatalf("stale handoff: %s", got)
	}
	app.Status = "revision_wait"
	state.Tasks["app"] = app
	agent.Status = "completed"
	if got := agentWaitReason(state, agent, "mock", true, time.Now()); got != "Platform 修正待ち" {
		t.Fatalf("correction wait: %s", got)
	}
	platform := state.Tasks["platform"]
	platform.Status = "decision_wait"
	state.Tasks["platform"] = platform
	if got := agentWaitReason(state, agent, "mock", true, time.Now()); got != "Yu の判断待ち" {
		t.Fatalf("decision wait: %s", got)
	}
	app.Status = "decision_wait"
	state.Tasks["app"] = app
	if got := agentWaitReason(state, agent, "mock", true, time.Now()); got != "受入証拠の確認待ち" {
		t.Fatalf("corrupt acceptance wait: %s", got)
	}
}

func TestResumeAgentFromTaskDetailRejectsStaleCard(t *testing.T) {
	dir := t.TempDir()
	store, err := core.Open(filepath.Join(dir, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	task, _, err := store.CreatePlannedTask("m1", core.Platform, "change", "復旧案件", "v1")
	if err != nil {
		t.Fatal(err)
	}
	attempt, _, err := store.ClaimNext(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.FailAttempt(attempt.ID, "auth_required", "login needed", time.Time{}); err != nil {
		t.Fatal(err)
	}
	tokenFile := filepath.Join(dir, "token")
	cssFile := filepath.Join(dir, "tokens.css")
	if err := os.WriteFile(tokenFile, []byte(strings.Repeat("t", 64)), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cssFile, []byte(":root{}"), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := New(store, tokenFile, cssFile, "off", nil)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(s.Handler())
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	resp, err := client.PostForm(server.URL+"/login", url.Values{"token": {strings.Repeat("t", 64)}})
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	resp, err = client.Get(server.URL + "/tasks/" + task.ID)
	if err != nil {
		t.Fatal(err)
	}
	page, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(page), "この Agent を再開") || !strings.Contains(string(page), "auth_required") {
		t.Fatal("resume action missing from task detail")
	}
	csrf := regexp.MustCompile(`name="csrf" value="([a-f0-9]+)"`).FindSubmatch(page)
	if len(csrf) != 2 {
		t.Fatal("CSRF missing")
	}
	agent := store.Snapshot().Agents[attempt.AgentID]
	form := url.Values{"csrf": {string(csrf[1])}, "status": {"auth_required"}, "generation": {"0"}, "reason": {"本人が再認証を確認"}}
	form.Set("generation", fmt.Sprint(agent.SessionGeneration))
	resp, err = client.PostForm(server.URL+"/api/agents/"+agent.ID+"/resume", form)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	resumed := store.Snapshot().Agents[agent.ID]
	if resumed.Status != "ready" || resumed.SessionGeneration != agent.SessionGeneration+1 {
		t.Fatalf("resume state: %s generation %d", resumed.Status, resumed.SessionGeneration)
	}
	seenReason := false
	for _, event := range store.Snapshot().Events {
		if event.Type == "agent_resumed" && event.Subject == agent.ID && event.Detail == form.Get("reason") {
			seenReason = true
		}
	}
	if !seenReason {
		t.Fatal("resume reason missing from event")
	}
	resp, err = client.PostForm(server.URL+"/api/agents/"+agent.ID+"/resume", form)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("stale card: %d", resp.StatusCode)
	}
	if store.Snapshot().Agents[agent.ID].SessionGeneration != resumed.SessionGeneration {
		t.Fatal("stale card resumed twice")
	}
}

func TestCreateLinkedAppAcceptanceFromForm(t *testing.T) {
	dir := t.TempDir()
	store, err := core.Open(filepath.Join(dir, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	platform, _, err := store.CreatePlannedTask("golden-path", core.Platform, "change", "canary", "v1")
	if err != nil {
		t.Fatal(err)
	}
	tokenFile := filepath.Join(dir, "token")
	cssFile := filepath.Join(dir, "tokens.css")
	if err := os.WriteFile(tokenFile, []byte(strings.Repeat("t", 64)), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cssFile, []byte(":root{}"), 0600); err != nil {
		t.Fatal(err)
	}
	server, err := New(store, tokenFile, cssFile, "off", nil)
	if err != nil {
		t.Fatal(err)
	}
	form := url.Values{"source_task": {platform.ID}, "mission": {"golden-path"}, "contract": {"v1"}, "team": {"app"}, "kind": {"acceptance"}, "title": {"consumer canary"}}
	request := func() *http.Request {
		return httptest.NewRequest(http.MethodPost, "/api/tasks", strings.NewReader(form.Encode()))
	}
	form.Set("contract", "wrong")
	bad := request()
	bad.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	badResponse := httptest.NewRecorder()
	server.createTask(badResponse, bad)
	if badResponse.Code != http.StatusBadRequest || len(store.Snapshot().Tasks) != 1 {
		t.Fatal("mismatched contract was accepted")
	}
	form.Set("contract", "v1")
	good := request()
	good.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	goodResponse := httptest.NewRecorder()
	server.createTask(goodResponse, good)
	if goodResponse.Code != http.StatusSeeOther {
		t.Fatalf("linked task form: %d %s", goodResponse.Code, goodResponse.Body.String())
	}
	linked := 0
	for _, task := range store.Snapshot().Tasks {
		if task.SourceTaskID == platform.ID && task.Team == core.App && task.Kind == "acceptance" {
			linked++
		}
	}
	if linked != 1 {
		t.Fatalf("linked App tasks=%d", linked)
	}
}

func TestPrivateWebLoginCSRFAndTask(t *testing.T) {
	dir := t.TempDir()
	store, err := core.Open(filepath.Join(dir, "state"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.SeedKnowledge(); err != nil {
		t.Fatal(err)
	}
	if _, created, err := store.QueueDailyReport(time.Date(2026, 9, 27, 11, 0, 0, 0, time.UTC)); err != nil || !created {
		t.Fatalf("scheduled report setup failed: %t %v", created, err)
	}
	tokenFile := filepath.Join(dir, "admin-token")
	if err := os.WriteFile(tokenFile, []byte(strings.Repeat("t", 64)), 0600); err != nil {
		t.Fatal(err)
	}
	css := filepath.Join(dir, "tokens.css")
	if err := os.WriteFile(css, []byte(":root{--background:40 27% 94%}"), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := New(store, tokenFile, css, "off", nil)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(s.Handler())
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	resp, err := client.Get(server.URL + "/login")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("login page: %d", resp.StatusCode)
	}
	resp.Body.Close()
	resp, err = client.Get(server.URL + "/static/app.css")
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("public stylesheet unavailable: %v %v", resp, err)
	}
	resp.Body.Close()
	resp, err = client.Get(server.URL + "/static/app.js")
	if err != nil || resp.StatusCode != 200 || !strings.Contains(resp.Header.Get("Content-Security-Policy"), "script-src 'self'") {
		t.Fatalf("draft script or CSP unavailable: %v %v", resp, err)
	}
	resp.Body.Close()
	resp, err = client.PostForm(server.URL+"/login", url.Values{"token": {strings.Repeat("t", 64)}})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("login failed: %d %s", resp.StatusCode, b)
	}
	if !strings.Contains(string(b), `id="daily-2026-09-27"`) || !strings.Contains(string(b), "waiting_destination") {
		t.Fatal("scheduled outbox is missing from private dashboard")
	}
	csrfMatch := regexp.MustCompile(`name="csrf" value="([a-f0-9]+)"`).FindSubmatch(b)
	if len(csrfMatch) != 2 {
		t.Fatal("CSRF token missing from authenticated page")
	}
	form := url.Values{"mission": {"golden-path"}, "team": {"platform"}, "kind": {"change"}, "title": {"契約を確認する"}, "contract": {"draft-v1"}}
	resp, err = client.PostForm(server.URL+"/api/tasks", form)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if len(store.Snapshot().Tasks) != 0 {
		t.Fatal("task accepted without CSRF")
	}
	form.Set("csrf", string(csrfMatch[1]))
	resp, err = client.PostForm(server.URL+"/api/tasks", form)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if len(store.Snapshot().Tasks) != 1 {
		t.Fatal("valid task not persisted")
	}
	if len(store.Snapshot().Agents) != 5 {
		t.Fatal("change task did not receive five independent stage agents")
	}
	var requirements core.Agent
	for _, a := range store.Snapshot().Agents {
		if a.Role == "requirements" {
			requirements = a
		}
	}
	question, err := store.AskQuestion(requirements.ID, "対象の契約版は？")
	if err != nil {
		t.Fatal(err)
	}
	answerForm := url.Values{"csrf": {string(csrfMatch[1])}, "action_id": {question.ID}, "answer": {"draft-v1"}}
	resp, err = client.PostForm(server.URL+"/api/questions/"+question.ID+"/answer", answerForm)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if store.Snapshot().Questions[question.ID].Answer != "draft-v1" {
		t.Fatal("mobile answer not persisted")
	}
	resp, err = client.PostForm(server.URL+"/api/questions/"+question.ID+"/answer", answerForm)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatal("replayed answer was not idempotent")
	}
	sha := strings.Repeat("a", 40)
	var task core.Task
	for _, tsk := range store.Snapshot().Tasks {
		task = tsk
	}
	if err := store.SetHeadSHA(task.ID, sha); err != nil {
		t.Fatal(err)
	}
	gate, err := store.AddAgent(task.ID, "release_gate", "codex", "gpt-6-astra")
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range []core.Agent{requirements, gate} {
		m, err := store.BuildManifest(a.ID, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := store.SetAgentSession(a.ID, a.Provider, a.Model, "thread-"+a.ID, m.InputSHA256, 1); err != nil {
			t.Fatal(err)
		}
	}
	source, err := store.PutArtifact(requirements.ID, "change", []byte("diff"))
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := store.PutArtifact(gate.ID, "review", []byte("needs Yu decision"))
	if err != nil {
		t.Fatal(err)
	}
	d, err := store.RecordReleaseDecision(core.ReleaseDecision{AuthorAgentID: requirements.ID, GateAgentID: gate.ID, Verdict: "needs_human", Reason: "公開済み app の変更", Operation: "merge", Repository: "yu-min3/kensan-lab", Ref: "refs/heads/feat/canary", HeadSHA: sha, TargetEnvironment: "private-canary", PolicyVersion: core.ReleasePolicyVersion, ArtifactRefs: []core.ArtifactRef{{ID: source.ID, Version: source.Version, SHA256: source.SHA256}}, EvidenceRefs: []core.ArtifactRef{{ID: evidence.ID, Version: evidence.Version, SHA256: evidence.SHA256}}, ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	approval, err := store.RequestApproval(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	approvalForm := url.Values{"csrf": {string(csrfMatch[1])}, "action_id": {approval.ID}, "operation": {"merge"}, "sha": {sha}, "verdict": {"approved"}}
	resp, err = client.PostForm(server.URL+"/api/approvals/"+approval.ID+"/decide", approvalForm)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if store.Snapshot().Approvals[approval.ID].Status != "approved" || len(store.Snapshot().Intents) != 0 {
		t.Fatal("approval not recorded safely")
	}
	resp, err = client.Get(server.URL + "/tasks/" + task.ID)
	if err != nil {
		t.Fatal(err)
	}
	detail, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(detail), task.Title) || !strings.Contains(string(detail), question.Prompt) || !strings.Contains(string(detail), approval.Reason) || !strings.Contains(string(detail), "担当と待機理由") {
		t.Fatalf("task detail is incomplete: %d", resp.StatusCode)
	}
	resp, err = client.Get(server.URL + "/tasks/missing")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown task detail: %d", resp.StatusCode)
	}
	resp, err = client.PostForm(server.URL+"/api/reports/preview", url.Values{"csrf": {string(csrfMatch[1])}})
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if len(store.Snapshot().Reports) != 1 {
		t.Fatal("daily preview not persisted")
	}
	resp, err = client.Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	b, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(b), "契約を確認する") || !strings.Contains(string(b), "Platform") || !strings.Contains(string(b), "対象の契約版は？") || !strings.Contains(string(b), "承認を記録") && !strings.Contains(string(b), "approved") || !strings.Contains(string(b), "not_configured") || !strings.Contains(string(b), "依存待ち: requirements") {
		t.Fatal("task missing from UI")
	}
}

func TestNonLoopbackBindRejected(t *testing.T) {
	for _, addr := range []string{"0.0.0.0:8787", "[::]:8787", "192.168.0.113:8787", "localhost:8787"} {
		if err := LoopbackOnly(addr); err == nil {
			t.Fatalf("accepted public or ambiguous bind %s", addr)
		}
	}
	for _, addr := range []string{"127.0.0.1:8787", "[::1]:8787"} {
		if err := LoopbackOnly(addr); err != nil {
			t.Fatalf("rejected loopback bind %s: %v", addr, err)
		}
	}
}

package web

import (
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
	tokenFile := filepath.Join(dir, "admin-token")
	if err := os.WriteFile(tokenFile, []byte(strings.Repeat("t", 64)), 0600); err != nil {
		t.Fatal(err)
	}
	css := filepath.Join(dir, "tokens.css")
	if err := os.WriteFile(css, []byte(":root{--background:40 27% 94%}"), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := New(store, tokenFile, css, false)
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
	if len(store.Snapshot().Agents) != 4 {
		t.Fatal("change task did not receive four independent stage agents")
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

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/core"
)

// Exercises the real loopback process and disk state, not only httptest handlers.
// It deliberately uses the simulation worker and makes no model or publish calls.
func TestMockControllerSurvivesProcessRestart(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and restarts the controller process")
	}
	dir := t.TempDir()
	binary := filepath.Join(dir, "sense-dev")
	build := exec.Command("go", "build", "-buildvcs=false", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build controller: %v\n%s", err, output)
	}
	tokenFile := filepath.Join(dir, "admin-token")
	if err := os.WriteFile(tokenFile, []byte(strings.Repeat("t", 64)), 0600); err != nil {
		t.Fatal(err)
	}
	cssFile := filepath.Join(dir, "tokens.css")
	if err := os.WriteFile(cssFile, []byte(":root{}"), 0600); err != nil {
		t.Fatal(err)
	}
	port, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := port.Addr().String()
	port.Close()
	baseURL := "http://" + address
	logPath := filepath.Join(dir, "controller.log")
	stateDir := filepath.Join(dir, "state")

	first := startMockProcess(t, binary, address, stateDir, tokenFile, cssFile, logPath, baseURL)
	client, csrf := loginMockProcess(t, baseURL)
	createMockTask(t, client, baseURL, csrf, "before-restart")
	before := waitMockState(t, client, baseURL, logPath, func(st core.State) bool {
		return len(st.Tasks) == 1 && len(st.Attempts) >= 1
	})
	var firstID string
	for id := range before.Tasks {
		firstID = id
	}
	first.stop(t)

	second := startMockProcess(t, binary, address, stateDir, tokenFile, cssFile, logPath, baseURL)
	client, csrf = loginMockProcess(t, baseURL) // sessions are process-local; state is not.
	recovered := readMockState(t, client, baseURL)
	if _, ok := recovered.Tasks[firstID]; !ok || len(recovered.Attempts) < len(before.Attempts) {
		t.Fatalf("task or attempts were lost across restart: tasks=%d attempts=%d", len(recovered.Tasks), len(recovered.Attempts))
	}
	createMockTask(t, client, baseURL, csrf, "after-restart")
	continued := waitMockState(t, client, baseURL, logPath, func(st core.State) bool {
		if len(st.Tasks) != 2 {
			return false
		}
		for _, attempt := range st.Attempts {
			if task, ok := st.Tasks[attempt.TaskID]; ok && task.Title == "after-restart" {
				return true
			}
		}
		return false
	})
	if _, ok := continued.Tasks[firstID]; !ok {
		t.Fatal("first task disappeared while the restarted dispatcher ran")
	}
	second.stop(t)
}

type mockProcess struct {
	command *exec.Cmd
	done    chan error
}

func startMockProcess(t *testing.T, binary, address, stateDir, tokenFile, cssFile, logPath, baseURL string) *mockProcess {
	t.Helper()
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(binary, "-listen", address, "-data", stateDir, "-admin-token-file", tokenFile, "-tokens-css", cssFile, "-mock-worker")
	command.Stdout, command.Stderr = logFile, logFile
	if err := command.Start(); err != nil {
		logFile.Close()
		t.Fatal(err)
	}
	p := &mockProcess{command: command, done: make(chan error, 1)}
	go func() {
		p.done <- command.Wait()
		logFile.Close()
	}()
	t.Cleanup(func() {
		if command.ProcessState == nil {
			_ = command.Process.Kill()
		}
	})
	client := &http.Client{Timeout: 300 * time.Millisecond, Transport: &http.Transport{Proxy: nil}}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get(baseURL + "/login")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return p
			}
		}
		select {
		case err := <-p.done:
			contents, _ := os.ReadFile(logPath)
			t.Fatalf("controller exited before ready: %v\n%s", err, contents)
		default:
		}
		time.Sleep(50 * time.Millisecond)
	}
	contents, _ := os.ReadFile(logPath)
	t.Fatalf("controller did not become ready:\n%s", contents)
	return nil
}

func (p *mockProcess) stop(t *testing.T) {
	t.Helper()
	if err := p.command.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-p.done:
		if err != nil {
			t.Fatalf("controller did not stop cleanly: %v", err)
		}
	case <-time.After(5 * time.Second):
		_ = p.command.Process.Kill()
		t.Fatal("controller did not stop after SIGTERM")
	}
}

func loginMockProcess(t *testing.T, baseURL string) (*http.Client, string) {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 3 * time.Second, Transport: &http.Transport{Proxy: nil}}
	resp, err := client.PostForm(baseURL+"/login", url.Values{"token": {strings.Repeat("t", 64)}})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login returned %d", resp.StatusCode)
	}
	match := regexp.MustCompile(`name="csrf" value="([a-f0-9]+)"`).FindSubmatch(body)
	if len(match) != 2 {
		t.Fatal("authenticated page omitted CSRF token")
	}
	return client, string(match[1])
}

func createMockTask(t *testing.T, client *http.Client, baseURL, csrf, title string) {
	t.Helper()
	form := url.Values{"csrf": {csrf}, "mission": {"private-restart-test"}, "team": {"platform"}, "kind": {"change"}, "title": {title}, "contract": {"draft-v1"}}
	resp, err := client.PostForm(baseURL+"/api/tasks", form)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create task returned %d", resp.StatusCode)
	}
}

func readMockState(t *testing.T, client *http.Client, baseURL string) core.State {
	t.Helper()
	resp, err := client.Get(baseURL + "/api/state")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("state returned %d", resp.StatusCode)
	}
	var st core.State
	if err := json.NewDecoder(resp.Body).Decode(&st); err != nil {
		t.Fatal(err)
	}
	return st
}

func waitMockState(t *testing.T, client *http.Client, baseURL, logPath string, condition func(core.State) bool) core.State {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		st := readMockState(t, client, baseURL)
		if condition(st) {
			return st
		}
		time.Sleep(100 * time.Millisecond)
	}
	contents, _ := os.ReadFile(logPath)
	t.Fatal(fmt.Sprintf("mock state did not progress before timeout; controller log:\n%s", contents))
	return core.State{}
}

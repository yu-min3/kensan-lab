package provider

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"testing"
	"time"
)

func fakeAppServer(t *testing.T, conn net.Conn, accountType string, usedPercent int) {
	t.Helper()
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		var request struct {
			ID     *int   `json:"id"`
			Method string `json:"method"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &request); err != nil {
			t.Error(err)
			return
		}
		if request.ID == nil {
			continue
		}
		var result any
		switch request.Method {
		case "initialize":
			result = map[string]any{"userAgent": "fake"}
		case "account/read":
			result = map[string]any{"account": map[string]string{"type": accountType}}
		case "account/rateLimits/read":
			result = map[string]any{"rateLimits": map[string]any{"primary": map[string]int{"usedPercent": usedPercent}}, "rateLimitsByLimitId": map[string]any{"codex": map[string]any{"primary": map[string]int{"usedPercent": usedPercent}}}}
		case "thread/start", "thread/resume":
			result = map[string]any{"thread": map[string]string{"id": "thread-123"}}
		case "turn/start":
			result = map[string]any{"turn": map[string]string{"id": "turn-123", "status": "inProgress"}}
		default:
			t.Errorf("unexpected method %s", request.Method)
			return
		}
		if request.Method == "turn/start" {
			// Completion before the request response must not be lost.
			for _, event := range []map[string]any{
				{"method": "item/completed", "params": map[string]any{"item": map[string]string{"type": "agentMessage", "phase": "final_answer", "text": "review passed"}}},
				{"method": "turn/completed", "params": map[string]any{"turn": map[string]string{"status": "completed"}}},
			} {
				line, _ := json.Marshal(event)
				_, _ = conn.Write(append(line, '\n'))
			}
		}
		response, _ := json.Marshal(map[string]any{"id": *request.ID, "result": result})
		_, _ = conn.Write(append(response, '\n'))
	}
}

func TestCodexProtocolSubscriptionOnly(t *testing.T) {
	for _, tc := range []struct {
		name    string
		account string
		usage   int
		wantErr error
	}{
		{"pro", "chatgpt", 20, nil},
		{"api-key-denied", "apiKey", 20, ErrAuthRequired},
		{"quota-wait", "chatgpt", 80, ErrQuotaWait},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client, server := net.Pipe()
			defer client.Close()
			go fakeAppServer(t, server, tc.account, tc.usage)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			called := false
			result, err := runCodexProtocol(ctx, client, client, Request{Model: "gpt-6-sol", Workdir: "/tmp/worktree", Prompt: "review", OnSession: func(id string) error {
				called = true
				if id != "thread-123" {
					t.Fatal(id)
				}
				return nil
			}}, 70)
			if !errors.Is(err, tc.wantErr) && (err != nil || tc.wantErr != nil) {
				t.Fatalf("got %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr == nil && (!called || result.Text != "review passed" || result.Status != "completed") {
				t.Fatalf("unexpected successful result %+v called=%v", result, called)
			}
			if tc.wantErr != nil && called {
				t.Fatal("session started despite auth/quota gate")
			}
		})
	}
}

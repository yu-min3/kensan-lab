package reportdelivery

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/core"
)

func TestSlackSendsFixedChannelAndTaskLinks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("token missing")
		}
		body, _ := io.ReadAll(r.Body)
		for _, want := range []string{`"channel":"C123456789"`, "https://sense.example.test/tasks/task-1", "https://sense.example.test/#question-q1"} {
			if !strings.Contains(string(body), want) {
				t.Errorf("missing %q in message", want)
			}
		}
		_, _ = w.Write([]byte(`{"ok":true,"ts":"123.456"}`))
	}))
	defer server.Close()
	token := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(token, []byte("test-token\n"), 0600); err != nil {
		t.Fatal(err)
	}
	sender := Slack{TokenFile: token, BaseURL: "https://sense.example.test", Endpoint: server.URL, Client: server.Client()}
	entry := core.ReportOutboxEntry{Date: "2026-10-03", Status: "sending", Destination: "C123456789", Summary: "summary", TaskIDs: []string{"task-1"}, PendingIDs: []string{"question-q1"}}
	if receipt, err := sender.Send(context.Background(), entry); err != nil || receipt != "123.456" {
		t.Fatalf("Slack receipt missing: %q %v", receipt, err)
	}
	sender.BaseURL = "http://sense.example.test"
	if err := sender.Validate(); err == nil {
		t.Fatal("insecure report link accepted")
	}
}

package llmopsclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/agentclient"
)

func TestWireContractAndRetryKey(t *testing.T) {
	retries := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture" {
			t.Error("missing auth")
		}
		if r.URL.Path == "/v1/jobs" {
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if body["kind"] != "online" || body["idempotency_key"] != nil || r.Header.Get("Idempotency-Key") != "capture-key" {
				t.Error("wrong contract", body)
			}
		} else {
			retries = append(retries, r.Header.Get("Idempotency-Key"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(202)
		w.Write([]byte(`{"id":"platform-job","state":"queued","kind":"online","attempt_id":"platform-job"}`))
	}))
	defer server.Close()
	c, e := New(server.URL, func(context.Context) (string, error) { return "fixture", nil })
	if e != nil {
		t.Fatal(e)
	}
	_, e = c.Create(context.Background(), agentclient.Request{Profile: "text", Prompt: "extract", Input: map[string]string{"text": "sample"}, OutputSchema: json.RawMessage(`{"type":"object"}`), IdempotencyKey: "capture-key"})
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 2; i++ {
		if _, e := c.Action(context.Background(), "platform-job", "retry"); e != nil {
			t.Fatal(e)
		}
	}
	if len(retries) != 2 || retries[0] == "" || retries[0] != retries[1] {
		t.Fatal("retry response loss can duplicate attempt", retries)
	}
	if _, e := c.Get(context.Background(), "../../other"); e == nil {
		t.Fatal("path injection accepted")
	}
}

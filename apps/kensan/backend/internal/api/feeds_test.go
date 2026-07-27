package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestFeedEndpoints(t *testing.T) {
	ts, root := newTestServer(t)
	write := func(rel, content string) {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	feed := func(date string) string {
		return `---
type: note
tags: [feed, daily-briefing]
title: "Daily Briefing ` + date + `"
status: active
created: ` + date + `
updated: ` + date + `
generated_at: ` + date + `T06:30:00+09:00
---

## 要対応

なし
`
	}
	write("feeds/2026/07/24.md", feed("2026-07-24"))
	write("feeds/2026/07/25.md", feed("2026-07-25"))
	write("feeds/state/import.json", `{
		"schemaVersion":1,
		"reportDate":"2026-07-25",
		"status":"success",
		"lastAttemptAt":"2026-07-25T07:15:00+09:00",
		"lastSuccessAt":"2026-07-25T07:15:00+09:00"
	}`)

	var list struct {
		Feeds []struct {
			Date string `json:"date"`
		} `json:"feeds"`
		Total int `json:"total"`
	}
	if code := getJSON(t, ts.URL+"/api/v1/feeds", &list); code != 200 {
		t.Fatalf("feeds: %d", code)
	}
	if list.Total != 2 || list.Feeds[0].Date != "2026-07-25" {
		t.Fatalf("unexpected feeds: %+v", list)
	}

	var latest struct {
		Feed struct {
			Date    string `json:"date"`
			Content string `json:"content"`
		} `json:"feed"`
		State json.RawMessage `json:"state"`
	}
	if code := getJSON(t, ts.URL+"/api/v1/feeds/latest", &latest); code != 200 {
		t.Fatalf("latest feed: %d", code)
	}
	if latest.Feed.Date != "2026-07-25" || latest.Feed.Content != "## 要対応\n\nなし\n" || len(latest.State) == 0 {
		t.Fatalf("unexpected latest: %+v", latest)
	}

	body := bytes.NewBufferString(`{"key":"https://example.com/thread/1","title":"確認事項","version":"v1","acknowledged":true}`)
	req, err := http.NewRequest(http.MethodPut, ts.URL+"/api/v1/feeds/acknowledgements", body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("acknowledge: %d", resp.StatusCode)
	}

	var acknowledged struct {
		Items []struct {
			Key string `json:"key"`
		} `json:"items"`
	}
	if code := getJSON(t, ts.URL+"/api/v1/feeds/acknowledgements", &acknowledged); code != http.StatusOK {
		t.Fatalf("acknowledgements: %d", code)
	}
	if len(acknowledged.Items) != 1 || acknowledged.Items[0].Key != "https://example.com/thread/1" {
		t.Fatalf("unexpected acknowledgements: %+v", acknowledged.Items)
	}
}

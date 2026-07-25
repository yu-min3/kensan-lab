package feeds

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yu-min3/kensan-lab/apps/kensan/backend/internal/workspace"
)

func writeFixture(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestListAndLoadLatest(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "feeds/2026/07/24.md", feedFixture("2026-07-24"))
	writeFixture(t, root, "feeds/2026/07/25.md", feedFixture("2026-07-25"))
	writeFixture(t, root, "feeds/not-a-feed.md", feedFixture("2026-07-26"))
	writeFixture(t, root, "feeds/state/import.json", `{
		"schemaVersion":1,
		"reportDate":"2026-07-25",
		"status":"success",
		"lastAttemptAt":"2026-07-25T07:15:00+09:00",
		"lastSuccessAt":"2026-07-25T07:15:00+09:00"
	}`)
	ws := workspace.New(root)
	entries, err := List(ws)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Date != "2026-07-25" {
		t.Fatalf("unexpected entries: %+v", entries)
	}
	latest, err := LoadLatest(ws, time.Date(2026, 7, 25, 12, 0, 0, 0, time.FixedZone("JST", 9*60*60)))
	if err != nil {
		t.Fatal(err)
	}
	if latest.Feed == nil || latest.Feed.Date != "2026-07-25" || latest.Feed.GeneratedAt != "2026-07-25T06:30:00+09:00" {
		t.Fatalf("unexpected feed: %+v", latest.Feed)
	}
	if latest.Stale || latest.State == nil || latest.State.Status != "success" {
		t.Fatalf("unexpected latest: %+v", latest)
	}
	if latest.Feed.Content != "## 要対応\n\nなし\n" {
		t.Fatalf("frontmatter was not removed: %q", latest.Feed.Content)
	}
}

func TestLoadLatestWithoutFeedReturnsState(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "feeds/state/import.json", `{
		"schemaVersion":1,
		"reportDate":"2026-07-25",
		"status":"failed",
		"lastAttemptAt":"2026-07-25T07:15:00+09:00",
		"errorCode":"report_not_found",
		"errorMessage":"report not found"
	}`)
	latest, err := LoadLatest(workspace.New(root), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if latest.Feed != nil || latest.State == nil || latest.State.ErrorCode != "report_not_found" {
		t.Fatalf("unexpected latest: %+v", latest)
	}
}

func TestLoadLatestKeepsFeedWhenStateIsInvalid(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "feeds/2026/07/25.md", feedFixture("2026-07-25"))
	writeFixture(t, root, "feeds/state/import.json", `{invalid`)
	latest, err := LoadLatest(workspace.New(root), time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if latest.Feed == nil || latest.State != nil || latest.StateError == "" || !latest.Stale {
		t.Fatalf("unexpected latest: %+v", latest)
	}
}

func feedFixture(date string) string {
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

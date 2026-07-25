package briefing

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeInbox struct {
	report Report
	err    error
	calls  int
}

func (f *fakeInbox) Fetch(context.Context, time.Time) (Report, error) {
	f.calls++
	return f.report, f.err
}

func TestImporterWritesWorkspaceDocumentAndIsIdempotent(t *testing.T) {
	root := t.TempDir()
	date := time.Date(2026, 7, 25, 0, 0, 0, 0, time.FixedZone("JST", 9*60*60))
	modified := time.Date(2026, 7, 25, 6, 31, 0, 627000000, date.Location())
	inbox := &fakeInbox{report: Report{
		ExternalID: "drive-123",
		Name:       "2026-07-25.md",
		MIMEType:   "text/markdown",
		ModifiedAt: modified,
		Content:    validReport("2026-07-25"),
	}}
	importer := Importer{
		Root:  root,
		Inbox: inbox,
		Now: func() time.Time {
			return time.Date(2026, 7, 25, 7, 15, 0, 0, date.Location())
		},
	}

	first, err := importer.Import(context.Background(), date)
	if err != nil {
		t.Fatal(err)
	}
	if first.NoOp {
		t.Fatal("first import must write")
	}
	content, err := os.ReadFile(first.Path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	for _, expected := range []string{
		"type: note",
		"tags: [feed, daily-briefing]",
		"title: Daily Briefing 2026-07-25",
		"source_file_id: drive-123",
		`source_modified_at: "2026-07-25T06:31:00.627+09:00"`,
		"## 今日のニュース",
	} {
		if !strings.Contains(text, expected) {
			t.Errorf("missing %q in:\n%s", expected, text)
		}
	}

	second, err := importer.Import(context.Background(), date)
	if err != nil {
		t.Fatal(err)
	}
	if !second.NoOp {
		t.Fatal("second import must be a no-op")
	}

	stateContent, err := os.ReadFile(filepath.Join(root, "feeds", "state", "import.json"))
	if err != nil {
		t.Fatal(err)
	}
	var state ImportState
	if err := json.Unmarshal(stateContent, &state); err != nil {
		t.Fatal(err)
	}
	if state.Status != "success" || state.SourceFileID != "drive-123" {
		t.Fatalf("unexpected state: %+v", state)
	}
}

func TestImporterFailurePreservesExistingReportAndSuccessState(t *testing.T) {
	root := t.TempDir()
	date := time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC)
	reportPath := filepath.Join(root, "feeds", "2026", "07", "25.md")
	if err := os.MkdirAll(filepath.Dir(reportPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(reportPath, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}
	success := ImportState{
		SchemaVersion: 1,
		ReportDate:    "2026-07-24",
		Status:        "success",
		LastSuccessAt: "2026-07-24T07:15:00Z",
		SourceFileID:  "previous",
	}
	stateBytes, _ := json.Marshal(success)
	statePath := filepath.Join(root, "feeds", "state", "import.json")
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(statePath, stateBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	importer := Importer{
		Root:  root,
		Inbox: &fakeInbox{err: ErrReportNotFound},
		Now:   func() time.Time { return time.Date(2026, 7, 25, 7, 15, 0, 0, time.UTC) },
	}
	if _, err := importer.Import(context.Background(), date); !errors.Is(err, ErrReportNotFound) {
		t.Fatalf("expected report not found, got %v", err)
	}
	content, err := os.ReadFile(reportPath)
	if err != nil || string(content) != "existing" {
		t.Fatalf("existing report changed: %q err=%v", content, err)
	}
	failed, err := readState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if failed.Status != "failed" || failed.ErrorCode != "report_not_found" || failed.SourceFileID != "previous" || failed.LastSuccessAt == "" {
		t.Fatalf("unexpected failure state: %+v", failed)
	}
}

func TestImporterRejectsFilenameAndMIMEType(t *testing.T) {
	date := time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC)
	tests := []Report{
		{ExternalID: "id", ModifiedAt: date, Name: "wrong.md", MIMEType: "text/markdown", Content: validReport("2026-07-25")},
		{ExternalID: "id", ModifiedAt: date, Name: "2026-07-25.md", MIMEType: "application/pdf", Content: validReport("2026-07-25")},
	}
	for _, report := range tests {
		importer := Importer{Root: t.TempDir(), Inbox: &fakeInbox{report: report}}
		if _, err := importer.Import(context.Background(), date); err == nil {
			t.Fatalf("expected error for %+v", report)
		}
	}
}

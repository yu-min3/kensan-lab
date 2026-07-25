package drive

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yu-min3/kensan-lab/apps/feed/internal/briefing"
	gdrive "google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

func TestFetchSelectsLatestAllowedFileAndDownloadsIt(t *testing.T) {
	var queries []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/files":
			queries = append(queries, r.URL.Query().Get("q"))
			if r.URL.Query().Get("pageToken") == "" {
				fmt.Fprint(w, `{
					"nextPageToken":"next",
					"files":[
						{"id":"old","name":"2026-07-25.md","mimeType":"text/markdown","modifiedTime":"2026-07-25T06:00:00+09:00"},
						{"id":"ignored","name":"2026-07-25.md","mimeType":"application/pdf","modifiedTime":"2026-07-25T07:00:00+09:00"}
					]
				}`)
				return
			}
			fmt.Fprint(w, `{
				"files":[
					{"id":"latest","name":"2026-07-25.md","mimeType":"text/plain","modifiedTime":"2026-07-25T06:30:00+09:00"}
				]
			}`)
		case r.URL.Path == "/files/latest" && r.URL.Query().Get("alt") == "media":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write(validReport())
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	service, err := gdrive.NewService(
		context.Background(),
		option.WithEndpoint(server.URL+"/"),
		option.WithoutAuthentication(),
		option.WithHTTPClient(server.Client()),
	)
	if err != nil {
		t.Fatal(err)
	}
	inbox, err := NewWithService(service, "output-folder")
	if err != nil {
		t.Fatal(err)
	}
	report, err := inbox.Fetch(context.Background(), time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if report.ExternalID != "latest" || report.MIMEType != "text/plain" || !strings.Contains(string(report.Content), "## 要対応") {
		t.Fatalf("unexpected report: %+v", report)
	}
	if len(queries) != 2 || !strings.Contains(queries[0], "'output-folder' in parents") || !strings.Contains(queries[0], "trashed = false") {
		t.Fatalf("unexpected queries: %#v", queries)
	}
}

func TestSelectLatestRejectsTie(t *testing.T) {
	files := []*gdrive.File{
		{Id: "a", ModifiedTime: "2026-07-25T06:30:00+09:00"},
		{Id: "b", ModifiedTime: "2026-07-25T06:30:00+09:00"},
	}
	if _, _, err := selectLatest(files); err != briefing.ErrReportAmbiguous {
		t.Fatalf("expected ambiguous report, got %v", err)
	}
}

func TestFetchReturnsNotFoundWhenNoAllowedFileExists(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"files":[{"id":"pdf","name":"2026-07-25.md","mimeType":"application/pdf","modifiedTime":"2026-07-25T06:30:00Z"}]}`)
	}))
	defer server.Close()
	service, err := gdrive.NewService(
		context.Background(),
		option.WithEndpoint(server.URL+"/"),
		option.WithoutAuthentication(),
		option.WithHTTPClient(server.Client()),
	)
	if err != nil {
		t.Fatal(err)
	}
	inbox, _ := NewWithService(service, "output-folder")
	if _, err := inbox.Fetch(context.Background(), time.Now()); err != briefing.ErrReportNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}

func validReport() []byte {
	return []byte(`---
schema_version: 1
date: 2026-07-25
generated_at: 2026-07-25T06:30:00+09:00
generator: claude-scheduled-task
---

## 要対応

なし

## 今日のニュース

なし

## リリース・定点観測

なし
`)
}

package briefing

import (
	"strings"
	"testing"
	"time"
)

func validReport(date string) []byte {
	return []byte(`---
schema_version: 1
date: ` + date + `
generated_at: ` + date + `T06:30:00+09:00
generator: claude-scheduled-task
---

## 要対応

なし

## 今日のニュース

### Go release

- 出典: https://go.dev/blog/example

## リリース・定点観測

なし
`)
}

func TestValidateAcceptsReport(t *testing.T) {
	date := time.Date(2026, 7, 25, 0, 0, 0, 0, time.FixedZone("JST", 9*60*60))
	document, err := Validate(validReport("2026-07-25"), date)
	if err != nil {
		t.Fatal(err)
	}
	if document.Metadata.Date != "2026-07-25" || !strings.Contains(string(document.Body), "Go release") {
		t.Fatalf("unexpected document: %+v", document)
	}
}

func TestValidateRejectsUnsafeAndMalformedReports(t *testing.T) {
	tests := map[string]func([]byte) []byte{
		"raw HTML": func(input []byte) []byte {
			return append(input, []byte("\n<script>alert(1)</script>\n")...)
		},
		"unsafe URL": func(input []byte) []byte {
			return []byte(strings.Replace(string(input), "https://go.dev/blog/example", "javascript:alert(1)", 1))
		},
		"duplicate heading": func(input []byte) []byte {
			return append(input, []byte("\n## 要対応\n\n重複\n")...)
		},
		"unknown frontmatter": func(input []byte) []byte {
			return []byte(strings.Replace(string(input), "generator:", "unknown: true\ngenerator:", 1))
		},
		"wrong date": func(input []byte) []byte {
			return []byte(strings.Replace(string(input), "date: 2026-07-25", "date: 2026-07-24", 1))
		},
	}
	expected := time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC)
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := Validate(mutate(validReport("2026-07-25")), expected); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestValidateRejectsOversizeAndInvalidUTF8(t *testing.T) {
	if _, err := Validate(make([]byte, MaxReportSize+1), time.Time{}); err == nil {
		t.Fatal("expected size error")
	}
	if _, err := Validate([]byte{0xff}, time.Time{}); err == nil {
		t.Fatal("expected UTF-8 error")
	}
}

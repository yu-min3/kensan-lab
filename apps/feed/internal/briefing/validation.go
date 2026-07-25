package briefing

import (
	"bytes"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"gopkg.in/yaml.v3"
)

var requiredHeadings = map[string]bool{
	"要対応":       true,
	"今日のニュース":   true,
	"リリース・定点観測": true,
}

var dangerousMarkdownURL = regexp.MustCompile(`(?i)(?:^|[[:space:](<])(?:javascript|data|file):`)

func Validate(content []byte, expectedDate time.Time) (Document, error) {
	if len(content) > MaxReportSize {
		return Document{}, fmt.Errorf("invalid report: size %d exceeds %d", len(content), MaxReportSize)
	}
	if !utf8.Valid(content) {
		return Document{}, fmt.Errorf("invalid report: content is not UTF-8")
	}
	if bytes.IndexByte(content, 0) >= 0 {
		return Document{}, fmt.Errorf("invalid report: content contains NUL")
	}

	metaBytes, body, err := splitFrontmatter(content)
	if err != nil {
		return Document{}, err
	}
	var metadata ExternalMetadata
	decoder := yaml.NewDecoder(bytes.NewReader(metaBytes))
	decoder.KnownFields(true)
	if err := decoder.Decode(&metadata); err != nil {
		return Document{}, fmt.Errorf("invalid report frontmatter: %w", err)
	}
	if metadata.SchemaVersion != SchemaVersion {
		return Document{}, fmt.Errorf("invalid report: unsupported schema_version %d", metadata.SchemaVersion)
	}
	if metadata.Generator != "claude-scheduled-task" {
		return Document{}, fmt.Errorf("invalid report: unsupported generator %q", metadata.Generator)
	}
	reportDate, err := time.Parse("2006-01-02", metadata.Date)
	if err != nil {
		return Document{}, fmt.Errorf("invalid report date: %w", err)
	}
	if metadata.GeneratedAt.IsZero() {
		return Document{}, fmt.Errorf("invalid report: generated_at is required")
	}
	jst := time.FixedZone("Asia/Tokyo", 9*60*60)
	if metadata.GeneratedAt.In(jst).Format("2006-01-02") != metadata.Date {
		return Document{}, fmt.Errorf("invalid report: generated_at date does not match report date in Asia/Tokyo")
	}
	if !expectedDate.IsZero() && reportDate.Format("2006-01-02") != expectedDate.Format("2006-01-02") {
		return Document{}, fmt.Errorf("invalid report: date %s does not match expected %s", metadata.Date, expectedDate.Format("2006-01-02"))
	}
	if err := validateMarkdown(body); err != nil {
		return Document{}, err
	}
	return Document{Metadata: metadata, Body: body}, nil
}

func splitFrontmatter(content []byte) ([]byte, []byte, error) {
	normalized := bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))
	if !bytes.HasPrefix(normalized, []byte("---\n")) {
		return nil, nil, fmt.Errorf("invalid report: frontmatter must start at byte 0")
	}
	end := bytes.Index(normalized[4:], []byte("\n---\n"))
	if end < 0 {
		return nil, nil, fmt.Errorf("invalid report: frontmatter is not closed")
	}
	end += 4
	meta := normalized[4:end]
	body := normalized[end+5:]
	if bytes.Contains(body, []byte("\n---\n")) && bytes.HasPrefix(bytes.TrimSpace(body), []byte("---")) {
		return nil, nil, fmt.Errorf("invalid report: multiple frontmatter blocks")
	}
	return meta, body, nil
}

func validateMarkdown(source []byte) error {
	if dangerousMarkdownURL.Match(source) {
		return fmt.Errorf("invalid report: dangerous URL scheme is not allowed")
	}
	md := goldmark.New()
	doc := md.Parser().Parse(text.NewReader(source))
	headings := map[string]int{}
	err := ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := node.(type) {
		case *ast.RawHTML, *ast.HTMLBlock:
			return ast.WalkStop, fmt.Errorf("invalid report: raw HTML is not allowed")
		case *ast.Heading:
			if n.Level == 2 {
				headings[strings.TrimSpace(string(n.Text(source)))]++
			}
		case *ast.Link:
			if err := validateURL(n.Destination); err != nil {
				return ast.WalkStop, err
			}
		case *ast.AutoLink:
			if err := validateURL(n.URL(source)); err != nil {
				return ast.WalkStop, err
			}
		}
		return ast.WalkContinue, nil
	})
	if err != nil {
		return err
	}
	for heading := range requiredHeadings {
		if headings[heading] != 1 {
			return fmt.Errorf("invalid report: heading %q must appear exactly once", heading)
		}
	}
	return nil
}

func validateURL(raw []byte) error {
	parsed, err := url.Parse(strings.TrimSpace(string(raw)))
	if err != nil {
		return fmt.Errorf("invalid report URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("invalid report: URL scheme %q is not allowed", parsed.Scheme)
	}
	return nil
}

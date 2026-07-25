package briefing

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

var (
	ErrReportNotFound  = errors.New("report not found")
	ErrReportAmbiguous = errors.New("report is ambiguous")
)

type ReportInbox interface {
	Fetch(context.Context, time.Time) (Report, error)
}

type Importer struct {
	Root  string
	Inbox ReportInbox
	Now   func() time.Time
}

func (i Importer) Import(ctx context.Context, date time.Time) (result ImportResult, err error) {
	now := time.Now
	if i.Now != nil {
		now = i.Now
	}
	attemptedAt := now()
	report, err := i.Inbox.Fetch(ctx, date)
	if err != nil {
		_ = i.writeFailureState(date, attemptedAt, classifyError(err), err)
		return ImportResult{}, err
	}
	if report.ExternalID == "" || report.ModifiedAt.IsZero() {
		err = fmt.Errorf("invalid report: source identity and modified time are required")
		_ = i.writeFailureState(date, attemptedAt, "invalid_report", err)
		return ImportResult{}, err
	}
	if report.Name != date.Format("2006-01-02")+".md" {
		err = fmt.Errorf("invalid report: filename %q does not match date", report.Name)
		_ = i.writeFailureState(date, attemptedAt, "invalid_report", err)
		return ImportResult{}, err
	}
	if report.MIMEType != "text/markdown" && report.MIMEType != "text/plain" {
		err = fmt.Errorf("invalid report: MIME type %q is not allowed", report.MIMEType)
		_ = i.writeFailureState(date, attemptedAt, "invalid_report", err)
		return ImportResult{}, err
	}
	document, err := Validate(report.Content, date)
	if err != nil {
		_ = i.writeFailureState(date, attemptedAt, "invalid_report", err)
		return ImportResult{}, err
	}

	path := filepath.Join(i.Root, "feeds", date.Format("2006"), date.Format("01"), date.Format("02")+".md")
	existing, readErr := os.ReadFile(path)
	if readErr == nil && sameSource(existing, report) {
		if err := i.writeSuccessState(date, attemptedAt, report); err != nil {
			return ImportResult{}, err
		}
		return ImportResult{Path: path, NoOp: true}, nil
	}
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return ImportResult{}, fmt.Errorf("read existing report: %w", readErr)
	}

	output, err := workspaceDocument(document, report)
	if err != nil {
		return ImportResult{}, err
	}
	if err := atomicWrite(path, output, 0o644); err != nil {
		_ = i.writeFailureState(date, attemptedAt, "workspace_write_failed", err)
		return ImportResult{}, fmt.Errorf("write report: %w", err)
	}
	if err := i.writeSuccessState(date, attemptedAt, report); err != nil {
		return ImportResult{}, err
	}
	return ImportResult{Path: path}, nil
}

type workspaceMetadata struct {
	Type             string    `yaml:"type"`
	Tags             []string  `yaml:"tags,flow"`
	Title            string    `yaml:"title"`
	Status           string    `yaml:"status"`
	Created          string    `yaml:"created"`
	Updated          string    `yaml:"updated"`
	SchemaVersion    int       `yaml:"schema_version"`
	ReportDate       string    `yaml:"report_date"`
	GeneratedAt      time.Time `yaml:"generated_at"`
	Generator        string    `yaml:"generator"`
	SourceFileID     string    `yaml:"source_file_id"`
	SourceModifiedAt string    `yaml:"source_modified_at"`
}

func workspaceDocument(document Document, report Report) ([]byte, error) {
	date := document.Metadata.Date
	metadata := workspaceMetadata{
		Type:             "note",
		Tags:             []string{"feed", "daily-briefing"},
		Title:            "Daily Briefing " + date,
		Status:           "active",
		Created:          date,
		Updated:          date,
		SchemaVersion:    document.Metadata.SchemaVersion,
		ReportDate:       date,
		GeneratedAt:      document.Metadata.GeneratedAt,
		Generator:        document.Metadata.Generator,
		SourceFileID:     report.ExternalID,
		SourceModifiedAt: report.ModifiedAt.Format(time.RFC3339Nano),
	}
	frontmatter, err := yaml.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal workspace frontmatter: %w", err)
	}
	var output bytes.Buffer
	output.WriteString("---\n")
	output.Write(frontmatter)
	output.WriteString("---\n\n")
	output.Write(bytes.TrimLeft(document.Body, "\n"))
	return output.Bytes(), nil
}

func sameSource(content []byte, report Report) bool {
	metadataBytes, _, err := splitFrontmatter(content)
	if err != nil {
		return false
	}
	var metadata workspaceMetadata
	if err := yaml.Unmarshal(metadataBytes, &metadata); err != nil {
		return false
	}
	return metadata.SourceFileID == report.ExternalID &&
		metadata.SourceModifiedAt == report.ModifiedAt.Format(time.RFC3339Nano)
}

func atomicWrite(path string, content []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(dir, ".feed-*.tmp")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err := temp.Chmod(mode); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(content); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempName, path)
}

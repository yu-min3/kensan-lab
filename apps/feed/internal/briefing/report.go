package briefing

import "time"

const (
	SchemaVersion = 1
	MaxReportSize = 1 << 20
)

type ExternalMetadata struct {
	SchemaVersion int       `yaml:"schema_version"`
	Date          string    `yaml:"date"`
	GeneratedAt   time.Time `yaml:"generated_at"`
	Generator     string    `yaml:"generator"`
}

type Document struct {
	Metadata ExternalMetadata
	Body     []byte
}

type Report struct {
	ExternalID string
	Name       string
	MIMEType   string
	ModifiedAt time.Time
	Content    []byte
}

type ImportResult struct {
	Path string
	NoOp bool
}

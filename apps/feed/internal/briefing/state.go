package briefing

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type ImportState struct {
	SchemaVersion    int    `json:"schemaVersion"`
	ReportDate       string `json:"reportDate"`
	Status           string `json:"status"`
	LastAttemptAt    string `json:"lastAttemptAt"`
	LastSuccessAt    string `json:"lastSuccessAt,omitempty"`
	SourceFileID     string `json:"sourceFileId,omitempty"`
	SourceModifiedAt string `json:"sourceModifiedAt,omitempty"`
	ErrorCode        string `json:"errorCode"`
	ErrorMessage     string `json:"errorMessage"`
}

func (i Importer) statePath() string {
	return filepath.Join(i.Root, "feeds", "state", "import.json")
}

func (i Importer) writeSuccessState(date, attemptedAt time.Time, report Report) error {
	state := ImportState{
		SchemaVersion:    1,
		ReportDate:       date.Format("2006-01-02"),
		Status:           "success",
		LastAttemptAt:    attemptedAt.Format(time.RFC3339),
		LastSuccessAt:    attemptedAt.Format(time.RFC3339),
		SourceFileID:     report.ExternalID,
		SourceModifiedAt: report.ModifiedAt.Format(time.RFC3339Nano),
	}
	return i.writeState(state)
}

func (i Importer) writeFailureState(date, attemptedAt time.Time, code string, failure error) error {
	state := ImportState{
		SchemaVersion: 1,
		ReportDate:    date.Format("2006-01-02"),
		Status:        "failed",
		LastAttemptAt: attemptedAt.Format(time.RFC3339),
		ErrorCode:     code,
		ErrorMessage:  safeError(failure),
	}
	previous, err := readState(i.statePath())
	if err == nil {
		state.LastSuccessAt = previous.LastSuccessAt
		state.SourceFileID = previous.SourceFileID
		state.SourceModifiedAt = previous.SourceModifiedAt
	}
	return i.writeState(state)
}

func (i Importer) writeState(state ImportState) error {
	content, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal import state: %w", err)
	}
	content = append(content, '\n')
	if err := atomicWrite(i.statePath(), content, 0o644); err != nil {
		return fmt.Errorf("write import state: %w", err)
	}
	return nil
}

func readState(path string) (ImportState, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return ImportState{}, err
	}
	var state ImportState
	if err := json.Unmarshal(content, &state); err != nil {
		return ImportState{}, err
	}
	return state, nil
}

func classifyError(err error) string {
	switch {
	case errors.Is(err, ErrReportNotFound):
		return "report_not_found"
	case errors.Is(err, ErrReportAmbiguous):
		return "report_ambiguous"
	default:
		return "download_failed"
	}
}

func safeError(err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	if len(message) > 300 {
		message = message[:300]
	}
	return message
}

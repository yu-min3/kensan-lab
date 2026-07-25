// Package feeds はworkspaceに保存されたPersonal Daily Briefingと同期状態を読み取る。
package feeds

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"time"

	"github.com/yu-min3/kensan-lab/apps/kensan/backend/internal/workspace"
)

var feedPath = regexp.MustCompile(`^feeds/(\d{4})/(\d{2})/(\d{2})\.md$`)

type Entry struct {
	Date  string `json:"date"`
	Title string `json:"title"`
	Path  string `json:"path"`
}

type Feed struct {
	Entry
	GeneratedAt string `json:"generatedAt,omitempty"`
	Content     string `json:"content"`
}

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

type Latest struct {
	Feed       *Feed        `json:"feed"`
	State      *ImportState `json:"state"`
	StateError string       `json:"stateError,omitempty"`
	Stale      bool         `json:"stale"`
}

func List(ws *workspace.Workspace) ([]Entry, error) {
	docs, err := ws.Scan()
	entries := make([]Entry, 0)
	for _, doc := range docs {
		match := feedPath.FindStringSubmatch(doc.Path)
		if match == nil {
			continue
		}
		date := match[1] + "-" + match[2] + "-" + match[3]
		if _, parseErr := time.Parse("2006-01-02", date); parseErr != nil {
			continue
		}
		title := doc.Meta.Title
		if title == "" {
			title = "Daily Briefing " + date
		}
		entries = append(entries, Entry{Date: date, Title: title, Path: doc.Path})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Date > entries[j].Date })
	return entries, err
}

func LoadLatest(ws *workspace.Workspace, now time.Time) (Latest, error) {
	entries, scanErr := List(ws)
	state, stateErr := loadState(ws)
	if stateErr != nil && !errors.Is(stateErr, os.ErrNotExist) {
		state = nil
	}
	result := Latest{State: state}
	if stateErr != nil && !errors.Is(stateErr, os.ErrNotExist) {
		result.StateError = stateErr.Error()
	}
	if len(entries) == 0 {
		return result, scanErr
	}
	entry := entries[0]
	doc, content, err := ws.Read(entry.Path)
	if err != nil {
		return Latest{}, err
	}
	result.Feed = &Feed{
		Entry:       entry,
		GeneratedAt: extraString(doc.Meta.Extra, "generated_at"),
		Content:     stripFrontmatter(content),
	}
	today := now.In(time.FixedZone("Asia/Tokyo", 9*60*60)).Format("2006-01-02")
	result.Stale = entry.Date != today || state == nil || state.Status != "success" || state.ReportDate != today
	return result, scanErr
}

func loadState(ws *workspace.Workspace) (*ImportState, error) {
	path, err := ws.Abs("feeds/state/import.json")
	if err != nil {
		return nil, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var state ImportState
	if err := json.Unmarshal(content, &state); err != nil {
		return nil, fmt.Errorf("parse feed import state: %w", err)
	}
	if state.SchemaVersion != 1 {
		return nil, fmt.Errorf("unsupported feed import state version: %d", state.SchemaVersion)
	}
	return &state, nil
}

func stripFrontmatter(content []byte) string {
	normalized := bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))
	if !bytes.HasPrefix(normalized, []byte("---\n")) {
		return string(content)
	}
	if end := bytes.Index(normalized[4:], []byte("\n---\n")); end >= 0 {
		return string(bytes.TrimLeft(normalized[end+9:], "\n"))
	}
	return string(content)
}

func extraString(extra map[string]any, key string) string {
	if extra == nil {
		return ""
	}
	switch value := extra[key].(type) {
	case string:
		return value
	case time.Time:
		return value.Format(time.RFC3339)
	default:
		return ""
	}
}

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
	"strings"
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

const acknowledgementsPath = "feeds/state/acknowledged.json"

const (
	acknowledgementRetention = 90 * 24 * time.Hour
	maxAcknowledgements      = 2000
	maxAcknowledgementBytes  = 2 * 1024 * 1024
)

type Acknowledgement struct {
	Key            string `json:"key"`
	Title          string `json:"title"`
	Version        string `json:"version"`
	AcknowledgedAt string `json:"acknowledgedAt"`
	ExpiresAt      string `json:"expiresAt"`
}

type acknowledgementState struct {
	SchemaVersion int                        `json:"schemaVersion"`
	Items         map[string]Acknowledgement `json:"items"`
}

func ListAcknowledgements(ws *workspace.Workspace, now time.Time) ([]Acknowledgement, error) {
	state, err := loadAcknowledgements(ws)
	if errors.Is(err, os.ErrNotExist) {
		return []Acknowledgement{}, nil
	}
	if err != nil {
		return nil, err
	}
	items := make([]Acknowledgement, 0, len(state.Items))
	for _, item := range state.Items {
		expiresAt, parseErr := time.Parse(time.RFC3339, item.ExpiresAt)
		if parseErr != nil || !expiresAt.After(now) {
			continue
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].AcknowledgedAt > items[j].AcknowledgedAt })
	return items, nil
}

func SetAcknowledgement(ws *workspace.Workspace, key, title, version string, acknowledged bool, now time.Time) error {
	key = strings.TrimSpace(key)
	title = strings.TrimSpace(title)
	version = strings.TrimSpace(version)
	if key == "" || len(key) > 2048 {
		return errors.New("acknowledgement key must be between 1 and 2048 bytes")
	}
	if len(title) > 300 {
		return errors.New("acknowledgement title must be at most 300 bytes")
	}
	if version == "" || len(version) > 256 {
		return errors.New("acknowledgement version must be between 1 and 256 bytes")
	}
	return ws.Mutate(acknowledgementsPath, func(content []byte, exists bool) ([]byte, error) {
		state := acknowledgementState{SchemaVersion: 1, Items: map[string]Acknowledgement{}}
		if exists {
			if len(content) > maxAcknowledgementBytes {
				return nil, errors.New("feed acknowledgement state exceeds 2 MiB")
			}
			if err := json.Unmarshal(content, &state); err != nil {
				return nil, fmt.Errorf("parse feed acknowledgements: %w", err)
			}
			if state.SchemaVersion != 1 {
				return nil, fmt.Errorf("unsupported feed acknowledgement version: %d", state.SchemaVersion)
			}
			if state.Items == nil {
				state.Items = map[string]Acknowledgement{}
			}
		}
		pruneAcknowledgements(state.Items, now)
		if acknowledged {
			state.Items[key] = Acknowledgement{
				Key:            key,
				Title:          title,
				Version:        version,
				AcknowledgedAt: now.Format(time.RFC3339),
				ExpiresAt:      now.Add(acknowledgementRetention).Format(time.RFC3339),
			}
		} else {
			delete(state.Items, key)
		}
		trimAcknowledgements(state.Items, maxAcknowledgements)
		out, err := json.MarshalIndent(state, "", "  ")
		if err != nil {
			return nil, err
		}
		if len(out) > maxAcknowledgementBytes {
			return nil, errors.New("feed acknowledgement state exceeds 2 MiB")
		}
		return append(out, '\n'), nil
	})
}

func pruneAcknowledgements(items map[string]Acknowledgement, now time.Time) {
	for key, item := range items {
		expiresAt, err := time.Parse(time.RFC3339, item.ExpiresAt)
		if err != nil || !expiresAt.After(now) {
			delete(items, key)
		}
	}
}

func trimAcknowledgements(items map[string]Acknowledgement, limit int) {
	if len(items) <= limit {
		return
	}
	sorted := make([]Acknowledgement, 0, len(items))
	for _, item := range items {
		sorted = append(sorted, item)
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].AcknowledgedAt > sorted[j].AcknowledgedAt })
	for _, item := range sorted[limit:] {
		delete(items, item.Key)
	}
}

func loadAcknowledgements(ws *workspace.Workspace) (*acknowledgementState, error) {
	path, err := ws.Abs(acknowledgementsPath)
	if err != nil {
		return nil, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(content) > maxAcknowledgementBytes {
		return nil, errors.New("feed acknowledgement state exceeds 2 MiB")
	}
	var state acknowledgementState
	if err := json.Unmarshal(content, &state); err != nil {
		return nil, fmt.Errorf("parse feed acknowledgements: %w", err)
	}
	if state.SchemaVersion != 1 {
		return nil, fmt.Errorf("unsupported feed acknowledgement version: %d", state.SchemaVersion)
	}
	if state.Items == nil {
		state.Items = map[string]Acknowledgement{}
	}
	return &state, nil
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

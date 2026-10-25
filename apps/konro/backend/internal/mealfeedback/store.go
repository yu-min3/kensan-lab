package mealfeedback

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var ErrConflict = errors.New("記録が更新されています。最新版を確認してください")
var ErrInvalid = errors.New("記録の内容を確認してください")

type Event struct {
	ID           string `json:"id"`
	Revision     int    `json:"revision"`
	ProposalID   string `json:"proposalId,omitempty"`
	PlanID       string `json:"planId,omitempty"`
	PlanRevision int    `json:"planRevision,omitempty"`
	File         string `json:"file,omitempty"`
	Version      string `json:"version,omitempty"`
	Type         string `json:"type"`
	Reason       string `json:"reason,omitempty"`
	OccurredAt   string `json:"occurredAt,omitempty"`
	Cancelled    bool   `json:"cancelled"`
	RequestHash  string `json:"requestHash,omitempty"`
}
type CookedRecipe struct {
	Title   string `json:"title,omitempty"`
	File    string `json:"file"`
	Version string `json:"version"`
	Scale   string `json:"scale"`
	Made    bool   `json:"made"`
}
type CookRecord struct {
	ID           string         `json:"id"`
	Revision     int            `json:"revision"`
	Date         string         `json:"date"`
	PlanID       string         `json:"planId,omitempty"`
	PlanRevision int            `json:"planRevision,omitempty"`
	ProposalID   string         `json:"proposalId,omitempty"`
	Recipes      []CookedRecipe `json:"recipes"`
	Satisfaction int            `json:"satisfaction"`
	Amount       string         `json:"amount"`
	Notes        string         `json:"notes"`
	Cancelled    bool           `json:"cancelled"`
	RequestHash  string         `json:"requestHash,omitempty"`
}
type Preferences struct {
	Revision  int      `json:"revision"`
	Avoid     []string `json:"avoid"`
	Use       []string `json:"use"`
	Notes     string   `json:"notes"`
	Suggested []string `json:"suggested"`
}
type Store struct {
	mu   sync.Mutex
	root string
}

func New(root string) *Store { return &Store{root: root} }

var idPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,128}$`)

func digest(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func (s *Store) read(folder, id string, v any) error {
	if !idPattern.MatchString(id) {
		return ErrInvalid
	}
	dir := filepath.Join(s.root, folder)
	if info, e := os.Lstat(dir); e == nil && (!info.IsDir() || info.Mode()&os.ModeSymlink != 0) {
		return ErrInvalid
	}
	path := filepath.Join(dir, id+".json")
	info, e := os.Lstat(path)
	if e != nil {
		return e
	}
	if !info.Mode().IsRegular() || info.Size() > 2<<20 {
		return ErrInvalid
	}
	b, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	return json.Unmarshal(b, v)
}
func (s *Store) write(folder, id string, v any) error {
	if !idPattern.MatchString(id) {
		return ErrInvalid
	}
	dir := filepath.Join(s.root, folder)
	if e := os.MkdirAll(dir, 0700); e != nil {
		return e
	}
	info, e := os.Lstat(dir)
	if e != nil {
		return e
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return ErrInvalid
	}
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	if len(b) > 2<<20 {
		return ErrInvalid
	}
	f, e := os.CreateTemp(dir, ".record-")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if e = f.Chmod(0600); e == nil {
		_, e = f.Write(b)
	}
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e == nil {
		e = ce
	}
	if e != nil {
		return e
	}
	if e = os.Rename(tmp, filepath.Join(dir, id+".json")); e != nil {
		return e
	}
	d, e := os.Open(dir)
	if e != nil {
		return e
	}
	e = d.Sync()
	d.Close()
	return e
}
func (s *Store) Record(in Event) (Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !idPattern.MatchString(in.ID) || in.Type != "adopt" && in.Type != "swap" && in.Type != "fixed" && in.Type != "evaluation" && in.Type != "cooked" || len(in.Reason) > 1000 || len(in.File) > 2048 {
		return Event{}, ErrInvalid
	}
	hash := digest(in)
	var old Event
	e := s.read("feedback", in.ID, &old)
	if e == nil {
		if old.RequestHash == hash {
			return old, nil
		}
		if in.Revision != old.Revision {
			return old, ErrConflict
		}
	} else if !errors.Is(e, os.ErrNotExist) {
		return old, e
	} else if in.Revision != 0 {
		return old, ErrConflict
	}
	in.Revision = old.Revision + 1
	in.RequestHash = hash
	if in.OccurredAt == "" {
		in.OccurredAt = time.Now().UTC().Format(time.RFC3339)
	} else if _, e = time.Parse(time.RFC3339, in.OccurredAt); e != nil {
		return old, ErrInvalid
	}
	return in, s.write("feedback", in.ID, in)
}
func (s *Store) Events() ([]Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, e := os.ReadDir(filepath.Join(s.root, "feedback"))
	if errors.Is(e, os.ErrNotExist) {
		return []Event{}, nil
	}
	if e != nil {
		return nil, e
	}
	result := []Event{}
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		var v Event
		if e = s.read("feedback", strings.TrimSuffix(entry.Name(), ".json"), &v); e != nil {
			return nil, e
		}
		result = append(result, v)
	}
	return result, nil
}
func (s *Store) Cook(in CookRecord) (CookRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !idPattern.MatchString(in.ID) || len(in.Recipes) > 50 || len(in.Notes) > 4000 || in.Satisfaction < 0 || in.Satisfaction > 5 || in.Amount != "" && in.Amount != "too_much" && in.Amount != "just_right" && in.Amount != "too_little" {
		return CookRecord{}, ErrInvalid
	}
	if _, e := time.Parse("2006-01-02", in.Date); e != nil {
		return CookRecord{}, ErrInvalid
	}
	for _, r := range in.Recipes {
		if r.File == "" || r.Version == "" || len(r.File) > 2048 || len(r.Version) > 128 {
			return CookRecord{}, ErrInvalid
		}
	}
	hash := digest(in)
	var old CookRecord
	e := s.read("cook-history", in.ID, &old)
	if e == nil {
		if old.RequestHash == hash {
			return old, nil
		}
		if in.Revision != old.Revision {
			return old, ErrConflict
		}
	} else if !errors.Is(e, os.ErrNotExist) {
		return old, e
	} else if in.Revision != 0 {
		return old, ErrConflict
	}
	in.Revision = old.Revision + 1
	in.RequestHash = hash
	return in, s.write("cook-history", in.ID, in)
}
func (s *Store) GetCook(id string) (CookRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var r CookRecord
	e := s.read("cook-history", id, &r)
	return r, e
}
func (s *Store) History() ([]CookRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, e := os.ReadDir(filepath.Join(s.root, "cook-history"))
	if errors.Is(e, os.ErrNotExist) {
		return []CookRecord{}, nil
	}
	if e != nil {
		return nil, e
	}
	records := []CookRecord{}
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		var r CookRecord
		if e = s.read("cook-history", strings.TrimSuffix(entry.Name(), ".json"), &r); e != nil {
			return nil, e
		}
		records = append(records, r)
	}
	return records, nil
}
func (s *Store) preferences() (Preferences, error) {
	var p Preferences
	e := s.read(".settings", "meal-preferences", &p)
	if errors.Is(e, os.ErrNotExist) {
		return Preferences{Revision: 0, Avoid: []string{}, Use: []string{}, Suggested: []string{}}, nil
	}
	return p, e
}
func (s *Store) Preferences() (Preferences, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.preferences()
}
func (s *Store) SavePreferences(in Preferences) (Preferences, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	old, e := s.preferences()
	if e != nil {
		return old, e
	}
	if in.Revision != old.Revision {
		return old, ErrConflict
	}
	if len(in.Avoid) > 30 || len(in.Use) > 30 || len(in.Notes) > 4000 || len(in.Suggested) > 30 {
		return old, ErrInvalid
	}
	for _, v := range append(append([]string{}, in.Avoid...), in.Use...) {
		if strings.TrimSpace(v) == "" || len(v) > 200 {
			return old, ErrInvalid
		}
	}
	in.Revision++
	return in, s.write(".settings", "meal-preferences", in)
}

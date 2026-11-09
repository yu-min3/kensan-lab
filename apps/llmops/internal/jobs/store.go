// Package jobs owns durable admission and runtime-job correlation.
package jobs

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

var (
	ErrNotFound    = errors.New("job not found")
	ErrConflict    = errors.New("idempotency conflict")
	ErrQuota       = errors.New("daily admission limit")
	ErrInvalid     = errors.New("invalid admission or transition")
	ErrUnavailable = errors.New("job storage unavailable")
	ErrLocked      = errors.New("another writer holds the job store")
)

// Owner is resolved by the identity service, never from request JSON.
type Owner struct {
	App     string `json:"app"`
	Role    string `json:"role"`
	Subject string `json:"subject"`
}
type Admission struct {
	Owner         Owner           `json:"owner"`
	Key           string          `json:"key"`
	Kind          string          `json:"kind"`
	Payload       json.RawMessage `json:"payload"`
	RuntimeClient string          `json:"runtime_client"`
	RuntimeTarget string          `json:"runtime_target"`
}

// Record stores private input for recovery; it must not be returned verbatim
// from an HTTP API. Public views must omit the downstream credentials' IDs,
// payload, and internal phase unless explicitly authorized.
type Record struct {
	ID            string    `json:"id"`
	Admission     Admission `json:"admission"`
	RequestHash   string    `json:"request_hash"`
	DownstreamKey string    `json:"downstream_key"`
	RuntimeJobID  string    `json:"runtime_job_id,omitempty"`
	Phase         string    `json:"phase"` // prepared, uncertain, reconciling, linked, terminal
	State         string    `json:"state"`
	Reason        string    `json:"reason,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
type envelope struct {
	Version int      `json:"version"`
	Records []Record `json:"records"`
	SHA256  string   `json:"sha256"`
}
type Store struct {
	mu      sync.Mutex
	dir     string
	lock    *os.File
	records map[string]Record
	now     func() time.Time
	broken  bool
	// save is the atomic filesystem boundary, replaceable by package tests only.
	save func([]Record) error
}

const maxSnapshot = 64 << 20

// Open exclusively locks a private directory and validates its complete
// snapshot. No automatic corruption repair or retry of LLM work is performed.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create job directory: %w", err)
	}
	fi, err := os.Stat(dir)
	if err != nil || !fi.IsDir() || fi.Mode().Perm()&0077 != 0 {
		return nil, fmt.Errorf("%w: private directory required", ErrUnavailable)
	}
	fd, err := syscall.Open(filepath.Join(dir, "writer.lock"), syscall.O_CREAT|syscall.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, err
	}
	lock := os.NewFile(uintptr(fd), "job-store-lock")
	info, err := lock.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		lock.Close()
		return nil, ErrUnavailable
	}
	if err = syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		lock.Close()
		return nil, ErrLocked
	}
	s := &Store{dir: dir, lock: lock, records: make(map[string]Record), now: time.Now}
	s.save = s.atomicSave
	if err = s.load(); err != nil {
		s.Close()
		return nil, err
	}
	// A crash after the dispatch marker can mean the runtime accepted the job.
	// The caller must repeat Create with exactly the persisted key and input.
	changed := false
	for id, r := range s.records {
		if r.Phase == "uncertain" {
			r.Phase = "reconciling"
			s.records[id] = r
			changed = true
		}
	}
	if changed {
		if err = s.commit(s.records); err != nil {
			s.Close()
			return nil, err
		}
	}
	return s, nil
}
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lock == nil {
		return nil
	}
	err := s.lock.Close()
	s.lock = nil
	return err
}
func (s *Store) available() error {
	if s.lock == nil || s.broken {
		return ErrUnavailable
	}
	return nil
}
func validToken(s string) bool {
	if s == "" || len(s) > 128 {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.' || c == ':') {
			return false
		}
	}
	return true
}
func normalize(a Admission) (Admission, string, error) {
	if !validToken(a.Owner.App) || !validToken(a.Owner.Role) || !validToken(a.Owner.Subject) || !validToken(a.Key) || !validToken(a.RuntimeClient) || !validToken(a.RuntimeTarget) || (a.Kind != "online" && a.Kind != "eval") || len(a.Payload) == 0 || len(a.Payload) > 512<<10 {
		return a, "", ErrInvalid
	}
	var v any
	d := json.NewDecoder(bytes.NewReader(a.Payload))
	d.UseNumber()
	if d.Decode(&v) != nil || d.Decode(new(any)) != io.EOF {
		return a, "", ErrInvalid
	}
	p, err := json.Marshal(v)
	if err != nil {
		return a, "", ErrInvalid
	}
	a.Payload = p
	b, _ := json.Marshal(a)
	sum := sha256.Sum256(b)
	return a, hex.EncodeToString(sum[:]), nil
}
func copyRecord(r Record) Record {
	r.Admission.Payload = append(json.RawMessage(nil), r.Admission.Payload...)
	return r
}
func cloneRecords(records map[string]Record) map[string]Record {
	m := make(map[string]Record, len(records))
	for id, r := range records {
		m[id] = r
	}
	return m
}
func scopeEqual(a, b Admission) bool { return a.Owner == b.Owner && a.Key == b.Key }

// Admit atomically reserves one daily slot for app × kind and stores input.
// dailyLimit comes from operator policy. Zero denies fresh admissions; an
// identical replay returns its existing job even when the limit is exhausted.
func (s *Store) Admit(a Admission, dailyLimit int) (Record, bool, error) {
	a, digest, err := normalize(a)
	if err != nil || dailyLimit < 0 {
		return Record{}, false, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err = s.available(); err != nil {
		return Record{}, false, err
	}
	now := s.now().UTC()
	count := 0
	for _, r := range s.records {
		if scopeEqual(r.Admission, a) {
			if r.RequestHash != digest {
				return Record{}, false, ErrConflict
			}
			return copyRecord(r), true, nil
		}
		if r.Admission.Owner.App == a.Owner.App && r.Admission.Kind == a.Kind && r.CreatedAt.UTC().Format("2006-01-02") == now.Format("2006-01-02") {
			count++
		}
	}
	if count >= dailyLimit {
		return Record{}, false, ErrQuota
	}
	var random [16]byte
	if _, err = rand.Read(random[:]); err != nil {
		return Record{}, false, ErrUnavailable
	}
	id := hex.EncodeToString(random[:])
	r := Record{ID: id, Admission: a, RequestHash: digest, DownstreamKey: "llmops-" + id, Phase: "prepared", State: "queued", CreatedAt: now, UpdatedAt: now}
	next := cloneRecords(s.records)
	next[id] = r
	if err = s.commit(next); err != nil {
		return Record{}, false, err
	}
	return copyRecord(r), false, nil
}
func (s *Store) Get(owner Owner, id string) (Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.available(); err != nil {
		return Record{}, err
	}
	r, ok := s.records[id]
	if !ok || r.Admission.Owner != owner {
		return Record{}, ErrNotFound
	}
	return copyRecord(r), nil
}
func (s *Store) List(owner Owner) ([]Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.available(); err != nil {
		return nil, err
	}
	out := []Record{}
	for _, r := range s.records {
		if r.Admission.Owner == owner {
			out = append(out, copyRecord(r))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

// Pending is operator/internal recovery data, never a cross-app HTTP listing.
func (s *Store) Pending() ([]Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.available(); err != nil {
		return nil, err
	}
	out := []Record{}
	for _, r := range s.records {
		if r.Phase == "prepared" || r.Phase == "uncertain" || r.Phase == "reconciling" {
			out = append(out, copyRecord(r))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
func (s *Store) change(owner Owner, id string, fn func(*Record) error) (Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.available(); err != nil {
		return Record{}, err
	}
	r, ok := s.records[id]
	if !ok || r.Admission.Owner != owner {
		return Record{}, ErrNotFound
	}
	if err := fn(&r); err != nil {
		return Record{}, err
	}
	r.UpdatedAt = s.now().UTC()
	next := cloneRecords(s.records)
	next[id] = r
	if err := s.commit(next); err != nil {
		return Record{}, err
	}
	return copyRecord(r), nil
}

// BeginDispatch must succeed before any network request is issued.
func (s *Store) BeginDispatch(owner Owner, id string) (Record, error) {
	return s.change(owner, id, func(r *Record) error {
		if r.Phase != "prepared" && r.Phase != "uncertain" && r.Phase != "reconciling" {
			return ErrInvalid
		}
		r.Phase = "uncertain"
		return nil
	})
}

// CancelPrepared cancels a job that has never crossed the dispatch boundary.
// Uncertain jobs must first be reconciled with the runtime before cancelling.
func (s *Store) CancelPrepared(owner Owner, id string) (Record, error) {
	return s.change(owner, id, func(r *Record) error {
		if r.Phase != "prepared" {
			return ErrInvalid
		}
		r.Phase = "terminal"
		r.State = "cancelled"
		r.Reason = "cancelled_before_dispatch"
		return nil
	})
}

func validState(v string) bool {
	switch v {
	case "queued", "running", "waiting", "succeeded", "failed", "cancelled":
		return true
	}
	return false
}
func terminal(v string) bool { return v == "succeeded" || v == "failed" || v == "cancelled" }

// Link records a confirmed downstream response. A different runtime ID must
// never silently replace an existing correlation.
func (s *Store) Link(owner Owner, id, runtimeID, state, reason string) (Record, error) {
	return s.change(owner, id, func(r *Record) error {
		if !validToken(runtimeID) || !validState(state) || len(reason) > 512 {
			return ErrInvalid
		}
		if r.Phase == "prepared" || (r.Phase == "terminal" && r.RuntimeJobID == "") {
			return ErrInvalid
		}
		if r.RuntimeJobID != "" && r.RuntimeJobID != runtimeID {
			return ErrConflict
		}
		if r.Phase == "terminal" && (r.State != state || r.Reason != reason) {
			return ErrInvalid
		}
		r.RuntimeJobID = runtimeID
		r.State = state
		r.Reason = reason
		r.Phase = "linked"
		if terminal(state) {
			r.Phase = "terminal"
		}
		return nil
	})
}

func sortedRecords(records map[string]Record) []Record {
	out := make([]Record, 0, len(records))
	for _, r := range records {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
func (s *Store) commit(next map[string]Record) error {
	if err := s.save(sortedRecords(next)); err != nil {
		s.broken = true
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	s.records = next
	return nil
}
func checksum(records []Record) string {
	b, _ := json.Marshal(records)
	v := sha256.Sum256(b)
	return hex.EncodeToString(v[:])
}
func (s *Store) atomicSave(records []Record) error {
	b, err := json.Marshal(envelope{Version: 1, Records: records, SHA256: checksum(records)})
	if err != nil {
		return err
	}
	if len(b) > maxSnapshot {
		return errors.New("job snapshot exceeds limit")
	}
	f, err := os.CreateTemp(s.dir, ".snapshot-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(b)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = os.Rename(name, filepath.Join(s.dir, "snapshot.json")); err != nil {
		return err
	}
	dir, err := os.Open(s.dir)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
func (s *Store) load() error {
	path := filepath.Join(s.dir, "snapshot.json")
	fi, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !fi.Mode().IsRegular() || fi.Mode().Perm()&0077 != 0 || fi.Size() > maxSnapshot {
		return fmt.Errorf("%w: invalid snapshot file", ErrUnavailable)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var env envelope
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&env) != nil || d.Decode(new(any)) != io.EOF || env.Version != 1 || env.SHA256 != checksum(env.Records) {
		return fmt.Errorf("%w: corrupt snapshot", ErrUnavailable)
	}
	scopes := make(map[string]bool)
	for _, r := range env.Records {
		a, digest, e := normalize(r.Admission)
		scope := strings.Join([]string{a.Owner.App, a.Owner.Role, a.Owner.Subject, a.Key}, "\x00")
		if e != nil || digest != r.RequestHash || !validToken(r.ID) || r.DownstreamKey != "llmops-"+r.ID || r.CreatedAt.IsZero() || r.UpdatedAt.Before(r.CreatedAt) || !validState(r.State) || scopes[scope] {
			return fmt.Errorf("%w: inconsistent snapshot", ErrUnavailable)
		}
		if _, exists := s.records[r.ID]; exists {
			return fmt.Errorf("%w: duplicate job ID", ErrUnavailable)
		}
		switch r.Phase {
		case "prepared", "uncertain", "reconciling":
			if !validBeforeLink(r) {
				return ErrUnavailable
			}
		case "linked":
			if !validToken(r.RuntimeJobID) || terminal(r.State) {
				return ErrUnavailable
			}
		case "terminal":
			if (!validToken(r.RuntimeJobID) && !validUnlinkedTerminal(r)) || !terminal(r.State) {
				return ErrUnavailable
			}
		default:
			return ErrUnavailable
		}
		scopes[scope] = true
		s.records[r.ID] = r
	}
	return nil
}

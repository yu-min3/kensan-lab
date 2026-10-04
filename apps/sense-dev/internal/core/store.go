package core

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

type Store struct {
	root string
	lock *os.File
	mu   sync.Mutex
	data State
}

func Open(root string) (*Store, error) {
	if root == "" || !filepath.IsAbs(root) || filepath.Clean(root) == "/" {
		return nil, errors.New("state root must be an absolute non-root directory")
	}
	root = filepath.Clean(root)
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("state root must be a real directory")
	}
	if info.Mode().Perm()&0077 != 0 {
		return nil, fmt.Errorf("state root must be private (mode %04o)", info.Mode().Perm())
	}
	lock, err := os.OpenFile(filepath.Join(root, "controller.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		lock.Close()
		return nil, fmt.Errorf("controller already owns state: %w", err)
	}
	s := &Store{root: root, lock: lock, data: NewState()}
	b, err := os.ReadFile(filepath.Join(root, "state.json"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		s.Close()
		return nil, err
	}
	if len(b) > 0 {
		if err := json.Unmarshal(b, &s.data); err != nil || s.data.SchemaVersion != SchemaVersion {
			s.Close()
			return nil, fmt.Errorf("invalid state or schema version: %v", err)
		}
		if s.data.ImageReleases == nil {
			s.data.ImageReleases = map[string]ImageReleaseRecord{}
		}
		if s.data.ReleaseFlows == nil {
			s.data.ReleaseFlows = map[string]ReleaseFlow{}
		}
		if s.data.Deployments == nil {
			s.data.Deployments = map[string]DeploymentReceipt{}
		}
		if s.data.FeedbackLoops == nil {
			s.data.FeedbackLoops = map[string]FeedbackLoop{}
		}
		if s.data.Questions == nil {
			s.data.Questions = map[string]Question{}
		}
		if s.data.Approvals == nil {
			s.data.Approvals = map[string]ApprovalRequest{}
		}
		if s.data.Reports == nil {
			s.data.Reports = map[string]DailyReport{}
		}
		if s.data.ReportOutbox == nil {
			s.data.ReportOutbox = map[string]ReportOutboxEntry{}
		}
	}
	return s, nil
}

func (s *Store) Close() error {
	if s.lock == nil {
		return nil
	}
	err := syscall.Flock(int(s.lock.Fd()), syscall.LOCK_UN)
	closeErr := s.lock.Close()
	s.lock = nil
	if err != nil {
		return err
	}
	return closeErr
}

func (s *Store) Snapshot() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, _ := json.Marshal(s.data)
	var clone State
	_ = json.Unmarshal(b, &clone)
	return clone
}

func (s *Store) update(fn func(*State) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := json.Marshal(s.data)
	if err != nil {
		return err
	}
	var next State
	if err := json.Unmarshal(b, &next); err != nil {
		return err
	}
	if err := fn(&next); err != nil {
		return err
	}
	if err := writeJSONAtomic(filepath.Join(s.root, "state.json"), next); err != nil {
		return err
	}
	s.data = next
	return nil
}

func writeJSONAtomic(path string, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".state-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if err := f.Chmod(0600); err != nil {
		return err
	}
	if _, err := f.Write(b); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(f.Name(), path); err != nil {
		return err
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}

func newID() (string, error) {
	b := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func digest(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func event(kind, subject, detail string) Event {
	id, _ := newID()
	return Event{ID: id, Type: kind, Subject: subject, Detail: detail, At: time.Now().UTC()}
}

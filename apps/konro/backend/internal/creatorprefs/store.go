// Package creatorprefs owns the file-backed, device-shared creator preferences.
package creatorprefs

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var ErrConflict = errors.New("別の画面で設定が更新されました。最新の一覧を確認して、もう一度操作してください")
var ErrInvalid = errors.New("作り手の設定を確認してください")

type Creator struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	URL        string `json:"url"`
	Additional bool   `json:"additional"`
}
type Snapshot struct {
	Revision string    `json:"revision"`
	Creators []Creator `json:"creators"`
}
type Result struct {
	Snapshot
	Undo string `json:"undo,omitempty"`
}
type document struct {
	Version  int       `json:"version"`
	Creators []Creator `json:"creators"`
}
type receipt struct {
	creator Creator
	remove  bool
	until   time.Time
	index   int
}
type Store struct {
	mu       sync.Mutex
	dir      string
	receipts map[string]receipt
}

func New(dir string) *Store {
	return &Store{dir: filepath.Join(dir, ".settings"), receipts: map[string]receipt{}}
}
func Initial() []Creator {
	return []Creator{
		{ID: "youtube:UCLcqrpFhRlJnm6etuLQblWg", Name: "笠原将弘", URL: "https://www.youtube.com/channel/UCLcqrpFhRlJnm6etuLQblWg", Additional: true},
		{ID: "youtube:UCP2gnyy_-ToZeIDw6qeI6HA", Name: "ジョージ（城二郎）", URL: "https://www.youtube.com/channel/UCP2gnyy_-ToZeIDw6qeI6HA", Additional: true},
		{ID: "kurashiru:chef:6", Name: "中村和成", URL: "https://www.kurashiru.com/chefs/6", Additional: true},
	}
}

var channelID = regexp.MustCompile(`^youtube:UC[A-Za-z0-9_-]{22}$`)
var chefID = regexp.MustCompile(`^kurashiru:chef:[1-9][0-9]{0,9}$`)

func ProfileURL(id string) string {
	if channelID.MatchString(id) {
		return "https://www.youtube.com/channel/" + strings.TrimPrefix(id, "youtube:")
	}
	if chefID.MatchString(id) {
		return "https://www.kurashiru.com/chefs/" + strings.TrimPrefix(id, "kurashiru:chef:")
	}
	return ""
}
func Valid(c Creator) bool {
	return ProfileURL(c.ID) != "" && c.URL == ProfileURL(c.ID) && strings.TrimSpace(c.Name) != "" && len([]rune(c.Name)) <= 120 && !strings.ContainsAny(c.Name, "\r\n\x00")
}
func revision(b []byte) string { v := sha256.Sum256(b); return hex.EncodeToString(v[:]) }
func (s *Store) ensureDir() error {
	if err := os.MkdirAll(s.dir, 0700); err != nil {
		return err
	}
	info, err := os.Lstat(s.dir)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return ErrInvalid
	}
	return nil
}
func (s *Store) read() (Snapshot, error) {
	if err := s.ensureDir(); err != nil {
		return Snapshot{}, err
	}
	path := filepath.Join(s.dir, "creators.json")
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return s.write(Initial())
	}
	if err != nil {
		return Snapshot{}, err
	}
	if !info.Mode().IsRegular() || info.Size() > 256<<10 {
		return Snapshot{}, ErrInvalid
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return Snapshot{}, err
	}
	var d document
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	if dec.Decode(&d) != nil || dec.Decode(new(any)) != io.EOF || d.Version != 1 || validate(d.Creators) != nil {
		return Snapshot{}, errors.New("好きな作り手の設定ファイルを読み込めません。元のファイルは変更していません")
	}
	return Snapshot{Revision: revision(b), Creators: d.Creators}, nil
}
func validate(cs []Creator) error {
	if cs == nil || len(cs) > 200 {
		return ErrInvalid
	}
	ids := map[string]bool{}
	additional := 0
	for _, c := range cs {
		if !Valid(c) || ids[c.ID] {
			return ErrInvalid
		}
		ids[c.ID] = true
		if c.Additional {
			additional++
		}
	}
	if additional > 3 {
		return errors.New("追加でも探す作り手は3人までです")
	}
	return nil
}
func (s *Store) write(cs []Creator) (Snapshot, error) {
	if err := validate(cs); err != nil {
		return Snapshot{}, err
	}
	if err := s.ensureDir(); err != nil {
		return Snapshot{}, err
	}
	b, err := json.MarshalIndent(document{Version: 1, Creators: cs}, "", "  ")
	if err != nil {
		return Snapshot{}, err
	}
	b = append(b, '\n')
	f, err := os.CreateTemp(s.dir, ".creators-")
	if err != nil {
		return Snapshot{}, err
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
		return Snapshot{}, err
	}
	if err = os.Rename(name, filepath.Join(s.dir, "creators.json")); err != nil {
		return Snapshot{}, err
	}
	d, err := os.Open(s.dir)
	if err != nil {
		return Snapshot{}, err
	}
	err = d.Sync()
	d.Close()
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{Revision: revision(b), Creators: cs}, nil
}
func (s *Store) Get() (Snapshot, error) { s.mu.Lock(); defer s.mu.Unlock(); return s.read() }
func count(cs []Creator) int {
	n := 0
	for _, c := range cs {
		if c.Additional {
			n++
		}
	}
	return n
}
func (s *Store) remember(c Creator, remove bool, index int) string {
	for k, r := range s.receipts {
		if time.Now().After(r.until) {
			delete(s.receipts, k)
		}
	}
	if len(s.receipts) >= 128 {
		for k := range s.receipts {
			delete(s.receipts, k)
			break
		}
	}
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return ""
	}
	token := hex.EncodeToString(id[:])
	s.receipts[token] = receipt{c, remove, time.Now().Add(5 * time.Minute), index}
	return token
}
func (s *Store) Add(expected string, c Creator) (Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap, err := s.read()
	if err != nil {
		return Result{}, err
	}
	if snap.Revision != expected {
		return Result{}, ErrConflict
	}
	if !Valid(c) {
		return Result{}, ErrInvalid
	}
	for _, old := range snap.Creators {
		if old.ID == c.ID {
			return Result{Snapshot: snap}, nil
		}
	}
	c.Additional = count(snap.Creators) < 3
	cs := append(snap.Creators, c)
	next, err := s.write(cs)
	if err != nil {
		return Result{}, err
	}
	return Result{Snapshot: next, Undo: s.remember(c, true, len(snap.Creators))}, nil
}
func (s *Store) Remove(expected, id string) (Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap, err := s.read()
	if err != nil {
		return Result{}, err
	}
	if snap.Revision != expected {
		return Result{}, ErrConflict
	}
	for i, c := range snap.Creators {
		if c.ID == id {
			cs := append(snap.Creators[:i:i], snap.Creators[i+1:]...)
			next, err := s.write(cs)
			if err != nil {
				return Result{}, err
			}
			return Result{Snapshot: next, Undo: s.remember(c, false, i)}, nil
		}
	}
	return Result{}, os.ErrNotExist
}
func (s *Store) Additional(expected, id string, on bool) (Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap, err := s.read()
	if err != nil {
		return Result{}, err
	}
	if snap.Revision != expected {
		return Result{}, ErrConflict
	}
	for i, c := range snap.Creators {
		if c.ID == id {
			snap.Creators[i].Additional = on
			next, err := s.write(snap.Creators)
			return Result{Snapshot: next}, err
		}
	}
	return Result{}, os.ErrNotExist
}
func (s *Store) Undo(expected, token string) (Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap, err := s.read()
	if err != nil {
		return Result{}, err
	}
	if snap.Revision != expected {
		return Result{}, ErrConflict
	}
	r, ok := s.receipts[token]
	if !ok || time.Now().After(r.until) {
		return Result{}, errors.New("取り消せる時間を過ぎました。一覧からもう一度変更できます")
	}
	cs := snap.Creators
	if r.remove {
		for i, c := range cs {
			if c.ID == r.creator.ID {
				cs = append(cs[:i:i], cs[i+1:]...)
				break
			}
		}
	} else {
		for _, c := range cs {
			if c.ID == r.creator.ID {
				return Result{}, ErrConflict
			}
		}
		r.creator.Additional = r.creator.Additional && count(cs) < 3
		index := r.index
		if index > len(cs) {
			index = len(cs)
		}
		cs = append(cs, Creator{})
		copy(cs[index+1:], cs[index:])
		cs[index] = r.creator
	}
	next, err := s.write(cs)
	if err != nil {
		return Result{}, err
	}
	delete(s.receipts, token)
	return Result{Snapshot: next}, nil
}

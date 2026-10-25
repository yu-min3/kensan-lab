// Package taxonomy assigns display groups without rewriting recipe tags.
package taxonomy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var ErrConflict = errors.New("分類が別の画面で更新されています。読み直してください")
var ErrInvalid = errors.New("タグ分類の内容を確認してください")

type Group struct {
	ID       string `json:"id"`
	Category string `json:"category"`
	Label    string `json:"label"`
}
type Snapshot struct {
	Version     int               `json:"version"`
	Revision    string            `json:"revision"`
	Assignments map[string]string `json:"assignments"`
	Groups      []Group           `json:"groups"`
}
type Store struct {
	mu  sync.Mutex
	dir string
}

func New(dir string) *Store { return &Store{dir: dir} }
func Groups() []Group {
	return []Group{{"cuisine", "料理種類", "系統"}, {"dish", "料理種類", "品目"}, {"ingredient", "素材種類", "素材"}, {"other", "その他", "自由タグ"}}
}
func Initial() map[string]string {
	a := map[string]string{}
	for group, tags := range map[string][]string{"cuisine": {"和食", "中華", "洋食", "イタリアン、フレンチ"}, "dish": {"メイン", "副菜", "パスタ", "スープ", "デザート"}, "ingredient": {"野菜", "魚介", "豚肉", "鶏肉", "牛肉", "きのこ", "卵"}, "other": {"実里ちゃん", "その他"}} {
		for _, tag := range tags {
			a[tag] = group
		}
	}
	return a
}
func valid(a map[string]string) bool {
	if a == nil || len(a) > 2000 {
		return false
	}
	for tag, group := range a {
		if strings.TrimSpace(tag) == "" || len(tag) > 3000 || strings.ContainsAny(tag, "\x00\r\n") {
			return false
		}
		if group != "cuisine" && group != "dish" && group != "ingredient" && group != "other" {
			return false
		}
	}
	return true
}
func (s *Store) read() (Snapshot, error) {
	path := filepath.Join(s.dir, "tag-taxonomy.json")
	info, e := os.Lstat(path)
	if errors.Is(e, os.ErrNotExist) {
		return Snapshot{1, "initial", Initial(), Groups()}, nil
	}
	if e != nil {
		return Snapshot{}, e
	}
	if !info.Mode().IsRegular() || info.Size() > 1<<20 {
		return Snapshot{}, ErrInvalid
	}
	b, e := os.ReadFile(path)
	if e != nil {
		return Snapshot{}, e
	}
	var snap Snapshot
	if json.Unmarshal(b, &snap) != nil || snap.Version != 1 || !valid(snap.Assignments) {
		return Snapshot{}, ErrInvalid
	}
	h := sha256.Sum256(b)
	snap.Revision = hex.EncodeToString(h[:])
	snap.Groups = Groups()
	return snap, nil
}
func (s *Store) Get() (Snapshot, error) { s.mu.Lock(); defer s.mu.Unlock(); return s.read() }
func (s *Store) Put(expected string, a map[string]string) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	old, e := s.read()
	if e != nil {
		return Snapshot{}, e
	}
	if expected == "" || expected != old.Revision {
		return Snapshot{}, ErrConflict
	}
	if !valid(a) {
		return Snapshot{}, ErrInvalid
	}
	b, e := json.MarshalIndent(Snapshot{Version: 1, Assignments: a}, "", "  ")
	if e != nil {
		return Snapshot{}, e
	}
	f, e := os.CreateTemp(s.dir, ".taxonomy-")
	if e != nil {
		return Snapshot{}, e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if e = f.Chmod(0600); e == nil {
		_, e = f.Write(b)
	}
	if e == nil {
		e = f.Sync()
	}
	closeErr := f.Close()
	if e == nil {
		e = closeErr
	}
	if e != nil {
		return Snapshot{}, e
	}
	if e = os.Rename(tmp, filepath.Join(s.dir, "tag-taxonomy.json")); e != nil {
		return Snapshot{}, e
	}
	dir, e := os.Open(s.dir)
	if e != nil {
		return Snapshot{}, e
	}
	e = dir.Sync()
	dir.Close()
	if e != nil {
		return Snapshot{}, e
	}
	return s.read()
}

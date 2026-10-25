package mealplan

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/ingredients"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/recipe"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

var ErrConflict = errors.New("買い物リストが更新されています。最新版を確認してください")
var ErrInvalid = errors.New("献立や数量の入力を確認してください")

type Selection struct {
	ID        string         `json:"id,omitempty"`
	File      string         `json:"file"`
	Scale     string         `json:"scale"`
	Refresh   bool           `json:"refresh,omitempty"`
	Overrides map[int]string `json:"overrides,omitempty"`
}
type Item struct {
	ID        string         `json:"id"`
	Recipe    recipe.Recipe  `json:"recipe"`
	Scale     string         `json:"scale"`
	Overrides map[int]string `json:"overrides,omitempty"`
}
type Source struct {
	ItemID  string `json:"itemId"`
	File    string `json:"file"`
	Version string `json:"version"`
	Title   string `json:"title"`
	Row     int    `json:"row"`
	Raw     string `json:"raw"`
	Scaled  string `json:"scaled"`
}
type Shopping struct {
	ID      string           `json:"id"`
	Line    ingredients.Line `json:"line"`
	Text    string           `json:"text"`
	Sources []Source         `json:"sources"`
	Status  string           `json:"status"`
	Have    string           `json:"have,omitempty"`
	Note    string           `json:"note,omitempty"`
	Manual  bool             `json:"manual"`
}
type Plan struct {
	Schema     int               `json:"schema"`
	ID         string            `json:"id"`
	Revision   int               `json:"revision"`
	Name       string            `json:"name"`
	Items      []Item            `json:"items"`
	Shopping   []Shopping        `json:"shopping"`
	Operations map[string]string `json:"operations"`
	CreateHash string            `json:"createHash"`
}
type Manual struct {
	ID   string `json:"id,omitempty"`
	Text string `json:"text"`
}
type Change struct {
	Revision int         `json:"revision"`
	Name     string      `json:"name"`
	Items    []Selection `json:"items,omitempty"`
	Manual   []Manual    `json:"manual,omitempty"`
	ItemID   string      `json:"itemId,omitempty"`
	Status   string      `json:"status,omitempty"`
	Have     string      `json:"have,omitempty"`
	Note     string      `json:"note,omitempty"`
	Preview  bool        `json:"preview,omitempty"`
}
type Store struct {
	mu        sync.Mutex
	root, dir string
}

func New(root string) *Store { return &Store{root: root, dir: filepath.Join(root, "meal-plans")} }

var safeID = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,128}$`)

func hash(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func (s *Store) ensure() error {
	if e := os.MkdirAll(s.dir, 0700); e != nil {
		return e
	}
	i, e := os.Lstat(s.dir)
	if e != nil {
		return e
	}
	if !i.IsDir() || i.Mode()&os.ModeSymlink != 0 {
		return ErrInvalid
	}
	return nil
}
func (s *Store) read(id string) (Plan, error) {
	if !safeID.MatchString(id) {
		return Plan{}, ErrInvalid
	}
	if e := s.ensure(); e != nil {
		return Plan{}, e
	}
	p := filepath.Join(s.dir, id+".json")
	info, e := os.Lstat(p)
	if e != nil {
		return Plan{}, e
	}
	if !info.Mode().IsRegular() || info.Size() > 8<<20 {
		return Plan{}, ErrInvalid
	}
	b, e := os.ReadFile(p)
	if e != nil {
		return Plan{}, e
	}
	var plan Plan
	if json.Unmarshal(b, &plan) != nil || plan.Schema != 1 || plan.ID != id || plan.Revision < 1 || len(plan.Items) == 0 || plan.Operations == nil {
		return Plan{}, ErrInvalid
	}
	return plan, nil
}
func (s *Store) write(p Plan) error {
	if e := s.ensure(); e != nil {
		return e
	}
	b, e := json.MarshalIndent(p, "", "  ")
	if e != nil {
		return e
	}
	if len(b) > 8<<20 {
		return ErrInvalid
	}
	f, e := os.CreateTemp(s.dir, ".plan-")
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
	if e = os.Rename(tmp, filepath.Join(s.dir, p.ID+".json")); e != nil {
		return e
	}
	dir, e := os.Open(s.dir)
	if e != nil {
		return e
	}
	e = dir.Sync()
	dir.Close()
	return e
}
func (s *Store) Get(id string) (Plan, error) { s.mu.Lock(); defer s.mu.Unlock(); return s.read(id) }
func (s *Store) List() ([]Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.ensure(); e != nil {
		return nil, e
	}
	entries, e := os.ReadDir(s.dir)
	if e != nil {
		return nil, e
	}
	result := []Plan{}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") {
			p, e := s.read(strings.TrimSuffix(entry.Name(), ".json"))
			if e != nil {
				return nil, e
			}
			p.Operations = nil
			result = append(result, p)
		}
	}
	return result, nil
}
func (s *Store) items(selections []Selection, old []Item) ([]Item, error) {
	if len(selections) == 0 || len(selections) > 50 {
		return nil, ErrInvalid
	}
	result := []Item{}
	seen := map[string]bool{}
	for i, v := range selections {
		if _, e := ingredients.Multiply("1", v.Scale); e != nil {
			return nil, e
		}
		var snap *recipe.Recipe
		for _, item := range old {
			if item.ID == v.ID && item.Recipe.File == v.File && !v.Refresh {
				r := item.Recipe
				snap = &r
				break
			}
		}
		if snap == nil {
			r, e := recipe.Load(s.root, v.File)
			if e != nil {
				return nil, e
			}
			snap = r
		}
		id := v.ID
		if id == "" {
			id = hash([]any{v.File, i})[:16]
		}
		if !safeID.MatchString(id) || seen[id] {
			return nil, ErrInvalid
		}
		seen[id] = true
		for row, text := range v.Overrides {
			if row < 0 || row >= len(snap.Ingredients) || strings.TrimSpace(text) == "" || len(text) > 4000 {
				return nil, ErrInvalid
			}
		}
		result = append(result, Item{ID: id, Recipe: *snap, Scale: v.Scale, Overrides: v.Overrides})
	}
	return result, nil
}
func aggregate(items []Item, old []Shopping) []Shopping {
	result := []Shopping{}
	indexes := map[string]int{}
	for _, item := range items {
		for row, raw := range item.Recipe.Ingredients {
			line := ingredients.Parse(raw)
			if replacement, ok := item.Overrides[row]; ok {
				line = ingredients.Parse(replacement)
			}
			if line.Kind == "group" {
				continue
			}
			line, _ = line.Scaled(item.Scale)
			id := line.Key()
			source := Source{item.ID, item.Recipe.File, item.Recipe.Version, item.Recipe.Title, row, raw, line.Text()}
			if line.Kind == "unparsed" {
				source.Scaled = fmt.Sprintf("%s（倍率%s・要確認）", raw, item.Scale)
			}
			if idx, ok := indexes[id]; ok {
				entry := &result[idx]
				entry.Sources = append(entry.Sources, source)
				if line.Kind == "exact" || line.Kind == "range" {
					entry.Line.Min = ingredients.Add(entry.Line.Min, line.Min)
					if line.Max != "" {
						entry.Line.Max = ingredients.Add(entry.Line.Max, line.Max)
					}
					entry.Text = entry.Line.Text()
				}
			} else {
				indexes[id] = len(result)
				result = append(result, Shopping{ID: id, Line: line, Text: line.Text(), Sources: []Source{source}, Status: "buy"})
			}
		}
	}
	for i := range result {
		for _, previous := range old {
			if result[i].ID == previous.ID && !previous.Manual {
				result[i].Status = previous.Status
				result[i].Have = previous.Have
				result[i].Note = previous.Note
				if (result[i].Text != previous.Text || hash(result[i].Sources) != hash(previous.Sources)) && (previous.Status != "buy" || previous.Have != "") {
					result[i].Status = "review"
					result[i].Have = ""
				}
				break
			}
		}
	}
	for _, previous := range old {
		if previous.Manual {
			result = append(result, previous)
		}
	}
	return result
}
func (s *Store) Create(op string, in Change) (Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !safeID.MatchString(op) {
		return Plan{}, ErrInvalid
	}
	id := hash(op)[:32]
	digest := hash(in)
	if old, e := s.read(id); e == nil {
		if old.CreateHash != digest {
			return Plan{}, ErrConflict
		}
		return old, nil
	} else if !errors.Is(e, os.ErrNotExist) {
		return Plan{}, e
	}
	items, e := s.items(in.Items, nil)
	if e != nil {
		return Plan{}, e
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = "買い物リスト"
	}
	if len(name) > 300 {
		return Plan{}, ErrInvalid
	}
	p := Plan{Schema: 1, ID: id, Revision: 1, Name: name, Items: items, Operations: map[string]string{}, CreateHash: digest}
	p.Shopping = aggregate(items, nil)
	if e = s.manual(&p, in.Manual, op); e != nil {
		return Plan{}, e
	}
	if e = s.write(p); e != nil {
		return Plan{}, e
	}
	return p, nil
}
func (s *Store) manual(p *Plan, rows []Manual, op string) error {
	for i, row := range rows {
		if strings.TrimSpace(row.Text) == "" || len(row.Text) > 4000 {
			return ErrInvalid
		}
		id := row.ID
		if id == "" {
			id = "manual-" + hash([]any{op, i})[:16]
		}
		if !safeID.MatchString(id) {
			return ErrInvalid
		}
		for _, existing := range p.Shopping {
			if existing.ID == id {
				return ErrInvalid
			}
		}
		line := ingredients.Parse(row.Text)
		if line.Kind == "group" {
			line.Kind = "unparsed"
		}
		p.Shopping = append(p.Shopping, Shopping{ID: id, Line: line, Text: row.Text, Status: "buy", Manual: true, Sources: []Source{}})
	}
	return nil
}
func (s *Store) Update(id, op string, in Change) (Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.read(id)
	if e != nil {
		return p, e
	}
	if !safeID.MatchString(op) {
		return p, ErrInvalid
	}
	digest := hash(in)
	if saved, ok := p.Operations[op]; ok {
		if saved != digest {
			return p, ErrConflict
		}
		return p, nil
	}
	if in.Revision != p.Revision {
		return p, ErrConflict
	}
	if in.Name != "" {
		if len(in.Name) > 300 {
			return p, ErrInvalid
		}
		p.Name = in.Name
	}
	if in.Items != nil {
		items, e := s.items(in.Items, p.Items)
		if e != nil {
			return p, e
		}
		p.Items = items
		p.Shopping = aggregate(items, p.Shopping)
	}
	if e = s.manual(&p, in.Manual, op); e != nil {
		return p, e
	}
	if in.ItemID != "" {
		found := false
		for i := range p.Shopping {
			v := &p.Shopping[i]
			if v.ID != in.ItemID {
				continue
			}
			found = true
			if in.Status != "buy" && in.Status != "home" && in.Status != "bought" && in.Status != "review" {
				return p, ErrInvalid
			}
			if len(in.Note) > 4000 {
				return p, ErrInvalid
			}
			if in.Have != "" {
				if v.Line.Kind != "exact" {
					return p, ErrInvalid
				}
				q, e := ingredients.Multiply(in.Have, "1")
				if e != nil {
					return p, e
				}
				if strings.HasPrefix(q, "-") {
					return p, ErrInvalid
				}
				v.Have = q
			} else {
				v.Have = ""
			}
			v.Status = in.Status
			v.Note = in.Note
		}
		if !found {
			return p, ErrInvalid
		}
	}
	if in.Preview {
		return p, nil
	}
	p.Revision++
	p.Operations[op] = digest
	if e = s.write(p); e != nil {
		return p, e
	}
	return p, nil
}
func (s Shopping) Remaining() string {
	if s.Have == "" {
		return s.Text
	}
	negative := "-" + s.Have
	q := ingredients.Add(s.Line.Min, negative)
	if strings.HasPrefix(q, "-") {
		q = "0"
	}
	line := s.Line
	line.Min = q
	return line.Text()
}
func RowKey(row int) string { return strconv.Itoa(row) }

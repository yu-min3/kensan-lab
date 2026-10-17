package recipe

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"
)

var ErrConflict = errors.New("レシピは別の操作で更新されています。再読み込みして確認してください")
var ErrInvalid = errors.New("レシピの入力を確認してください")

// One application owns writes to this directory. Serialize edits and ZIP writes.
var writes sync.Mutex

type Editable struct {
	Title       string   `json:"title"`
	Servings    string   `json:"servings"`
	Tags        []string `json:"tags"`
	PrepTime    string   `json:"prepTime"`
	CookTime    string   `json:"cookTime"`
	Source      string   `json:"source"`
	Rating      int      `json:"rating"`
	Ingredients []string `json:"ingredients"`
	Steps       []string `json:"steps"`
	Notes       string   `json:"notes"`
}

func editable(r *Recipe) Editable {
	return Editable{r.Title, r.Servings, r.Tags, r.PrepTime, r.CookTime, r.Source, r.Rating, r.Ingredients, r.Steps, r.Notes}
}
func normalizeEdit(e Editable) Editable {
	e.Title = strings.TrimSpace(e.Title)
	e.Servings = strings.TrimSpace(e.Servings)
	e.Source = strings.TrimSpace(e.Source)
	e.Notes = strings.TrimSpace(e.Notes)
	if e.Tags == nil {
		e.Tags = []string{}
	}
	if e.Ingredients == nil {
		e.Ingredients = []string{}
	}
	if e.Steps == nil {
		e.Steps = []string{}
	}
	return e
}
func validateEdit(e Editable) error {
	if err := ValidateContent(Content{e.Title, e.Servings, e.Ingredients, e.Steps}); err != nil {
		return err
	}
	if strings.ContainsRune(e.Notes, 0) || strings.TrimSpace(e.Title) == "" || len(e.Source) > 2048 || len(e.Notes) > 65536 || e.Rating < 0 || e.Rating > 5 || len(e.Tags) > 30 {
		return ErrInvalid
	}
	for _, s := range []string{e.Title, e.Servings, e.Source} {
		if strings.ContainsAny(s, "\r\n\x00") {
			return ErrInvalid
		}
	}
	for _, s := range []string{e.PrepTime, e.CookTime} {
		if s != "" && (s == "PT" || !duration.MatchString(s)) {
			return ErrInvalid
		}
	}
	for _, a := range [][]string{e.Tags} {
		for _, s := range a {
			if strings.TrimSpace(s) == "" || len(s) > 3000 || strings.ContainsAny(s, "\r\n\x00") {
				return ErrInvalid
			}
		}
	}
	return nil
}

var duration = regexp.MustCompile(`^PT(?:[0-9]+H)?(?:[0-9]+M)?(?:[0-9]+S)?$`)

func version(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func readSource(dir, name string) ([]byte, error) {
	if !validName(name) {
		return nil, ErrInvalid
	}
	if strings.HasPrefix(name, "imported/") {
		if i, e := os.Lstat(filepath.Join(dir, "imported")); e != nil {
			return nil, e
		} else if !i.IsDir() || i.Mode()&os.ModeSymlink != 0 {
			return nil, ErrInvalid
		}
	}
	p := filepath.Join(dir, name+".md")
	i, e := os.Lstat(p)
	if e != nil {
		return nil, e
	}
	if !i.Mode().IsRegular() {
		return nil, ErrInvalid
	}
	return os.ReadFile(p)
}

func atomicReplace(path string, data []byte) error { return atomicWrite(path, data, true) }
func atomicWrite(path string, data []byte, replace bool) error {
	f, e := os.CreateTemp(filepath.Dir(path), ".recipe-")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if e = f.Chmod(0600); e == nil {
		_, e = f.Write(data)
	}
	if e == nil {
		e = f.Sync()
	}
	closed := f.Close()
	if e == nil {
		e = closed
	}
	if e != nil {
		return e
	}
	if replace {
		e = os.Rename(tmp, path)
	} else {
		e = os.Link(tmp, path)
	}
	if e != nil {
		return e
	}
	d, e := os.Open(filepath.Dir(path))
	if e != nil {
		return e
	}
	defer d.Close()
	return d.Sync()
}
func Update(dir, name, expected string, e Editable) (*Recipe, error) {
	e = normalizeEdit(e)
	if err := validateEdit(e); err != nil {
		return nil, err
	}
	writes.Lock()
	defer writes.Unlock()
	old, err := readSource(dir, name)
	if err != nil {
		return nil, err
	}
	if expected == "" || expected != version(old) {
		return nil, ErrConflict
	}
	doc, err := DecodeDocument(old)
	if err != nil {
		return nil, err
	}
	data, err := doc.render(e, time.Now().Format("2006-01-02"), true)
	if err != nil {
		return nil, err
	}
	if err = atomicReplace(filepath.Join(dir, name+".md"), data); err != nil {
		return nil, err
	}
	return Load(dir, name)
}
func Create(dir, id string, e Editable) (*Recipe, error) {
	e = normalizeEdit(e)
	return create(dir, id, Recipe{Meta: Meta{Title: e.Title, Servings: e.Servings, Tags: e.Tags, PrepTime: e.PrepTime, CookTime: e.CookTime, Source: e.Source, Rating: e.Rating}, Ingredients: e.Ingredients, Steps: e.Steps, Notes: e.Notes}, true)
}
func create(dir, id string, r Recipe, edited bool) (*Recipe, error) {
	e := normalizeEdit(editable(&r))
	if err := validateEdit(e); err != nil {
		return nil, err
	}
	if id == "" {
		var random [16]byte
		if _, err := rand.Read(random[:]); err != nil {
			return nil, err
		}
		id = hex.EncodeToString(random[:])
	}
	if !importedID.MatchString(id) {
		return nil, ErrInvalid
	}
	writes.Lock()
	defer writes.Unlock()
	name := "imported/" + id
	if old, err := Load(dir, name); err == nil {
		if reflect.DeepEqual(normalizeEdit(editable(old)), e) {
			return old, nil
		}
		return nil, ErrConflict
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	target := filepath.Join(dir, "imported")
	if err := os.MkdirAll(target, 0700); err != nil {
		return nil, err
	}
	if i, err := os.Lstat(target); err != nil {
		return nil, err
	} else if !i.IsDir() || i.Mode()&os.ModeSymlink != 0 {
		return nil, ErrInvalid
	}
	data, err := Encode(r, time.Now().Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	if edited {
		doc, err := DecodeDocument(data)
		if err != nil {
			return nil, err
		}
		data, err = doc.render(e, "", true)
		if err != nil {
			return nil, err
		}
	}
	if err := atomicWrite(filepath.Join(dir, name+".md"), data, false); err != nil {
		return nil, err
	}
	return Load(dir, name)
}

// WriteLegacyImport keeps Konro's edits authoritative when a ZIP is reimported.
func WriteLegacyImport(dir, name string, data []byte) (bool, error) {
	if strings.ContainsAny(name, "/\\") || !strings.HasSuffix(name, ".md") || !validName(strings.TrimSuffix(name, ".md")) {
		return false, ErrInvalid
	}
	doc, err := DecodeDocument(data)
	if err != nil {
		return false, err
	}
	if err = validateEdit(normalizeEdit(editable(&doc.Recipe))); err != nil {
		return false, err
	}
	writes.Lock()
	defer writes.Unlock()
	old, err := readSource(dir, strings.TrimSuffix(name, ".md"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	if err == nil {
		doc, decodeErr := DecodeDocument(old)
		if decodeErr != nil {
			return false, decodeErr
		}
		if doc.Edited() {
			return false, nil
		}
	}
	if err = atomicReplace(filepath.Join(dir, name), data); err != nil {
		return false, err
	}
	return true, nil
}

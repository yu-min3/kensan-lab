package recipe

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
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
	if strings.ContainsRune(e.Notes, 0) || strings.TrimSpace(e.Title) == "" || len(e.Title) > 300 || len(e.Servings) > 200 || len(e.Source) > 2048 || len(e.Notes) > 65536 || e.Rating < 0 || e.Rating > 5 || len(e.Tags) > 30 || len(e.Ingredients) > 100 || len(e.Steps) > 150 {
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
	for _, a := range [][]string{e.Tags, e.Ingredients, e.Steps} {
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

// Our writable format uses one-line metadata. Refuse unsupported editable
// blocks instead of dropping values that the reader cannot represent.
func checkEditableFormat(data []byte) error {
	src := string(data)
	if strings.HasPrefix(src, "\ufeff") {
		return ErrInvalid
	}
	if strings.HasPrefix(src, "---") && !strings.HasPrefix(src, "---\n") {
		return ErrInvalid
	}
	if !strings.HasPrefix(src, "---\n") {
		return nil
	}
	end := strings.Index(src[4:], "\n---\n")
	if end < 0 {
		return ErrInvalid
	}
	keys := map[string]bool{"title": true, "servings": true, "tags": true, "prep_time": true, "cook_time": true, "source": true, "rating": true}
	editableBlock := false
	topLevelSeen := false
	for _, line := range strings.Split(src[4:4+end], "\n") {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		if line[0] == ' ' || line[0] == '\t' {
			if !topLevelSeen || editableBlock {
				return fmt.Errorf("%w: 複数行のmetadataには未対応です。元のファイルは変更していません", ErrInvalid)
			}
			continue
		}
		key, value, hasKey := strings.Cut(line, ":")
		if !hasKey || strings.HasPrefix(line, "- ") {
			if editableBlock {
				return fmt.Errorf("%w: 複数行のmetadataには未対応です。元のファイルは変更していません", ErrInvalid)
			}
			continue
		}
		if keys[strings.Trim(strings.TrimSpace(key), "\"'")] && !keys[key] {
			return ErrInvalid
		}
		topLevelSeen = true
		editableBlock = keys[key]
		value = strings.TrimSpace(value)
		if editableBlock && (strings.HasPrefix(value, "|") || strings.HasPrefix(value, ">")) {
			return fmt.Errorf("%w: 複数行のmetadataには未対応です。元のファイルは変更していません", ErrInvalid)
		}
	}
	return nil
}

func renderEdit(old []byte, e Editable) []byte {
	list := func(a []string) string {
		if a == nil {
			a = []string{}
		}
		b, _ := json.Marshal(a)
		return string(b)
	}
	values := map[string]string{"title": strconv.Quote(e.Title), "servings": strconv.Quote(e.Servings), "tags": list(e.Tags), "prep_time": e.PrepTime, "cook_time": e.CookTime, "source": strconv.Quote(e.Source), "rating": strconv.Itoa(e.Rating), "updated": time.Now().Format("2006-01-02"), "konro_edited": "true"}
	var fm []string
	if strings.HasPrefix(string(old), "---\n") {
		if end := strings.Index(string(old[4:]), "\n---\n"); end >= 0 {
			dropping := false
			for _, line := range strings.Split(string(old[4:4+end]), "\n") {
				if len(line) > 0 && line[0] != ' ' && line[0] != '\t' && line[0] != '#' {
					k, _, ok := strings.Cut(line, ":")
					_, dropping = values[k]
					dropping = dropping && ok
				}
				if !dropping {
					fm = append(fm, line)
				}
			}
		}
	}
	if len(old) == 0 {
		fm = append(fm, "created: "+time.Now().Format("2006-01-02"))
	}
	for _, key := range []string{"title", "servings", "tags", "prep_time", "cook_time", "source", "rating", "updated", "konro_edited"} {
		if values[key] != "" {
			fm = append(fm, key+": "+values[key])
		}
	}
	var b strings.Builder
	b.WriteString("---\n" + strings.Join(fm, "\n") + "\n---\n\n## 材料\n")
	for _, v := range e.Ingredients {
		fmt.Fprintf(&b, "- %s\n", v)
	}
	b.WriteString("\n## 手順\n")
	for i, v := range e.Steps {
		fmt.Fprintf(&b, "%d. %s\n", i+1, v)
	}
	b.WriteString("\n## メモ\n" + e.Notes + "\n")
	return []byte(b.String())
}
func atomicReplace(path string, data []byte) error {
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
	if e = os.Rename(tmp, path); e != nil {
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
	if err := checkEditableFormat(old); err != nil {
		return nil, err
	}

	if err = atomicReplace(filepath.Join(dir, name+".md"), renderEdit(old, e)); err != nil {
		return nil, err
	}
	return Load(dir, name)
}
func Create(dir, id string, e Editable) (*Recipe, error) {
	e = normalizeEdit(e)
	if err := validateEdit(e); err != nil {
		return nil, err
	}
	if id == "" {
		var r [16]byte
		if _, err := rand.Read(r[:]); err != nil {
			return nil, err
		}
		id = hex.EncodeToString(r[:])
	}
	if !importedID.MatchString(id) {
		return nil, ErrInvalid
	}
	writes.Lock()
	defer writes.Unlock()
	name := "imported/" + id
	// Stable client-generated IDs make a lost create response safe to retry.
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
	if err := atomicReplace(filepath.Join(dir, name+".md"), renderEdit(nil, e)); err != nil {
		return nil, err
	}
	return Load(dir, name)
}

// WriteLegacyImport keeps Konro's edits authoritative when a ZIP is reimported.
func WriteLegacyImport(dir, name string, data []byte) (bool, error) {
	if strings.ContainsAny(name, "/\\") || !strings.HasSuffix(name, ".md") || !validName(strings.TrimSuffix(name, ".md")) {
		return false, ErrInvalid
	}
	writes.Lock()
	defer writes.Unlock()
	old, err := readSource(dir, strings.TrimSuffix(name, ".md"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	if err == nil {
		src := string(old)
		if strings.HasPrefix(src, "---\n") {
			if end := strings.Index(src[4:], "\n---\n"); end >= 0 {
				for _, line := range strings.Split(src[4:4+end], "\n") {
					k, v, _ := strings.Cut(line, ":")
					if k == "konro_edited" && strings.TrimSpace(v) == "true" {
						return false, nil
					}
				}
			}
		}
	}
	if err = atomicReplace(filepath.Join(dir, name), data); err != nil {
		return false, err
	}
	return true, nil
}

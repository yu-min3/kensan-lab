// Package projectwatch derives display summaries from existing project records.
// It never modifies human-authored project files or diaries.
package projectwatch

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Source struct {
	ID      string `json:"id"`
	Project string `json:"project"`
	Path    string `json:"path"`
	Date    string `json:"date"`
	Text    string `json:"text"`
	Hash    string `json:"hash"`
}
type Project struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Date string `json:"date"`
	Text string `json:"text"`
	Next string `json:"next"`
}
type Snapshot struct {
	Warnings []string  `json:"warnings,omitempty"`
	Hash     string    `json:"hash"`
	Sources  []Source  `json:"sources"`
	Projects []Project `json:"projects"`
}

var projectName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
var datedLog = regexp.MustCompile(`(?m)^-\s+(\d{4}-\d{2}-\d{2})(?:\s*\([^\n)]*\))?\s*[:：]\s*(.*)$`)
var nextStep = regexp.MustCompile(`(?m)^- \[ \] (.+)$`)

func digest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

// section ignores heading-like text inside fenced code blocks.
func section(text, name string) string {
	var lines []string
	active := false
	fence := ""
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			marker := trimmed[:3]
			if fence == "" {
				fence = marker
			} else if fence == marker {
				fence = ""
			}
			if active {
				lines = append(lines, line)
			}
			continue
		}
		if fence == "" && strings.HasPrefix(line, "## ") {
			if active {
				break
			}
			active = strings.TrimSpace(strings.TrimPrefix(line, "## ")) == name
			continue
		}
		if active {
			lines = append(lines, line)
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func readLimited(r *os.Root, path string, max int64) ([]byte, error) {
	f, e := r.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, max+1))
	if e != nil {
		return nil, e
	}
	if int64(len(b)) > max {
		return nil, fmt.Errorf("input too large: %s", path)
	}
	return b, nil
}

// Collect reads only direct project READMEs and the latest imported feed.
// Source text is bounded explicitly; excessive input is an error, never silently dropped.
func Collect(root string) (Snapshot, error) { return collect(root, true) }

func collect(root string, forAI bool) (Snapshot, error) {
	r, e := os.OpenRoot(root)
	if e != nil {
		return Snapshot{}, e
	}
	defer r.Close()
	dirs, e := r.Open("projects")
	if e != nil {
		return Snapshot{}, e
	}
	entries, e := dirs.ReadDir(-1)
	dirs.Close()
	if e != nil {
		return Snapshot{}, e
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	snap := Snapshot{Sources: []Source{}, Projects: []Project{}}
	for _, ent := range entries {
		name := ent.Name()
		if !ent.IsDir() || !projectName.MatchString(name) {
			continue
		}
		path := "projects/" + name + "/README.md"
		b, e := readLimited(r, path, 1024*1024)
		if e != nil {
			if os.IsNotExist(e) {
				continue
			}
			if forAI {
				return Snapshot{}, e
			}
			snap.Warnings = append(snap.Warnings, "記録を読み取れません: "+name)
			continue
		}
		text := strings.ReplaceAll(string(b), "\r\n", "\n")
		var meta struct {
			Status string `yaml:"status"`
		}
		if strings.HasPrefix(text, "---\n") {
			if end := strings.Index(text[4:], "\n---"); end >= 0 {
				if e = yaml.Unmarshal([]byte(text[4:4+end]), &meta); e != nil {
					if forAI {
						return Snapshot{}, fmt.Errorf("invalid frontmatter: %s", path)
					}
					snap.Warnings = append(snap.Warnings, "frontmatterを読み取れません: "+name)
					continue
				}
			}
		}
		if meta.Status != "active" {
			continue
		}
		logs := section(text, "ログ")
		matches := datedLog.FindAllStringSubmatchIndex(logs, -1)
		type log struct{ date, text string }
		ls := []log{}
		for i, m := range matches {
			date := logs[m[2]:m[3]]
			if _, e := time.Parse("2006-01-02", date); e != nil {
				continue
			}
			end := len(logs)
			if i+1 < len(matches) {
				end = matches[i+1][0]
			}
			ls = append(ls, log{date, strings.TrimSpace(logs[m[4]:end])})
		}
		sort.SliceStable(ls, func(i, j int) bool { return ls[i].date > ls[j].date })
		p := Project{Name: name, Path: path}
		if len(ls) > 0 {
			p.Date = ls[0].date
			p.Text = ls[0].text
		}
		milestones := section(text, "マイルストーン")
		if next := nextStep.FindStringSubmatch(milestones); len(next) > 1 {
			p.Next = next[1]
		}
		snap.Projects = append(snap.Projects, p)
		if len(ls) > 3 {
			ls = ls[:3]
		}
		evidence := section(text, "現在地") + "\n\n## マイルストーン\n" + milestones + "\n\n## 最近のログ\n"
		for _, l := range ls {
			evidence += l.date + ": " + l.text + "\n"
		}
		if len(evidence) > 16000 {
			if forAI {
				return Snapshot{}, fmt.Errorf("project context exceeds 16000 bytes: %s", name)
			}
			snap.Warnings = append(snap.Warnings, "AI入力上限を超えています: "+name)
		}
		snap.Sources = append(snap.Sources, Source{ID: "project:" + name, Project: name, Path: path, Date: p.Date, Text: evidence, Hash: digest([]byte(evidence))})
	}
	// Restrict feed traversal to YYYY/MM/DD.md. Generated JSON and state never enter the input.
	years, e := r.Open("feeds")
	if e == nil {
		ys, e := years.ReadDir(-1)
		years.Close()
		if e != nil {
			return Snapshot{}, e
		}
		latest := ""
		for _, y := range ys {
			if !y.IsDir() || !regexp.MustCompile(`^\d{4}$`).MatchString(y.Name()) {
				continue
			}
			d, e := r.Open("feeds/" + y.Name())
			if e != nil {
				return Snapshot{}, e
			}
			ms, e := d.ReadDir(-1)
			d.Close()
			if e != nil {
				return Snapshot{}, e
			}
			for _, m := range ms {
				if !m.IsDir() || !regexp.MustCompile(`^\d{2}$`).MatchString(m.Name()) {
					continue
				}
				prefix := "feeds/" + y.Name() + "/" + m.Name()
				d, e := r.Open(prefix)
				if e != nil {
					return Snapshot{}, e
				}
				ds, e := d.ReadDir(-1)
				d.Close()
				if e != nil {
					return Snapshot{}, e
				}
				for _, day := range ds {
					if !day.IsDir() && regexp.MustCompile(`^\d{2}\.md$`).MatchString(day.Name()) {
						p := prefix + "/" + day.Name()
						date := y.Name() + "-" + m.Name() + "-" + strings.TrimSuffix(day.Name(), ".md")
						if _, e := time.Parse("2006-01-02", date); e == nil && p > latest {
							latest = p
						}
					}
				}
			}
		}
		if latest != "" {
			limit := int64(32000)
			if !forAI {
				limit = 1024 * 1024
			}
			b, e := readLimited(r, latest, limit)
			if e != nil {
				if forAI {
					return Snapshot{}, e
				}
				snap.Warnings = append(snap.Warnings, "AI参照用フィードを読み取れません")
			} else {
				if !forAI && len(b) > 32000 {
					snap.Warnings = append(snap.Warnings, "フィードがAI入力上限を超えています")
				}
				snap.Sources = append(snap.Sources, Source{ID: "feed:latest", Path: latest, Text: string(b), Hash: digest(b)})
			}
		}
	} else if !os.IsNotExist(e) {
		return Snapshot{}, e
	}
	b, _ := json.Marshal(snap.Sources)
	if len(b) > 120000 {
		if forAI {
			return Snapshot{}, fmt.Errorf("combined context exceeds 120000 bytes")
		}
		snap.Warnings = append(snap.Warnings, "資料全体がAI入力上限を超えています")
	}
	snap.Hash = digest(b)
	sort.SliceStable(snap.Projects, func(i, j int) bool { return snap.Projects[i].Date > snap.Projects[j].Date })
	return snap, nil
}

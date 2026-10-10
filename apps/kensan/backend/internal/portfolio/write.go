package portfolio

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/yu-min3/kensan-lab/apps/kensan/backend/internal/workspace"
)

var (
	ErrMismatch = errors.New("proposal changed or already decided")
	inboxPathRe = regexp.MustCompile(`^reflect/inbox/\d{4}-\d{2}-\d{2}\.md$`)
)

// Decide は提案を採用（accept=true）か却下する。
// 採用は先に portfolio.md の該当マスへ証拠を足し、次に inbox の行を [x] にする。
// 途中で失敗して再送されても、同じ証拠が既にあれば足さないので二重にならない。
// portfolio.md と inbox の判定を書くのはアプリだけ（夜のジョブは新しい inbox を作るだけ）。
func Decide(ws *workspace.Workspace, file string, line int, text string, accept bool, now time.Time) (Proposal, error) {
	if !inboxPathRe.MatchString(file) {
		return Proposal{}, fmt.Errorf("invalid proposal file: %s", file)
	}
	var p Proposal
	err := ws.Mutate(file, func(content []byte, exists bool) ([]byte, error) {
		if !exists {
			return nil, ErrMismatch
		}
		lines := strings.Split(string(content), "\n")
		if line < 1 || line > len(lines) {
			return nil, ErrMismatch
		}
		got, ok := parseProposal(lines[line-1], file, line)
		if !ok || got.Text != strings.TrimSpace(text) {
			return nil, ErrMismatch
		}
		p = got
		want := map[bool]string{true: "accepted", false: "rejected"}[accept]
		if got.State == want {
			return nil, nil // 再送
		}
		if got.State != "todo" {
			return nil, ErrMismatch
		}
		return nil, nil
	})
	if err != nil {
		return Proposal{}, err
	}
	if accept {
		if err := appendEvidence(ws, p, now); err != nil {
			return Proposal{}, err
		}
	}
	mark := map[bool]string{true: "x", false: "-"}[accept]
	err = ws.Mutate(file, func(content []byte, exists bool) ([]byte, error) {
		lines := strings.Split(string(content), "\n")
		if line < 1 || line > len(lines) {
			return nil, ErrMismatch
		}
		cur := lines[line-1]
		i := strings.Index(cur, "- [")
		if i < 0 || len(cur) < i+5 {
			return nil, ErrMismatch
		}
		if cur[i+3:i+4] == mark {
			return nil, nil
		}
		lines[line-1] = cur[:i+3] + mark + cur[i+4:]
		return workspace.TouchUpdated([]byte(strings.Join(lines, "\n")), now), nil
	})
	if err != nil {
		return Proposal{}, err
	}
	p.State = map[bool]string{true: "accepted", false: "rejected"}[accept]
	return p, nil
}

// evidenceLine は採用した提案を portfolio.md の 1 行にする。@public は付けない
// （外から見えるかは Yu が後で判断して付ける）。
func evidenceLine(p Proposal) string {
	b := p.Body
	s := "- " + b.When + " " + b.Title
	if b.Story != "" || b.Impact != "" {
		s += " — " + b.Story
		if b.Impact != "" {
			s += " → " + b.Impact
		}
	}
	if b.Project != "" {
		s += " @project(" + b.Project + ")"
	}
	return s
}

func appendEvidence(ws *workspace.Workspace, p Proposal, now time.Time) error {
	var rungName string
	for _, r := range Rungs {
		if r.ID == p.Rung {
			rungName = r.Name
		}
	}
	heading := "## " + rungName + " × " + p.Domain
	entry := evidenceLine(p)
	return ws.Mutate(File, func(content []byte, exists bool) ([]byte, error) {
		src := string(content)
		if !exists {
			return nil, fmt.Errorf("%s not found", File)
		}
		if strings.Contains(src, entry) {
			return nil, nil // 既に足してある（再送）
		}
		lines := strings.Split(strings.TrimRight(src, "\n"), "\n")
		start := -1
		for i, l := range lines {
			if strings.TrimSpace(l) == heading {
				start = i
				break
			}
		}
		if start == -1 {
			return workspace.TouchUpdated([]byte(strings.Join(lines, "\n")+"\n\n"+heading+"\n\n"+entry+"\n"), now), nil
		}
		end := len(lines)
		for i := start + 1; i < len(lines); i++ {
			if strings.HasPrefix(lines[i], "## ") {
				end = i
				break
			}
		}
		insert := end
		for insert > start+1 && strings.TrimSpace(lines[insert-1]) == "" {
			insert--
		}
		add := []string{entry}
		if insert == start+1 {
			add = []string{"", entry}
		}
		if insert < len(lines) && strings.TrimSpace(lines[insert]) != "" {
			add = append(add, "")
		}
		out := append(append([]string{}, lines[:insert]...), add...)
		out = append(out, lines[insert:]...)
		return workspace.TouchUpdated([]byte(strings.Join(out, "\n")+"\n"), now), nil
	})
}

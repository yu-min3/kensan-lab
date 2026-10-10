// Package diary は今日画面から日記を 1 行で残す経路と、記入の有無を返す。
// 保存先は conventions.md の daily（daily/YYYY/MM/DD.md の ## 日記）。別ファイルや DB は持たない。
package diary

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/yu-min3/kensan-lab/apps/kensan/backend/internal/workspace"
)

const heading = "## 日記"

// MaxLine は 1 行日記の上限（rune 数）。長文は日記ページで書く。
const MaxLine = 500

var (
	ErrEmpty   = errors.New("日記が空です")
	ErrTooLong = errors.New("日記が長すぎます")
)

func Path(date time.Time) string {
	return fmt.Sprintf("daily/%04d/%02d/%02d.md", date.Year(), date.Month(), date.Day())
}

// Normalize は改行をつぶして 1 行にする。空や上限超えはエラー。
func Normalize(text string) (string, error) {
	line := strings.Join(strings.Fields(text), " ")
	if line == "" {
		return "", ErrEmpty
	}
	if n := len([]rune(line)); n > MaxLine {
		return "", fmt.Errorf("%w: 1 行 %d 文字まで（%d 文字）。長文は日記ページで書く", ErrTooLong, MaxLine, n)
	}
	return line, nil
}

// AppendLine は ## 日記 の末尾へ「- HH:MM 本文」を足す。
// 見出しが無ければ末尾に ## 日記 を作る。他の節（学習・みのりちゃんへ 等）には触らない。
func AppendLine(content string, at time.Time, line string) string {
	entry := fmt.Sprintf("- %s %s", at.Format("15:04"), line)
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	start := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == heading {
			start = i
			break
		}
	}
	if start == -1 {
		return strings.Join(lines, "\n") + "\n\n" + heading + "\n\n" + entry + "\n"
	}
	// 本文の終わりは次の見出し。### 完了タスク（reflection の退避先）より前に入れる。
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "#") {
			end = i
			break
		}
	}
	// 節末の空行の手前に入れる。節が見出しだけなら空行を 1 つ挟む。
	insert := end
	for insert > start+1 && strings.TrimSpace(lines[insert-1]) == "" {
		insert--
	}
	add := []string{entry}
	if insert == start+1 {
		add = []string{"", entry}
	}
	out := append([]string{}, lines[:insert]...)
	out = append(out, add...)
	rest := lines[insert:]
	if len(rest) > 0 && strings.TrimSpace(rest[0]) != "" {
		out = append(out, "")
	}
	out = append(out, rest...)
	return strings.Join(out, "\n") + "\n"
}

// Add は date の daily に 1 行日記を足す。ファイルが無ければ規約どおりの骨組みから作る。
func Add(ws *workspace.Workspace, date, at time.Time, text string) (string, error) {
	line, err := Normalize(text)
	if err != nil {
		return "", err
	}
	rel := Path(date)
	err = ws.Mutate(rel, func(content []byte, exists bool) ([]byte, error) {
		src := string(content)
		if !exists {
			src = workspace.DailySkeleton(date)
		}
		return workspace.TouchUpdated([]byte(AppendLine(src, at, line)), at), nil
	})
	return rel, err
}

// Day は 1 日分の記入の有無。
type Day struct {
	Date    string `json:"date"`
	Written bool   `json:"written"`
}

// Summary は今日画面に出す事実だけ（最終記入日と直近の記入）。評価語は持たない。
type Summary struct {
	Last string `json:"last,omitempty"`
	Days []Day  `json:"days"`
}

var dailyFile = regexp.MustCompile(`^(\d{4})/(\d{2})/(\d{2})\.md$`)

// Recent は直近 days 日の記入と、全期間の最終記入日を返す。
// 「書いた」は ## 日記 に本文があること（骨組みだけのファイルは数えない）。
func Recent(root string, today time.Time, days int) (Summary, error) {
	s := Summary{Days: []Day{}}
	written := map[string]bool{}
	base := filepath.Join(root, "daily")
	err := filepath.WalkDir(base, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return filepath.SkipDir
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(base, p)
		m := dailyFile.FindStringSubmatch(filepath.ToSlash(rel))
		if m == nil {
			return nil
		}
		content, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if HasEntry(string(content)) {
			date := m[1] + "-" + m[2] + "-" + m[3]
			written[date] = true
			if date > s.Last {
				s.Last = date
			}
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return s, err
	}
	for i := days - 1; i >= 0; i-- {
		date := today.AddDate(0, 0, -i).Format("2006-01-02")
		s.Days = append(s.Days, Day{Date: date, Written: written[date]})
	}
	return s, nil
}

// HasEntry は ## 日記 の節に Yu が書いた本文があるか。
// 見出しだけの行と、reflection がタスクを退避する ### 完了タスク の中身は記入に数えない。
func HasEntry(content string) bool {
	in, skip := false, false
	for _, l := range strings.Split(content, "\n") {
		t := strings.TrimSpace(l)
		if t == heading {
			in, skip = true, false
			continue
		}
		if !in {
			continue
		}
		if strings.HasPrefix(l, "# ") || strings.HasPrefix(l, "## ") {
			return false
		}
		if strings.HasPrefix(t, "#") {
			skip = strings.TrimSpace(strings.TrimLeft(t, "#")) == "完了タスク"
			continue
		}
		if !skip && t != "" {
			return true
		}
	}
	return false
}

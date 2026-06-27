// Package goals は goals.md から North Star と今期のフォーカスを抽出する。
//
// ファイル契約（conventions.md / goals.md）:
//
//	goals.md  ## North Star            ← 北極星（1 文）
//	goals.md  ## 今期のフォーカス（...） ← 今期の重点（番号付きリスト）
//
// 月次ゴール（## 今月のゴール）はダッシュボードには出さない（D5）。
package goals

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	headingRe   = regexp.MustCompile(`^(#{1,6})\s+(.+?)\s*$`)
	focusItemRe = regexp.MustCompile(`^\s*\d+\.\s+(.+)$`)
)

// Focus は今期のフォーカス 1 項目。Title（太字部）と Detail（補足）に分ける。
type Focus struct {
	Title  string `json:"title"`
	Detail string `json:"detail"`
}

// Goals はダッシュボード上部に出す目標サマリ。
type Goals struct {
	NorthStar string  `json:"northStar"`
	Focus     []Focus `json:"focus"`
}

// Load は workspace ルートの goals.md を読んでパースする。ファイルが無ければ空を返す。
func Load(root string) (Goals, error) {
	content, err := os.ReadFile(filepath.Join(root, "goals.md"))
	if err != nil {
		if os.IsNotExist(err) {
			return Goals{}, nil
		}
		return Goals{}, err
	}
	return Parse(string(content)), nil
}

// Parse は goals.md 本文から North Star と今期のフォーカスを抽出する。
func Parse(content string) Goals {
	var g Goals
	var northStarLines []string
	section := ""
	for _, line := range strings.Split(content, "\n") {
		if h := headingRe.FindStringSubmatch(line); h != nil {
			section = h[2]
			continue
		}
		switch {
		case section == "North Star":
			if t := strings.TrimSpace(line); t != "" {
				northStarLines = append(northStarLines, stripQuotes(t))
			}
		case strings.HasPrefix(section, "今期のフォーカス"):
			if m := focusItemRe.FindStringSubmatch(line); m != nil {
				g.Focus = append(g.Focus, splitFocus(m[1]))
			}
		}
	}
	g.NorthStar = strings.Join(northStarLines, " ")
	return g
}

// splitFocus は「**タイトル** — 補足」を Title/Detail に分ける。
// 区切りは em dash（—）優先、なければ ' - '。太字マーカーは落とす。
func splitFocus(item string) Focus {
	item = strings.TrimSpace(item)
	sep, sepLen := -1, 0
	if i := strings.Index(item, "—"); i >= 0 {
		sep, sepLen = i, len("—")
	} else if i := strings.Index(item, " - "); i >= 0 {
		sep, sepLen = i, len(" - ")
	}
	title, detail := item, ""
	if sep >= 0 {
		title = strings.TrimSpace(item[:sep])
		detail = strings.TrimSpace(item[sep+sepLen:])
	}
	return Focus{Title: stripBold(title), Detail: detail}
}

func stripBold(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "**")
	s = strings.TrimSuffix(s, "**")
	return strings.TrimSpace(s)
}

// 「『...』」のような引用符で囲まれた North Star も素のテキストにする。
func stripQuotes(s string) string {
	for _, q := range [][2]string{{"「", "」"}, {"『", "』"}, {`"`, `"`}} {
		if strings.HasPrefix(s, q[0]) && strings.HasSuffix(s, q[1]) {
			return strings.TrimSpace(s[len(q[0]) : len(s)-len(q[1])])
		}
	}
	return s
}

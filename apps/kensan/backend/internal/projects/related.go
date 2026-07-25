package projects

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// RelatedItem は project に紐づく 1 リソース。
// 出どころ（Kind）を持たせて、自動収集と手書きを画面で区別できるようにする。
type RelatedItem struct {
	Kind   string `json:"kind"` // docs | records | articles | note | project | external
	Label  string `json:"label"`
	Target string `json:"target,omitempty"` // app で開ける .md パス
	URL    string `json:"url,omitempty"`    // 外部リンク
	Desc   string `json:"desc,omitempty"`
}

// autoDirs は project ディレクトリ内で自動収集する固定語彙（conventions の段階制と同じ）。
var autoDirs = []string{"docs", "records", "articles"}

// collectProjectDocs は projects/<name>/{docs,records,articles}/*.md を集める。
// README に手で列挙する必要をなくすのが目的（ファイルシステムの書き写しを排除する）。
func collectProjectDocs(root, name string) []RelatedItem {
	var out []RelatedItem
	// project 直下の平置き md（規約上、小規模 project は分割せず直下に置いてよい）
	if entries, err := os.ReadDir(filepath.Join(root, "projects", name)); err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") || e.Name() == "README.md" {
				continue
			}
			out = append(out, RelatedItem{
				Kind:   "docs",
				Label:  docTitle(filepath.Join(root, "projects", name, e.Name()), e.Name()),
				Target: filepath.ToSlash(filepath.Join("projects", name, e.Name())),
			})
		}
	}
	for _, dir := range autoDirs {
		base := filepath.Join(root, "projects", name, dir)
		entries, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			rel := filepath.ToSlash(filepath.Join("projects", name, dir, e.Name()))
			out = append(out, RelatedItem{
				Kind:   dir,
				Label:  docTitle(filepath.Join(base, e.Name()), e.Name()),
				Target: rel,
			})
		}
	}
	// records は日付ファイル名なので新しい順、それ以外は名前順に寄せる
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		if out[i].Kind == "records" {
			return out[i].Target > out[j].Target
		}
		return out[i].Label < out[j].Label
	})
	return out
}

// docTitle は frontmatter の title を優先し、無ければファイル名（拡張子なし）。
func docTitle(path, fallback string) string {
	b, err := os.ReadFile(path)
	if err == nil {
		if t := frontmatter(string(b))["title"]; t != "" {
			return strings.Trim(t, `"'`)
		}
	}
	return strings.TrimSuffix(fallback, ".md")
}

// manualRelated は README の ## 関連ノート・リソース を RelatedItem に変換する。
// 自動収集で拾える project 内 md は除外する（同じものが 2 回出るのを防ぐ）。
func manualRelated(notes []NoteRef, auto []RelatedItem) []RelatedItem {
	covered := map[string]bool{}
	for _, a := range auto {
		covered[a.Target] = true
	}
	var out []RelatedItem
	for _, n := range notes {
		if n.Target != "" && covered[n.Target] {
			continue
		}
		item := RelatedItem{Label: n.Label, Desc: n.Desc, Target: n.Target}
		switch {
		case n.Target == "" && strings.Contains(n.Label, "http"):
			item.Kind = "external"
			item.URL = extractURL(n.Label)
		case strings.HasPrefix(n.Target, "projects/") && strings.HasSuffix(n.Target, "/README.md"):
			item.Kind = "project"
		case n.Target != "":
			item.Kind = "note"
		default:
			item.Kind = "external"
		}
		out = append(out, item)
	}
	return out
}

func extractURL(s string) string {
	i := strings.Index(s, "http")
	if i < 0 {
		return ""
	}
	return strings.Fields(s[i:])[0]
}

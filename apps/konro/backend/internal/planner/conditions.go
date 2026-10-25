package planner

import (
	"errors"
	"fmt"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/ingredients"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/recipe"
	"golang.org/x/text/unicode/norm"
	"math/big"
	"regexp"
	"sort"
	"strings"
)

var ErrInvalid = errors.New("提案条件を確認してください")
var ErrConflict = errors.New("提案が更新されています。最新の内容を確認してください")

type Conditions struct {
	Idea     string   `json:"idea"`
	Count    int      `json:"count"`
	Servings string   `json:"servings"`
	Main     int      `json:"main"`
	Side     int      `json:"side"`
	Cuisine  string   `json:"cuisine,omitempty"`
	Avoid    []string `json:"avoid"`
	Use      []string `json:"use"`
	Fixed    []string `json:"fixed"`
	Omit     []string `json:"omit"`
	Seed     int      `json:"seed"`
}
type Candidate struct {
	Recipe recipe.Recipe `json:"recipe"`
	Base   string        `json:"base"`
	Scale  string        `json:"scale"`
}
type Pick struct {
	File   string `json:"file"`
	Reason string `json:"reason"`
}
type Menu struct {
	ID    string `json:"id"`
	Picks []Pick `json:"picks"`
}

var servingPattern = regexp.MustCompile(`^(\d+(?:\.\d+)?)\s*(?:人分|人前|servings?)?$`)

func BaseServings(raw string) string {
	m := servingPattern.FindStringSubmatch(strings.TrimSpace(norm.NFKC.String(raw)))
	if m == nil {
		return ""
	}
	r, ok := new(big.Rat).SetString(m[1])
	if !ok || r.Sign() <= 0 {
		return ""
	}
	return r.RatString()
}
func normalized(s string) string { return strings.ToLower(norm.NFKC.String(s)) }
func contains(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}
func Valid(c Conditions) error {
	target, ok := new(big.Rat).SetString(c.Servings)
	if !ok || target.Sign() <= 0 || target.Cmp(big.NewRat(100, 1)) > 0 {
		return ErrInvalid
	}
	if c.Count < 1 || c.Count > 12 || len(c.Idea) > 4000 || c.Main < 0 || c.Side < 0 || ((c.Main+c.Side) > 0 && c.Main+c.Side != c.Count) || len(c.Fixed) > c.Count || len(c.Avoid) > 30 || len(c.Use) > 30 || len(c.Omit) > 100 || len(c.Cuisine) > 200 {
		return ErrInvalid
	}
	if _, e := ingredients.Multiply(c.Servings, "1"); e != nil || strings.HasPrefix(c.Servings, "-") || c.Servings == "0" {
		return ErrInvalid
	}
	for _, v := range append(append([]string{}, c.Avoid...), c.Use...) {
		if strings.TrimSpace(v) == "" || len(v) > 200 {
			return ErrInvalid
		}
	}
	return nil
}
func Candidates(root string, c Conditions) ([]Candidate, error) {
	if e := Valid(c); e != nil {
		return nil, e
	}
	metas, e := recipe.List(root)
	if e != nil {
		return nil, e
	}
	result := []Candidate{}
	for _, meta := range metas {
		if contains(c.Omit, meta.File) {
			continue
		}
		r, e := recipe.Load(root, meta.File)
		if e != nil {
			return nil, e
		}
		if len(r.Ingredients) == 0 || len(r.Steps) == 0 {
			continue
		}
		base := BaseServings(r.Servings)
		if base == "" {
			continue
		}
		if c.Cuisine != "" && !contains(r.Tags, c.Cuisine) {
			continue
		}
		excluded := false
		for _, line := range r.Ingredients {
			parsed := ingredients.Parse(line)
			if len(c.Avoid) > 0 && parsed.Kind == "unparsed" {
				excluded = true
			}
			for _, name := range c.Avoid {
				if strings.Contains(normalized(line), normalized(name)) {
					excluded = true
				}
			}
		}
		if excluded {
			continue
		}
		a, _ := new(big.Rat).SetString(c.Servings)
		b, _ := new(big.Rat).SetString(base)
		scale := new(big.Rat).Quo(a, b).RatString()
		if _, e := ingredients.Multiply("1", scale); e != nil {
			continue
		}
		result = append(result, Candidate{*r, base, scale})
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Recipe.Rating != result[j].Recipe.Rating {
			return result[i].Recipe.Rating > result[j].Recipe.Rating
		}
		return result[i].Recipe.File < result[j].Recipe.File
	})
	existing := map[string]bool{}
	for _, r := range result {
		existing[r.Recipe.File] = true
	}
	seen := map[string]bool{}
	for _, file := range c.Fixed {
		if seen[file] || !existing[file] {
			return nil, fmt.Errorf("%w: 固定した料理が条件に合わないか人数未登録です", ErrInvalid)
		}
		seen[file] = true
	}
	return result, nil
}
func ValidateMenus(c Conditions, candidates []Candidate, menus []Menu) ([]Menu, error) {
	if len(menus) == 0 || len(menus) > 3 {
		return nil, ErrInvalid
	}
	byFile := map[string]Candidate{}
	for _, candidate := range candidates {
		byFile[candidate.Recipe.File] = candidate
	}
	setKeys := map[string]bool{}
	for index := range menus {
		menu := &menus[index]
		if len(menu.Picks) != c.Count {
			return nil, fmt.Errorf("%w: 品数が一致しません", ErrInvalid)
		}
		seen := map[string]bool{}
		main, side := 0, 0
		keys := []string{}
		for i, pick := range menu.Picks {
			candidate, ok := byFile[pick.File]
			if !ok || seen[pick.File] {
				return nil, fmt.Errorf("%w: 不明または重複した料理", ErrInvalid)
			}
			seen[pick.File] = true
			keys = append(keys, pick.File)
			isMain := contains(candidate.Recipe.Tags, "メイン") || contains(candidate.Recipe.Tags, "主菜")
			isSide := contains(candidate.Recipe.Tags, "副菜")
			if c.Main+c.Side > 0 && isMain == isSide {
				return nil, fmt.Errorf("%w: 主菜・副菜の分類が不明です", ErrInvalid)
			}
			if contains(candidate.Recipe.Tags, "メイン") || contains(candidate.Recipe.Tags, "主菜") {
				main++
			}
			if contains(candidate.Recipe.Tags, "副菜") {
				side++
			}
			menu.Picks[i].Reason = fmt.Sprintf("%s人分に換算 · 自分の評価 %d/5", ingredients.Display(c.Servings), candidate.Recipe.Rating)
			if candidate.Recipe.Rating == 0 {
				menu.Picks[i].Reason = ingredients.Display(c.Servings) + "人分に換算 · 未評価"
			}
		}
		for _, file := range c.Fixed {
			if !seen[file] {
				return nil, fmt.Errorf("%w: 固定した料理が抜けています", ErrInvalid)
			}
		}
		if c.Main+c.Side > 0 && (main != c.Main || side != c.Side) {
			return nil, fmt.Errorf("%w: 主菜・副菜の内訳が一致しません", ErrInvalid)
		}
		sort.Strings(keys)
		key := strings.Join(keys, "\x00")
		if setKeys[key] {
			return nil, fmt.Errorf("%w: 同じ献立が重複しています", ErrInvalid)
		}
		setKeys[key] = true
		menu.ID = fmt.Sprintf("menu-%d", index+1)
	}
	return menus, nil
}

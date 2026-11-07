package planner

import (
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/recipe"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHardConditions(t *testing.T) {
	c := Conditions{Count: 2, Servings: "4", Main: 1, Side: 1, Fixed: []string{"main"}}
	candidates := []Candidate{{Recipe: recipe.Recipe{Meta: recipe.Meta{File: "main", Tags: []string{"メイン"}}}}, {Recipe: recipe.Recipe{Meta: recipe.Meta{File: "side", Tags: []string{"副菜"}}}}}
	if _, e := ValidateMenus(c, candidates, []Menu{{Picks: []Pick{{File: "main"}, {File: "side"}}}}); e != nil {
		t.Fatal(e)
	}
	for _, files := range [][]Pick{{{File: "main"}, {File: "missing"}}, {{File: "side"}, {File: "side"}}, {{File: "side"}}} {
		if _, e := ValidateMenus(c, candidates, []Menu{{Picks: files}}); e == nil {
			t.Fatal(files)
		}
	}
	if BaseServings("3〜4人分") != "" || BaseServings("２人分") != "2" {
		t.Fatal("ambiguous servings")
	}
}

func TestAmbiguousRolesCannotSatisfySplit(t *testing.T) {
	c := Conditions{Count: 2, Servings: "2", Main: 1, Side: 1}
	candidates := []Candidate{{Recipe: recipe.Recipe{Meta: recipe.Meta{File: "both", Tags: []string{"メイン", "副菜"}}}}, {Recipe: recipe.Recipe{Meta: recipe.Meta{File: "neither"}}}}
	if _, err := ValidateMenus(c, candidates, []Menu{{Picks: []Pick{{File: "both"}, {File: "neither"}}}}); err == nil {
		t.Fatal("ambiguous classification accepted")
	}
}

func TestAvoidMaterialAliasesInBothDirections(t *testing.T) {
	for _, pair := range [][2]string{{"玉ねぎ", "たまねぎ"}, {"たまねぎ", "玉ねぎ"}, {"玉ねぎ", "玉葱"}, {"玉葱", "玉ねぎ"}, {"醤油", "しょうゆ"}, {"しょうゆ", "醤油"}} {
		t.Run(pair[0]+"-"+pair[1], func(t *testing.T) {
			root := t.TempDir()
			raw := "---\ntitle: 別名材料\nservings: 2人分\n---\n\n## 材料\n- " + pair[1] + " 1個\n\n## 手順\n1. 混ぜる\n"
			if err := os.WriteFile(filepath.Join(root, "alias.md"), []byte(raw), 0600); err != nil {
				t.Fatal(err)
			}
			candidates, err := Candidates(root, Conditions{Count: 1, Servings: "2", Avoid: []string{pair[0]}})
			if err != nil || len(candidates) != 0 {
				t.Fatal("excluded alias remained", candidates, err)
			}
		})
	}
}

func TestCountOnlyPreservesRecipeAmounts(t *testing.T) {
	root := t.TempDir()
	for name, servings := range map[string]string{"two": "2人分", "six": "6人分", "unknown": "", "range": "3〜4人分"} {
		raw := "---\ntitle: " + name + "\nservings: " + servings + "\n---\n\n## 材料\n- にんじん 1本\n\n## 手順\n1. 炒める\n"
		if err := os.WriteFile(filepath.Join(root, name+".md"), []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
	}
	c := Conditions{Count: 4, Fixed: []string{"unknown"}}
	candidates, err := Candidates(root, c)
	if err != nil || len(candidates) != 4 {
		t.Fatalf("count-only candidates: %+v %v", candidates, err)
	}
	picks := []Pick{}
	for _, candidate := range candidates {
		if candidate.Scale != "1" {
			t.Fatalf("unexpected shared amount: %+v", candidate)
		}
		picks = append(picks, Pick{File: candidate.Recipe.File})
	}
	menus, err := ValidateMenus(c, candidates, []Menu{{Picks: picks}})
	if err != nil {
		t.Fatal(err)
	}
	for _, pick := range menus[0].Picks {
		if !strings.Contains(pick.Reason, "レシピの元の分量") {
			t.Fatal(pick.Reason)
		}
	}
	legacy, err := Candidates(root, Conditions{Count: 2, Servings: "4"})
	if err != nil || len(legacy) != 2 {
		t.Fatal(legacy, err)
	}
	for _, candidate := range legacy {
		want := map[string]string{"two": "2", "six": "2/3"}[candidate.Recipe.File]
		if candidate.Scale != want {
			t.Fatalf("legacy scale changed: %+v", candidate)
		}
	}
}

func TestOptionalServingsStillValidatesExplicitValues(t *testing.T) {
	for _, value := range []string{"0", "-1", "101", "no", "1/0"} {
		if Valid(Conditions{Count: 4, Servings: value}) == nil {
			t.Fatal("invalid servings accepted", value)
		}
	}
	if err := Valid(Conditions{Count: 4}); err != nil {
		t.Fatal(err)
	}
}

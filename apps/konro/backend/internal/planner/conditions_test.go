package planner

import (
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/recipe"
	"os"
	"path/filepath"
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

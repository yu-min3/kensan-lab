package planner

import (
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/recipe"
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

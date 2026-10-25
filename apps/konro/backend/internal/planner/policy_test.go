package planner

import (
	"errors"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/mealplan"
	"path/filepath"
	"testing"
)

func TestPolicyComparisonAndRollback(t *testing.T) {
	root := fixture(t)
	s := New(root, nil, mealplan.New(root))
	trial, e := s.SavePolicy(Policy{RatingWeight: 2, UseWeight: 3, DiversityWeight: 1, Note: "候補"})
	if e != nil {
		t.Fatal(e)
	}
	c := Conditions{Count: 2, Servings: "4"}
	comparison, e := s.Compare(trial.Version, c)
	if e != nil {
		t.Fatal(e)
	}
	if comparison.Violations != 0 || comparison.Current.Version != "baseline-v1" {
		t.Fatal(comparison)
	}
	if _, e = s.Activate(trial.Version, comparison.ID, "stale"); e == nil {
		t.Fatal("stale activation")
	}
	if _, e = s.Activate(trial.Version, comparison.ID, "baseline-v1"); e != nil {
		t.Fatal(e)
	}
	back, e := s.Compare("baseline-v1", c)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Activate("baseline-v1", back.ID, trial.Version); e != nil {
		t.Fatal(e)
	}
}

func TestComparisonRejectsChangedPreferences(t *testing.T) {
	root := fixture(t)
	s := New(root, nil, mealplan.New(root))
	trial, err := s.SavePolicy(Policy{RatingWeight: 2})
	if err != nil {
		t.Fatal(err)
	}
	comparison, err := s.Compare(trial.Version, Conditions{Count: 1, Servings: "2"})
	if err != nil {
		t.Fatal(err)
	}
	if err = AtomicJSON(filepath.Join(root, ".settings", "meal-preferences.json"), map[string]any{"revision": 1, "notes": "更新"}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Activate(trial.Version, comparison.ID, "baseline-v1"); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
}

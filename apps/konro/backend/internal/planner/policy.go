package planner

import (
	"encoding/json"
	"errors"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/ingredients"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/mealfeedback"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Policy struct {
	Version         string  `json:"version"`
	RatingWeight    float64 `json:"ratingWeight"`
	UseWeight       float64 `json:"useWeight"`
	DiversityWeight float64 `json:"diversityWeight"`
	Note            string  `json:"note"`
}
type Comparison struct {
	PreferenceVersion string      `json:"preferenceVersion"`
	ID                string      `json:"id"`
	Conditions        Conditions  `json:"conditions"`
	Candidates        []Candidate `json:"candidates"`
	Current           Policy      `json:"current"`
	Trial             Policy      `json:"trial"`
	CurrentMenus      []Menu      `json:"currentMenus"`
	TrialMenus        []Menu      `json:"trialMenus"`
	Violations        int         `json:"violations"`
	FeedbackCount     int         `json:"feedbackCount"`
	CreatedAt         string      `json:"createdAt"`
}

func DefaultPolicy() Policy {
	return Policy{Version: "baseline-v1", RatingWeight: 1, UseWeight: 2, DiversityWeight: 1, Note: "初期値。改善効果は未評価。"}
}
func validPolicy(p Policy) bool {
	if len(p.Note) > 4000 {
		return false
	}
	for _, v := range []float64{p.RatingWeight, p.UseWeight, p.DiversityWeight} {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 20 {
			return false
		}
	}
	return true
}
func (s *Service) currentPolicy() (Policy, error) {
	var p Policy
	e := ReadJSON(filepath.Join(s.root, "planning-policy.json"), &p)
	if errors.Is(e, os.ErrNotExist) {
		return DefaultPolicy(), nil
	}
	if e != nil {
		return p, e
	}
	if !validPolicy(p) {
		return p, ErrInvalid
	}
	return p, nil
}
func (s *Service) policy(version string) (Policy, error) {
	if version == "baseline-v1" {
		return DefaultPolicy(), nil
	}
	if !idPattern.MatchString(version) {
		return Policy{}, ErrInvalid
	}
	var p Policy
	e := ReadJSON(filepath.Join(s.root, "planning-policies", version+".json"), &p)
	if e == nil && (!validPolicy(p) || p.Version != version) {
		return p, ErrInvalid
	}
	return p, e
}
func (s *Service) SavePolicy(p Policy) (Policy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !validPolicy(p) {
		return p, ErrInvalid
	}
	p.Version = ""
	p.Version = Digest(p)[:32]
	if e := AtomicJSON(filepath.Join(s.root, "planning-policies", p.Version+".json"), p); e != nil {
		return p, e
	}
	return p, nil
}
func (s *Service) Policies() (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, e := s.currentPolicy()
	if e != nil {
		return nil, e
	}
	policies := []Policy{DefaultPolicy()}
	entries, e := os.ReadDir(filepath.Join(s.root, "planning-policies"))
	if e != nil && !errors.Is(e, os.ErrNotExist) {
		return nil, e
	}
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		p, e := s.policy(strings.TrimSuffix(entry.Name(), ".json"))
		if e != nil {
			return nil, e
		}
		policies = append(policies, p)
	}
	return map[string]any{"current": current, "candidates": policies}, nil
}
func Rank(c Conditions, candidates []Candidate, p Policy) []Candidate {
	result := append([]Candidate{}, candidates...)
	frequency := map[string]int{}
	main := func(v Candidate) string {
		for _, line := range v.Recipe.Ingredients {
			parsed := ingredients.Parse(line)
			if parsed.Kind != "group" {
				return parsed.Name
			}
		}
		return ""
	}
	for _, v := range result {
		frequency[main(v)]++
	}
	score := func(v Candidate) float64 {
		value := float64(v.Recipe.Rating) * p.RatingWeight
		for _, preferred := range c.Use {
			for _, line := range v.Recipe.Ingredients {
				if materialMatches(line, preferred) {
					value += p.UseWeight
					break
				}
			}
		}
		if n := frequency[main(v)]; n > 0 {
			value += p.DiversityWeight / float64(n)
		}
		return value
	}
	sort.SliceStable(result, func(i, j int) bool { return score(result[i]) > score(result[j]) })
	return result
}
func (s *Service) Compare(version string, c Conditions) (Comparison, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	trial, e := s.policy(version)
	if e != nil {
		return Comparison{}, e
	}
	current, e := s.currentPolicy()
	if e != nil {
		return Comparison{}, e
	}
	preferences, pv, e := OptionalJSON(filepath.Join(s.root, ".settings", "meal-preferences.json"))
	if e != nil {
		return Comparison{}, e
	}
	b, _ := json.Marshal(preferences)
	var explicit mealfeedback.Preferences
	if json.Unmarshal(b, &explicit) != nil {
		return Comparison{}, ErrInvalid
	}
	c.Avoid = Unique(append(c.Avoid, explicit.Avoid...))
	c.Use = Unique(append(c.Use, explicit.Use...))
	candidates, e := Candidates(s.root, c)
	if e != nil {
		return Comparison{}, e
	}
	baseline, e := RuleMenus(c, Rank(c, candidates, current))
	if e != nil {
		return Comparison{}, e
	}
	newMenus, e := RuleMenus(c, Rank(c, candidates, trial))
	if e != nil {
		return Comparison{}, e
	}
	events, e := s.feedback.Events()
	if e != nil {
		return Comparison{}, e
	}
	validCount := 0
	for _, event := range events {
		if !event.Cancelled {
			validCount++
		}
	}
	comparison := Comparison{PreferenceVersion: pv, Conditions: c, Candidates: candidates, Current: current, Trial: trial, CurrentMenus: baseline, TrialMenus: newMenus, Violations: 0, FeedbackCount: validCount, CreatedAt: time.Now().UTC().Format(time.RFC3339)}
	comparison.ID = Digest([]any{c, candidates, current, trial, pv})[:32]
	return comparison, AtomicJSON(filepath.Join(s.root, "planning-evaluations", comparison.ID+".json"), comparison)
}
func (s *Service) Activate(version, comparisonID, expected string) (Policy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !idPattern.MatchString(comparisonID) {
		return Policy{}, ErrInvalid
	}
	current, e := s.currentPolicy()
	if e != nil {
		return current, e
	}
	if current.Version != expected {
		return current, ErrConflict
	}
	var comparison Comparison
	if e = ReadJSON(filepath.Join(s.root, "planning-evaluations", comparisonID+".json"), &comparison); e != nil {
		return current, e
	}
	if comparison.Trial.Version != version || comparison.Current.Version != expected || comparison.Violations != 0 {
		return current, ErrConflict
	}
	_, pv, e := OptionalJSON(filepath.Join(s.root, ".settings", "meal-preferences.json"))
	if e != nil {
		return current, e
	}
	if comparison.PreferenceVersion != pv {
		return current, ErrConflict
	}
	p, e := s.policy(version)
	if e != nil {
		return current, e
	}
	if e = AtomicJSON(filepath.Join(s.root, "planning-policy.json"), p); e != nil {
		return current, e
	}
	return p, nil
}

package planner

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/agentclient"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/mealfeedback"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/mealplan"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

type Proposal struct {
	Schema            int               `json:"schema"`
	ID                string            `json:"id"`
	Revision          int               `json:"revision"`
	Conditions        Conditions        `json:"conditions"`
	Candidates        []Candidate       `json:"candidates"`
	Mode              string            `json:"mode"`
	Pending           bool              `json:"pending"`
	Job               *agentclient.Job  `json:"job,omitempty"`
	Menus             []Menu            `json:"menus"`
	Interpretation    string            `json:"interpretation"`
	Error             string            `json:"error,omitempty"`
	State             string            `json:"state"`
	Repairs           int               `json:"repairs"`
	PreferenceVersion string            `json:"preferenceVersion"`
	PolicyVersion     string            `json:"policyVersion"`
	Preferences       any               `json:"preferences"`
	Policy            any               `json:"policy"`
	Adopted           map[string]string `json:"adopted"`
}
type Service struct {
	mu        sync.Mutex
	root, dir string
	agent     agentclient.Gateway
	plans     *mealplan.Store
	feedback  *mealfeedback.Store
}

func New(root string, agent agentclient.Gateway, plans *mealplan.Store) *Service {
	return &Service{root: root, dir: filepath.Join(root, "menu-proposals"), agent: agent, plans: plans, feedback: mealfeedback.New(root)}
}

var idPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)

func Digest(value any) string {
	b, _ := json.Marshal(value)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func AtomicJSON(path string, value any) error {
	dir := filepath.Dir(path)
	if e := os.MkdirAll(dir, 0700); e != nil {
		return e
	}
	info, e := os.Lstat(dir)
	if e != nil {
		return e
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return ErrInvalid
	}
	if info, e := os.Lstat(path); e == nil && !info.Mode().IsRegular() {
		return ErrInvalid
	} else if e != nil && !errors.Is(e, os.ErrNotExist) {
		return e
	}
	b, e := json.MarshalIndent(value, "", "  ")
	if e != nil {
		return e
	}
	if len(b) > 16<<20 {
		return ErrInvalid
	}
	f, e := os.CreateTemp(dir, ".json-")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if e = f.Chmod(0600); e == nil {
		_, e = f.Write(b)
	}
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e == nil {
		e = ce
	}
	if e != nil {
		return e
	}
	if e = os.Rename(tmp, path); e != nil {
		return e
	}
	d, e := os.Open(dir)
	if e != nil {
		return e
	}
	e = d.Sync()
	d.Close()
	return e
}
func ReadJSON(path string, value any) error {
	info, e := os.Lstat(path)
	if e != nil {
		return e
	}
	if !info.Mode().IsRegular() || info.Size() > 16<<20 {
		return ErrInvalid
	}
	b, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	return json.Unmarshal(b, value)
}
func OptionalJSON(path string) (any, string, error) {
	var value any
	e := ReadJSON(path, &value)
	if errors.Is(e, os.ErrNotExist) {
		return map[string]any{}, "initial", nil
	}
	if e != nil {
		return nil, "", e
	}
	return value, Digest(value), nil
}
func (s *Service) Feedback() *mealfeedback.Store { return s.feedback }
func (s *Service) read(id string) (*Proposal, error) {
	if !idPattern.MatchString(id) {
		return nil, ErrInvalid
	}
	var p Proposal
	if e := ReadJSON(filepath.Join(s.dir, id+".json"), &p); e != nil {
		return nil, e
	}
	if p.ID != id || p.Schema != 1 || p.Revision < 1 {
		return nil, ErrInvalid
	}
	return &p, nil
}
func (s *Service) write(p *Proposal) error {
	if p.Pending {
		p.State = "queued"
	} else if p.Job != nil {
		p.State = p.Job.State
	} else if p.Mode == "rules" {
		p.State = "succeeded"
	} else {
		p.State = "unavailable"
	}
	return AtomicJSON(filepath.Join(s.dir, p.ID+".json"), p)
}
func (s *Service) busy() bool {
	entries, _ := os.ReadDir(s.dir)
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		p, e := s.read(entry.Name()[:len(entry.Name())-5])
		if e == nil && (p.Pending || p.State == "running" || p.State == "queued" || p.State == "waiting") {
			return true
		}
	}
	return false
}
func (s *Service) Create(ctx context.Context, c Conditions, mode string) (*Proposal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if mode != "ai" && mode != "rules" {
		return nil, ErrInvalid
	}
	preferences, pv, e := OptionalJSON(filepath.Join(s.root, ".settings", "meal-preferences.json"))
	if e != nil {
		return nil, e
	}
	b, _ := json.Marshal(preferences)
	var explicit mealfeedback.Preferences
	if json.Unmarshal(b, &explicit) != nil {
		return nil, ErrInvalid
	}
	c.Avoid = append(c.Avoid, explicit.Avoid...)
	c.Use = append(c.Use, explicit.Use...)
	candidates, e := Candidates(s.root, c)
	if e != nil {
		return nil, e
	}
	if len(candidates) < c.Count {
		return nil, fmt.Errorf("%w: 人数が確定した候補が%d品です。条件・好み・人数登録を確認してください", ErrInvalid, len(candidates))
	}
	policy, sv, e := OptionalJSON(filepath.Join(s.root, "planning-policy.json"))
	if e != nil {
		return nil, e
	}
	id := Digest([]any{c, candidates, mode, pv, sv})[:32]
	if previous, e := s.read(id); e == nil {
		return previous, nil
	} else if !errors.Is(e, os.ErrNotExist) {
		return nil, e
	}
	if mode == "ai" && s.busy() {
		return nil, fmt.Errorf("%w: 先の提案を完了または中止してください", ErrConflict)
	}
	p := &Proposal{Schema: 1, ID: id, Revision: 1, Conditions: c, Candidates: candidates, Mode: mode, PreferenceVersion: pv, PolicyVersion: sv, Preferences: preferences, Policy: policy, Adopted: map[string]string{}}
	if mode == "rules" {
		p.Menus, e = RuleMenus(c, candidates)
		if e != nil {
			return nil, e
		}
		p.Interpretation = "入力した品数・人数・内訳・固定品・除外条件だけで組みました。自由文のイメージは解釈していません。"
	} else if s.agent == nil {
		p.Error = "AI接続が未設定です。条件だけの簡易提案か、一覧から手動で選べます。"
	} else {
		p.Pending = true
	}
	if e = s.write(p); e != nil {
		return nil, e
	}
	if p.Pending {
		if e = s.submit(ctx, p); e != nil {
			p.Error = e.Error()
			_ = s.write(p)
		}
	}
	return p, nil
}
func (s *Service) submit(ctx context.Context, p *Proposal) error {
	j, e := s.agent.Create(ctx, TextRequest("konro-menu-"+p.ID+fmt.Sprint(p.Repairs), p.Conditions, p.Candidates, p.Preferences, p.Policy))
	if e != nil {
		return e
	}
	p.Job = &j
	p.Pending = false
	p.Error = ""
	s.apply(p)
	return s.write(p)
}
func (s *Service) apply(p *Proposal) {
	if p.Job == nil || p.Job.State != "succeeded" {
		return
	}
	var output struct {
		Interpretation string `json:"interpretation"`
		Menus          []Menu `json:"menus"`
	}
	d := json.NewDecoder(bytes.NewReader(p.Job.Output))
	d.DisallowUnknownFields()
	if d.Decode(&output) != nil || d.Decode(new(any)) != io.EOF || len(output.Interpretation) > 4000 {
		p.Job.State = "failed"
		p.Job.Reason = "invalid_output"
		p.Error = "提案の形式を確認できません。修正再試行できます。"
		return
	}
	menus, e := ValidateMenus(p.Conditions, p.Candidates, output.Menus)
	if e != nil {
		p.Job.State = "failed"
		p.Job.Reason = "invalid_conditions"
		p.Error = "必須条件に合わない案を受け取ったため、採用していません。"
		return
	}
	p.Menus = menus
	p.Interpretation = output.Interpretation
	p.Error = ""
}
func (s *Service) Get(ctx context.Context, id string) (*Proposal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.read(id)
	if e != nil {
		return nil, e
	}
	before := Digest(p)
	if p.Pending && s.agent != nil {
		if e = s.submit(ctx, p); e != nil {
			p.Error = e.Error()
		}
	} else if p.Job != nil && s.agent != nil && (p.Job.State == "running" || p.Job.State == "queued" || p.Job.State == "waiting") {
		j, e := s.agent.Get(ctx, p.Job.ID)
		if e != nil {
			p.Error = "状態を取得できません。時間をおいて再確認してください。"
		} else {
			p.Job = &j
			s.apply(p)
		}
	}
	if Digest(p) != before {
		p.Revision++
		if e = s.write(p); e != nil {
			return nil, e
		}
	}
	return p, nil
}
func (s *Service) Action(ctx context.Context, id string, revision int, action string) (*Proposal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.read(id)
	if e != nil {
		return nil, e
	}
	if p.Revision != revision {
		return p, ErrConflict
	}
	if s.agent == nil {
		return p, fmt.Errorf("%w: AI接続がありません", ErrInvalid)
	}
	if p.Pending {
		if e = s.submit(ctx, p); e != nil {
			return p, e
		}
	}
	if p.Job == nil {
		return p, ErrInvalid
	}
	if action == "retry" && (p.Job.Reason == "invalid_output" || p.Job.Reason == "invalid_conditions") {
		if p.Repairs >= 1 {
			return p, fmt.Errorf("%w: 修正再試行は1回までです", ErrInvalid)
		}
		p.Repairs++
		p.Pending = true
		p.Job = nil
		p.Menus = nil
		p.Revision++
		if e = s.write(p); e != nil {
			return nil, e
		}
		if e = s.submit(ctx, p); e != nil {
			p.Error = e.Error()
			_ = s.write(p)
		}
		return p, nil
	}
	if action != "cancel" && action != "retry" {
		return p, ErrInvalid
	}
	j, e := s.agent.Action(ctx, p.Job.ID, action)
	if e != nil {
		return p, e
	}
	p.Job = &j
	p.Pending = false
	p.Revision++
	s.apply(p)
	return p, s.write(p)
}
func (s *Service) Adopt(id string, revision int, menuID, operation string) (mealplan.Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.read(id)
	if e != nil {
		return mealplan.Plan{}, e
	}
	if old, ok := p.Adopted[operation]; ok {
		parts := strings.SplitN(old, ":", 2)
		if len(parts) != 2 || parts[0] != menuID {
			return mealplan.Plan{}, ErrConflict
		}
		return s.plans.Get(parts[1])
	}
	if p.Revision != revision || p.State != "succeeded" {
		return mealplan.Plan{}, ErrConflict
	}
	var menu *Menu
	for i := range p.Menus {
		if p.Menus[i].ID == menuID {
			menu = &p.Menus[i]
		}
	}
	if menu == nil {
		return mealplan.Plan{}, ErrInvalid
	}
	selected := []mealplan.Selection{}
	for _, pick := range menu.Picks {
		for _, candidate := range p.Candidates {
			if candidate.Recipe.File == pick.File {
				selected = append(selected, mealplan.Selection{File: pick.File, Version: candidate.Recipe.Version, Scale: candidate.Scale})
			}
		}
	}
	plan, e := s.plans.Create(operation, mealplan.Change{Name: "作り置きの献立", ProposalID: p.ID, Items: selected})
	if e != nil {
		return plan, e
	}
	if _, e = s.feedback.Record(mealfeedback.Event{ID: "adopt-" + Digest(operation)[:32], Type: "adopt", ProposalID: p.ID, PlanID: plan.ID, PlanRevision: 1}); e != nil {
		return plan, e
	}
	p.Adopted[operation] = menuID + ":" + plan.ID
	p.Revision++
	if e = s.write(p); e != nil {
		return plan, e
	}
	return plan, nil
}
func RuleMenus(c Conditions, candidates []Candidate) ([]Menu, error) {
	menus := []Menu{}
	seenSets := map[string]bool{}
	for offset := 0; offset < len(candidates) && len(menus) < 3; offset++ {
		chosen := []Pick{}
		seen := map[string]bool{}
		main, side := 0, 0
		add := func(candidate Candidate) bool {
			r := candidate.Recipe
			if seen[r.File] {
				return false
			}
			isMain := contains(r.Tags, "メイン") || contains(r.Tags, "主菜")
			isSide := contains(r.Tags, "副菜")
			if c.Main+c.Side > 0 {
				if isMain == isSide {
					return false
				}
				if isMain && main >= c.Main || isSide && side >= c.Side {
					return false
				}
			}
			chosen = append(chosen, Pick{File: r.File})
			seen[r.File] = true
			if isMain {
				main++
			}
			if isSide {
				side++
			}
			return true
		}
		for _, file := range c.Fixed {
			for _, candidate := range candidates {
				if candidate.Recipe.File == file {
					add(candidate)
				}
			}
		}
		for i := 0; i < len(candidates) && len(chosen) < c.Count; i++ {
			add(candidates[(i+offset+c.Seed%len(candidates)+len(candidates))%len(candidates)])
		}
		if len(chosen) != c.Count {
			continue
		}
		keys := []string{}
		for _, p := range chosen {
			keys = append(keys, p.File)
		}
		sort.Strings(keys)
		key := Digest(keys)
		if seenSets[key] {
			continue
		}
		seenSets[key] = true
		menus = append(menus, Menu{Picks: chosen})
	}
	if len(menus) == 0 {
		return nil, fmt.Errorf("%w: 確認済みの分類だけでは献立を組めません。内訳や条件を確認してください", ErrInvalid)
	}
	return ValidateMenus(c, candidates, menus)
}

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
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
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
	CreatedAt         string            `json:"createdAt,omitempty"`
	Repairs           int               `json:"repairs"`
	PreferenceVersion string            `json:"preferenceVersion"`
	PolicyVersion     string            `json:"policyVersion"`
	Preferences       any               `json:"preferences"`
	Policy            any               `json:"policy"`
	Adopted           map[string]string `json:"adopted"`
	Trace             string            `json:"trace,omitempty"`
	TracedJob         string            `json:"tracedJob,omitempty"`
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

// Active reconciles persisted state with the runtime before reserving a new job.
func (s *Service) Active(ctx context.Context) ([]*Proposal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.activeLocked(ctx)
}
func (s *Service) activeLocked(ctx context.Context) ([]*Proposal, error) {
	result := []*Proposal{}
	entries, err := os.ReadDir(s.dir)
	if errors.Is(err, os.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		p, err := s.read(strings.TrimSuffix(entry.Name(), ".json"))
		if err != nil {
			return nil, err
		}
		if p.Pending || p.State == "running" || p.State == "queued" || p.State == "waiting" {
			p, err = s.getLocked(ctx, p.ID)
			if err != nil {
				return nil, err
			}
			if p.Pending || p.State == "running" || p.State == "queued" || p.State == "waiting" {
				result = append(result, p)
			}
		}
	}
	return result, nil
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
	c.Avoid = Unique(append(c.Avoid, explicit.Avoid...))
	c.Use = Unique(append(c.Use, explicit.Use...))
	candidates, e := Candidates(s.root, c)
	if e != nil {
		return nil, e
	}
	if len(candidates) < c.Count {
		return nil, fmt.Errorf("%w: 条件に合う候補が%d品です。品数・好み・レシピの材料と手順を確認してください", ErrInvalid, len(candidates))
	}
	policy, e := s.currentPolicy()
	sv := policy.Version
	if e != nil {
		return nil, e
	}
	id := Digest([]any{c, candidates, mode, pv, sv})[:32]
	if previous, e := s.read(id); e == nil {
		return previous, nil
	} else if !errors.Is(e, os.ErrNotExist) {
		return nil, e
	}
	if mode == "ai" {
		active, err := s.activeLocked(ctx)
		if err != nil {
			return nil, err
		}
		if len(active) > 0 {
			return nil, fmt.Errorf("%w: 進行中の提案を開いて完了または中止してください", ErrConflict)
		}
	}
	ctx, root := telemetry.Tracer().Start(ctx, "konro.menu", trace.WithNewRoot(), trace.WithAttributes(telemetry.KeyApp.String(telemetry.App), telemetry.KeyTask.String("menu"), telemetry.KeyTraceName.String("konro.menu"), telemetry.KeyRelease.String(telemetry.CodeVersion()), attribute.String("konro.menu.mode", mode), attribute.Int("konro.menu.candidates", len(candidates)), attribute.Int("konro.menu.count", c.Count), attribute.String("konro.menu.proposal", id)))
	defer root.End()
	if telemetry.CaptureContent() {
		b, _ := json.Marshal(c)
		root.SetAttributes(telemetry.KeyInput.String(telemetry.Content(string(b))))
	}
	p := &Proposal{Trace: telemetry.Traceparent(ctx), Schema: 1, ID: id, Revision: 1, CreatedAt: time.Now().UTC().Format(time.RFC3339), Conditions: c, Candidates: candidates, Mode: mode, PreferenceVersion: pv, PolicyVersion: sv, Preferences: preferences, Policy: policy, Adopted: map[string]string{}}
	if mode == "rules" {
		p.Menus, e = RuleMenus(c, Rank(c, candidates, policy))
		if e != nil {
			return nil, e
		}
		p.Interpretation = "入力した品数・内訳・固定品・除外条件だけで組みました。自由文のイメージは解釈していません。"
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
	req := TextRequest("konro-menu-"+p.ID+fmt.Sprint(p.Repairs), p.Conditions, p.Candidates, p.Preferences, p.Policy)
	ctx, span := telemetry.Tracer().Start(telemetry.Resume(ctx, p.Trace), "konro.menu.submit", trace.WithAttributes(telemetry.KeyTask.String("menu"), telemetry.KeyPromptName.String("konro.menu"), telemetry.KeyPromptVersion.String(PromptVersion()), attribute.Int("konro.menu.repairs", p.Repairs)))
	defer span.End()
	if telemetry.CaptureContent() {
		b, _ := json.Marshal(map[string]any{"prompt": req.Prompt, "input": req.Input})
		span.SetAttributes(telemetry.KeyInput.String(telemetry.Content(string(b))))
	}
	j, e := s.agent.Create(ctx, req)
	if e != nil {
		span.RecordError(e)
		span.SetStatus(codes.Error, "runtime unavailable")
		return e
	}
	span.SetAttributes(telemetry.KeyJobID.String(j.ID), telemetry.KeyJobState.String(j.State))
	p.Job = &j
	p.Pending = false
	p.Error = ""
	s.apply(ctx, p)
	return s.write(p)
}

// PromptVersion identifies the menu instruction and output schema.
func PromptVersion() string {
	r := TextRequest("", Conditions{}, nil, nil, nil)
	return telemetry.Version(r.Prompt, OutputSchema)
}

func (s *Service) observe(ctx context.Context, p *Proposal, outcome string) {
	if p.Job == nil || (p.Job.State != "succeeded" && p.Job.State != "failed" && p.Job.State != "cancelled") {
		return
	}
	key := p.Job.ID + ":" + p.Job.State + ":" + p.Job.Reason
	if p.TracedJob == key {
		return
	}
	p.TracedJob = key
	_, span := telemetry.Tracer().Start(telemetry.Resume(ctx, p.Trace), "konro.menu.apply", trace.WithAttributes(telemetry.KeyTask.String("menu"), telemetry.KeyJobID.String(p.Job.ID), telemetry.KeyJobState.String(p.Job.State), telemetry.KeyJobReason.String(p.Job.Reason), telemetry.KeyOutcome.String(outcome), telemetry.KeyPromptVersion.String(PromptVersion()), attribute.Int("konro.menu.menus", len(p.Menus))))
	if outcome != "applied" {
		span.SetStatus(codes.Error, outcome)
	}
	if telemetry.CaptureContent() && len(p.Job.Output) > 0 {
		span.SetAttributes(telemetry.KeyOutput.String(telemetry.Content(string(p.Job.Output))))
	}
	span.End()
}

func (s *Service) apply(ctx context.Context, p *Proposal) {
	if p.Job != nil && p.Job.State != "succeeded" {
		s.observe(ctx, p, "job_"+p.Job.State)
	}
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
		s.observe(ctx, p, "invalid_output")
		return
	}
	menus, e := ValidateMenus(p.Conditions, p.Candidates, output.Menus)
	if e != nil {
		p.Job.State = "failed"
		p.Job.Reason = "invalid_conditions"
		p.Error = "必須条件に合わない案を受け取ったため、採用していません。"
		s.observe(ctx, p, "invalid_conditions")
		return
	}
	p.Menus = menus
	p.Interpretation = output.Interpretation
	p.Error = ""
	s.observe(ctx, p, "applied")
}
func (s *Service) Get(ctx context.Context, id string) (*Proposal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getLocked(ctx, id)
}
func (s *Service) getLocked(ctx context.Context, id string) (*Proposal, error) {
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
			p.Error = ""
			s.apply(ctx, p)
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
	s.apply(ctx, p)
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
	_, currentPreferences, e := OptionalJSON(filepath.Join(s.root, ".settings", "meal-preferences.json"))
	if e != nil {
		return mealplan.Plan{}, e
	}
	currentPolicy, e := s.currentPolicy()
	if e != nil {
		return mealplan.Plan{}, e
	}
	if currentPreferences != p.PreferenceVersion || currentPolicy.Version != p.PolicyVersion {
		return mealplan.Plan{}, ErrConflict
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

func Unique(values []string) []string {
	result := []string{}
	seen := map[string]bool{}
	for _, v := range values {
		key := normalized(v)
		if !seen[key] {
			seen[key] = true
			result = append(result, v)
		}
	}
	return result
}

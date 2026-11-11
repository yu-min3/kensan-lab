// Package evalrun executes Konro's import cases through the production path
// (agent-runtime, same profile and schema), grades them, and records the run.
//
// Each case is one trace whose root carries Langfuse experiment attributes, so
// the run appears as an experiment against the synchronized dataset. Runtime
// spans join through the traceparent sent with the job.
package evalrun

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/agentclient"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/evalcases"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/grader"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/importer"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const DatasetName = "konro-import"

// ItemID is stable per case and dataset, so re-syncing updates in place.
func ItemID(c evalcases.Case) string { return DatasetName + "/" + c.ID }

// Sync makes the Langfuse dataset mirror the case file. The case file stays
// the source of truth; edits made in Langfuse are overwritten.
func Sync(ctx context.Context, lf Langfuse, set evalcases.Set) (string, error) {
	id, err := lf.EnsureDataset(ctx, DatasetName, "Konro recipe import regression cases (synthetic). Source of truth: apps/konro/backend/internal/evalcases/cases.json", map[string]any{"cases_hash": evalcases.Hash()})
	if err != nil {
		return "", err
	}
	for _, c := range set.Cases {
		_, draft := grader.Prepare(c)
		if err := lf.UpsertItem(ctx, DatasetItem{ID: ItemID(c), DatasetName: DatasetName,
			Input:          map[string]any{"mode": c.Mode, "text": c.Text, "draft": draft},
			ExpectedOutput: c.Expect,
			Metadata:       map[string]any{"case_id": c.ID, "split": c.Split, "tags": c.Tags, "cases_hash": evalcases.Hash(), "annotation": "synthetic"}}); err != nil {
			return "", fmt.Errorf("%s: %w", c.ID, err)
		}
	}
	return id, nil
}

type Config struct {
	Name       string            // experiment name, e.g. "baseline" or "prompt-v2"
	Prompts    map[string]string // mode -> instruction; empty uses production
	ModelLabel string            // operator's model choice; part of the cache key
	CacheDir   string            // "" disables caching
	DatasetID  string            // Langfuse dataset id ("" when not syncing)
	Poll       time.Duration
	Timeout    time.Duration
	Now        func() time.Time
}

type CaseResult struct {
	grader.Result
	Split         string          `json:"split"`
	PromptVersion string          `json:"prompt_version"`
	JobID         string          `json:"job_id,omitempty"`
	Cached        bool            `json:"cached"`
	TraceID       string          `json:"trace_id,omitempty"`
	Output        json.RawMessage `json:"output,omitempty"`
}

type Report struct {
	Name          string            `json:"name"`
	ExperimentID  string            `json:"experiment_id"`
	StartedAt     time.Time         `json:"started_at"`
	CasesHash     string            `json:"cases_hash"`
	GraderVersion string            `json:"grader_version"`
	ModelLabel    string            `json:"model_label"`
	Prompts       map[string]string `json:"prompt_versions"`
	Summary       grader.Summary    `json:"summary"`
	Results       []CaseResult      `json:"results"`
}

type cached struct {
	State  string          `json:"state"`
	Output json.RawMessage `json:"output"`
}

func (cfg Config) prompt(mode string) string {
	if p := cfg.Prompts[mode]; p != "" {
		return p
	}
	return importer.Prompt(mode)
}

// Run executes cases, grades them and returns the report. lf may be nil.
// Jobs are all submitted first and then polled: the runtime runs them one at a
// time, and evaluation jobs yield to online jobs on the runtime side.
func Run(ctx context.Context, agent agentclient.Gateway, lf Langfuse, cases []evalcases.Case, cfg Config) (Report, error) {
	if cfg.Name == "" || strings.ContainsAny(cfg.Name, "/ ") {
		return Report{}, errors.New("experiment name must be non-empty without spaces or slashes")
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Poll == 0 {
		cfg.Poll = 3 * time.Second
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Minute
	}
	start := cfg.Now().UTC()
	rep := Report{Name: cfg.Name, ExperimentID: cfg.Name + "-" + start.Format("20060102T150405Z"), StartedAt: start, CasesHash: evalcases.Hash(), GraderVersion: grader.Version, ModelLabel: cfg.ModelLabel, Prompts: map[string]string{}}
	type pending struct {
		c     evalcases.Case
		ctx   context.Context
		root  trace.Span
		job   agentclient.Job
		res   CaseResult
		input map[string]any
		key   string
	}
	var work []*pending
	for _, c := range cases {
		prompt := cfg.prompt(c.Mode)
		pv := telemetry.Version(prompt, importer.Schema)
		rep.Prompts[c.Mode] = pv
		_, draft := grader.Prepare(c)
		input := map[string]any{"text": c.Text, "draft": draft}
		attrs := []attribute.KeyValue{
			telemetry.KeyApp.String(telemetry.App), telemetry.KeyTask.String("import-eval"),
			telemetry.KeyTraceName.String("konro.eval.import"), telemetry.KeyPromptVersion.String(pv),
			attribute.String("langfuse.environment", "experiment"),
			attribute.String("langfuse.experiment.id", rep.ExperimentID),
			attribute.String("langfuse.experiment.name", rep.ExperimentID),
			attribute.String("langfuse.experiment.metadata.prompt_version", pv),
			attribute.String("langfuse.experiment.metadata.grader_version", grader.Version),
			attribute.String("langfuse.experiment.metadata.cases_hash", rep.CasesHash),
			attribute.String("langfuse.experiment.item.id", ItemID(c)),
			attribute.String("konro.eval.case", c.ID), attribute.String("konro.eval.split", c.Split),
		}
		if cfg.DatasetID != "" {
			attrs = append(attrs, attribute.String("langfuse.experiment.dataset.id", cfg.DatasetID))
		}
		ctx2, root := telemetry.Tracer().Start(ctx, "konro.eval.import", trace.WithNewRoot(), trace.WithAttributes(attrs...))
		root.SetAttributes(attribute.String("langfuse.experiment.item.root_observation_id", root.SpanContext().SpanID().String()))
		in, _ := json.Marshal(map[string]any{"mode": c.Mode, "input": input})
		exp, _ := json.Marshal(c.Expect)
		root.SetAttributes(telemetry.KeyInput.String(string(in)), attribute.String("langfuse.experiment.item.expected_output", string(exp)))
		key := cacheKey(prompt, input, cfg.ModelLabel)
		p := &pending{c: c, ctx: ctx2, root: root, input: input, key: key, res: CaseResult{Split: c.Split, PromptVersion: pv, TraceID: root.SpanContext().TraceID().String()}}
		if hit, ok := readCache(cfg.CacheDir, key); ok {
			p.job = agentclient.Job{State: hit.State, Output: hit.Output}
			p.res.Cached = true
		} else {
			sctx, sub := telemetry.Tracer().Start(ctx2, "konro.import.submit", trace.WithAttributes(telemetry.KeyPromptVersion.String(pv)))
			j, err := agent.Create(sctx, agentclient.Request{Profile: "text", Prompt: prompt, Input: input, OutputSchema: json.RawMessage(importer.Schema), IdempotencyKey: fmt.Sprintf("konro-eval-%s-%d", key[:24], start.Unix())})
			if err != nil {
				sub.RecordError(err)
				sub.SetStatus(codes.Error, "submit failed")
				j = agentclient.Job{State: "failed", Reason: "submit_failed"}
			}
			sub.SetAttributes(telemetry.KeyJobID.String(j.ID), telemetry.KeyJobState.String(j.State))
			sub.End()
			p.job = j
			p.res.JobID = j.ID
		}
		work = append(work, p)
	}
	deadline := cfg.Now().Add(cfg.Timeout)
	for {
		open := 0
		for _, p := range work {
			if terminal(p.job.State) || p.job.ID == "" {
				continue
			}
			j, err := agent.Get(p.ctx, p.job.ID)
			if err == nil && j.ID == p.job.ID {
				p.job = j
			}
			if !terminal(p.job.State) {
				open++
			}
		}
		if open == 0 || cfg.Now().After(deadline) || ctx.Err() != nil {
			break
		}
		select {
		case <-ctx.Done():
		case <-time.After(cfg.Poll):
		}
	}
	var scores []Score
	for _, p := range work {
		r := grader.Grade(p.c, p.job.State, p.job.Reason, p.job.Output)
		p.res.Result = r
		p.res.Output = p.job.Output
		if p.job.State == "succeeded" && !p.res.Cached {
			writeCache(cfg.CacheDir, p.key, cached{State: p.job.State, Output: p.job.Output})
		}
		_, g := telemetry.Tracer().Start(p.ctx, "konro.eval.grade", trace.WithAttributes(attribute.String("konro.grade.status", r.Status), attribute.Float64("konro.grade.score", r.Score), attribute.String("konro.grade.reasons", strings.Join(r.Reasons, ","))))
		g.End()
		if len(p.job.Output) > 0 {
			p.root.SetAttributes(telemetry.KeyOutput.String(telemetry.Content(string(p.job.Output))))
		}
		p.root.SetAttributes(telemetry.KeyJobState.String(p.job.State), telemetry.KeyJobReason.String(p.job.Reason), attribute.Bool("konro.eval.cached", p.res.Cached))
		if r.Status != grader.Passed {
			p.root.SetStatus(codes.Error, r.Status)
		}
		p.root.End()
		rep.Results = append(rep.Results, p.res)
		obs := p.root.SpanContext().SpanID().String()
		base := Score{TraceID: p.res.TraceID, ObservationID: obs, Timestamp: start, Environment: "experiment", Metadata: map[string]any{"grader_version": grader.Version, "case_id": p.c.ID}}
		status := base
		status.ID, status.Name, status.DataType, status.Value, status.Comment = rep.ExperimentID+"/"+p.c.ID+"/status", "status", "CATEGORICAL", r.Status, strings.Join(r.Reasons, ", ")
		scores = append(scores, status)
		if r.Status != grader.NotEvaluated {
			score := base
			score.ID, score.Name, score.DataType, score.Value = rep.ExperimentID+"/"+p.c.ID+"/score", "check_score", "NUMERIC", r.Score
			scores = append(scores, score)
		}
	}
	rep.Summary = grader.Summarize(gradeResults(rep.Results))
	if lf != nil {
		if err := lf.Scores(ctx, scores); err != nil {
			return rep, fmt.Errorf("scores not recorded: %w", err)
		}
	}
	return rep, nil
}

func gradeResults(rs []CaseResult) []grader.Result {
	out := make([]grader.Result, len(rs))
	for i, r := range rs {
		out[i] = r.Result
	}
	return out
}

func terminal(s string) bool { return s == "succeeded" || s == "failed" || s == "cancelled" }

func cacheKey(prompt string, input any, model string) string {
	b, _ := json.Marshal([]any{prompt, importer.Schema, input, model})
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func readCache(dir, key string) (cached, bool) {
	if dir == "" {
		return cached{}, false
	}
	b, err := os.ReadFile(filepath.Join(dir, key+".json"))
	var c cached
	if err != nil || json.Unmarshal(b, &c) != nil || c.State != "succeeded" {
		return cached{}, false
	}
	return c, true
}

func writeCache(dir, key string, c cached) {
	if dir == "" {
		return
	}
	if os.MkdirAll(dir, 0700) != nil {
		return
	}
	b, _ := json.Marshal(c)
	tmp := filepath.Join(dir, "."+key+".tmp")
	if os.WriteFile(tmp, b, 0600) == nil {
		_ = os.Rename(tmp, filepath.Join(dir, key+".json"))
	}
}

// Compare lists per-case differences between two reports on the same cases.
func Compare(a, b Report) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "| case | split | %s | %s |\n|---|---|---|---|\n", a.Name, b.Name)
	idx := map[string]CaseResult{}
	for _, r := range b.Results {
		idx[r.CaseID] = r
	}
	cell := func(r CaseResult) string {
		if r.Status == grader.NotEvaluated {
			return "— " + r.JobState
		}
		s := fmt.Sprintf("%s %.2f", r.Status, r.Score)
		if len(r.Reasons) > 0 {
			s += " (" + strings.Join(r.Reasons, ", ") + ")"
		}
		return s
	}
	for _, ra := range a.Results {
		rb, ok := idx[ra.CaseID]
		right := "missing"
		if ok {
			right = cell(rb)
		}
		mark := ""
		if ok && ra.Status != rb.Status {
			mark = " ◆"
		}
		fmt.Fprintf(&sb, "| %s%s | %s | %s | %s |\n", ra.CaseID, mark, ra.Split, cell(ra), right)
	}
	fmt.Fprintf(&sb, "\n%s: %+v\n%s: %+v\n", a.Name, a.Summary, b.Name, b.Summary)
	if a.CasesHash != b.CasesHash || a.GraderVersion != b.GraderVersion {
		sb.WriteString("\n注意: ケースまたは採点器の版が異なるため、点数は直接比較できない\n")
	}
	return sb.String()
}

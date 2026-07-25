// Package metrics は project ごとのメトリクス定義と観測履歴を扱う。
package metrics

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var idRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

type Config struct {
	Version int          `yaml:"version"`
	Metrics []Definition `yaml:"metrics"`
}

type Definition struct {
	ID          string          `yaml:"id"`
	Label       string          `yaml:"label"`
	Unit        string          `yaml:"unit"`
	Target      *float64        `yaml:"target"`
	Direction   string          `yaml:"direction"`
	Display     string          `yaml:"display"`
	Checkpoints []Checkpoint    `yaml:"checkpoints"`
	Collector   CollectorConfig `yaml:"collector"`
}

// Checkpoint は target までの途中の到達点。Journey / Hero の「次の到達点」に使う。
// 到達判定は観測値から自動で行い、README の checkbox は書き換えない。
type Checkpoint struct {
	Value float64 `yaml:"value" json:"value"`
	Label string  `yaml:"label" json:"label"`
}

// CollectorConfig は collector の指定。`type` だけを共通で解釈し、残りの
// キーは collector 固有として生のまま持つ。collector を足すときに
// この struct も metrics.yaml のスキーマも変更しなくて済むようにするため。
type CollectorConfig struct {
	Type string
	node *yaml.Node
}

func (c *CollectorConfig) UnmarshalYAML(node *yaml.Node) error {
	var head struct {
		Type string `yaml:"type"`
	}
	if err := node.Decode(&head); err != nil {
		return err
	}
	c.Type = head.Type
	c.node = node
	return nil
}

// Decode は collector 固有の設定を v へ読み出す。collector 側が自分の
// struct を定義して呼ぶ。設定ブロックが無い場合は何もしない。
func (c CollectorConfig) Decode(v any) error {
	if c.node == nil {
		return nil
	}
	return c.node.Decode(v)
}

type Observation struct {
	Metric string         `json:"metric"`
	At     string         `json:"at"`
	Value  float64        `json:"value"`
	Source string         `json:"source"`
	Fields map[string]any `json:"fields,omitempty"`
}

type Point struct {
	At    string  `json:"at"`
	Value float64 `json:"value"`
}

type Delta struct {
	Previous *float64 `json:"previous,omitempty"`
	Days30   *float64 `json:"days30,omitempty"`
}

type Best struct {
	Value float64 `json:"value"`
	At    string  `json:"at"`
}

type View struct {
	ID          string         `json:"id"`
	Label       string         `json:"label"`
	Unit        string         `json:"unit"`
	Target      *float64       `json:"target,omitempty"`
	Direction   string         `json:"direction"`
	Display     string         `json:"display"`
	Current     *float64       `json:"current,omitempty"`
	Fields      map[string]any `json:"fields,omitempty"`
	Delta       Delta          `json:"delta"`
	Best        *Best          `json:"best,omitempty"`
	Checkpoints []Checkpoint   `json:"checkpoints,omitempty"`
	UpdatedAt   string         `json:"updatedAt,omitempty"`
	Stale       bool           `json:"stale"`
	Series      []Point        `json:"series"`
}

type Result struct {
	Metrics []View `json:"metrics"`
}

func projectDir(root, project string) (string, error) {
	if !idRe.MatchString(project) {
		return "", fmt.Errorf("invalid project name: %q", project)
	}
	return filepath.Join(root, "projects", project), nil
}

func Load(root, project string, now time.Time) (Result, error) {
	dir, err := projectDir(root, project)
	if err != nil {
		return Result{}, err
	}
	config, err := loadConfig(filepath.Join(dir, "metrics.yaml"))
	if errors.Is(err, os.ErrNotExist) {
		return Result{Metrics: []View{}}, nil
	}
	if err != nil {
		return Result{}, err
	}
	observations, err := loadObservations(filepath.Join(dir, "metrics.ndjson"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Result{}, err
	}

	byMetric := map[string][]Observation{}
	for _, o := range observations {
		byMetric[o.Metric] = append(byMetric[o.Metric], o)
	}
	result := Result{Metrics: make([]View, 0, len(config.Metrics))}
	for _, definition := range config.Metrics {
		series := byMetric[definition.ID]
		sort.SliceStable(series, func(i, j int) bool { return series[i].At < series[j].At })
		result.Metrics = append(result.Metrics, buildView(definition, series, now))
	}
	return result, nil
}

func loadConfig(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var config Config
	decoder := yaml.NewDecoder(bytes.NewReader(b))
	decoder.KnownFields(true)
	if err := decoder.Decode(&config); err != nil {
		return Config{}, fmt.Errorf("parse metrics.yaml: %w", err)
	}
	if config.Version != 1 {
		return Config{}, fmt.Errorf("unsupported metrics version: %d", config.Version)
	}
	seen := map[string]bool{}
	for _, metric := range config.Metrics {
		if !idRe.MatchString(metric.ID) || metric.Label == "" || metric.Unit == "" {
			return Config{}, fmt.Errorf("invalid metric definition: %q", metric.ID)
		}
		if seen[metric.ID] {
			return Config{}, fmt.Errorf("duplicate metric id: %s", metric.ID)
		}
		seen[metric.ID] = true
		if metric.Direction != "increase" && metric.Direction != "decrease" && metric.Direction != "maintain" {
			return Config{}, fmt.Errorf("invalid direction for %s: %s", metric.ID, metric.Direction)
		}
		if metric.Display != "integer" && metric.Display != "decimal" && metric.Display != "percent" {
			return Config{}, fmt.Errorf("invalid display for %s: %s", metric.ID, metric.Display)
		}
		// collector 固有の設定検証は collector 自身が行う（Refresh 時にエラーになる）。
		// 未知の type でも Load は通す — 表示は既存の履歴で成立するため、
		// 収集できないことと履歴が読めないことを切り分ける。
	}
	return config, nil
}

func loadObservations(path string) ([]Observation, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []Observation
	scanner := bufio.NewScanner(f)
	line := 0
	for scanner.Scan() {
		line++
		if strings.TrimSpace(scanner.Text()) == "" {
			continue
		}
		var o Observation
		if err := json.Unmarshal(scanner.Bytes(), &o); err != nil {
			return nil, fmt.Errorf("parse metrics.ndjson line %d: %w", line, err)
		}
		if o.Metric == "" || o.At == "" || o.Source == "" {
			return nil, fmt.Errorf("invalid metrics.ndjson line %d", line)
		}
		if _, err := parseTime(o.At); err != nil {
			return nil, fmt.Errorf("invalid observation date on line %d: %w", line, err)
		}
		out = append(out, o)
	}
	return out, scanner.Err()
}

func buildView(d Definition, observations []Observation, now time.Time) View {
	v := View{ID: d.ID, Label: d.Label, Unit: d.Unit, Target: d.Target, Direction: d.Direction, Display: d.Display, Series: []Point{}}
	// checkpoint は値の昇順で返す（Journey がそのまま左→右に並べられるように）。
	v.Checkpoints = append(v.Checkpoints, d.Checkpoints...)
	sort.Slice(v.Checkpoints, func(i, j int) bool { return v.Checkpoints[i].Value < v.Checkpoints[j].Value })
	for _, o := range observations {
		v.Series = append(v.Series, Point{At: o.At, Value: o.Value})
	}
	if len(observations) == 0 {
		return v
	}
	current := observations[len(observations)-1]
	v.Current, v.Fields, v.UpdatedAt = &current.Value, current.Fields, current.At
	updated, _ := parseTime(current.At)
	v.Stale = now.Sub(updated) > 24*time.Hour
	if len(observations) > 1 {
		delta := current.Value - observations[len(observations)-2].Value
		v.Delta.Previous = &delta
	}
	cutoff := now.AddDate(0, 0, -30)
	baseline := -1
	for _, o := range observations {
		at, _ := parseTime(o.At)
		if !at.After(cutoff) {
			baseline++
			continue
		}
		break
	}
	if baseline < 0 {
		baseline = 0
	}
	if baseline < len(observations)-1 {
		delta := current.Value - observations[baseline].Value
		v.Delta.Days30 = &delta
	}
	best := observations[0]
	for _, o := range observations[1:] {
		better := d.Direction != "decrease" && o.Value > best.Value
		if d.Direction == "decrease" {
			better = o.Value < best.Value
		}
		if better {
			best = o
		}
	}
	v.Best = &Best{Value: best.Value, At: best.At}
	return v
}

func parseTime(value string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, nil
	}
	return time.Parse("2006-01-02", value)
}

// Env は collector が使える環境。ゼロ値で既定（実 HTTP クライアント・os.Getenv）に
// フォールバックするので、呼び出し側は通常 metrics.Env{} を渡すだけでよい。
type Env struct {
	HTTP   *http.Client
	Secret func(name string) string
}

func (e Env) httpClient() *http.Client {
	if e.HTTP != nil {
		return e.HTTP
	}
	return &http.Client{Timeout: 5 * time.Second}
}

func (e Env) secret(name string) string {
	if e.Secret != nil {
		return e.Secret(name)
	}
	return os.Getenv(name)
}

// CollectCtx は 1 metric 分の収集コンテキスト。HTTP を前提にしない
// （ベンチプレス記録のように workspace のファイルを読む collector も同じ形で書ける）。
type CollectCtx struct {
	Env
	Root    string // workspace root
	Project string
	Config  CollectorConfig
}

// Sample は collector が返す 1 観測。Fields は補助情報（best set の rep / 重量など）。
type Sample struct {
	Value  float64
	Fields map[string]any
	Source string
}

type Collector func(ctx context.Context, cc CollectCtx) (Sample, error)

var collectors = map[string]Collector{
	"github-stars": collectGitHubStars,
}

// Register は collector を追加する。type 名は metrics.yaml の collector.type と対応する。
func Register(name string, c Collector) { collectors[name] = c }

// Refresh は外部 collector を実行し、成功した observation を追記する。
func Refresh(ctx context.Context, root, project string, now time.Time, env Env) (Result, error) {
	dir, err := projectDir(root, project)
	if err != nil {
		return Result{}, err
	}
	config, err := loadConfig(filepath.Join(dir, "metrics.yaml"))
	if err != nil {
		return Result{}, err
	}
	var refreshErrs []error
	for _, metric := range config.Metrics {
		// collector 未指定は手入力メトリクス。収集対象外として黙って飛ばす。
		if metric.Collector.Type == "" {
			continue
		}
		collect, ok := collectors[metric.Collector.Type]
		if !ok {
			refreshErrs = append(refreshErrs, fmt.Errorf("%s: 未知の collector type %q", metric.ID, metric.Collector.Type))
			continue
		}
		sample, err := collect(ctx, CollectCtx{Env: env, Root: root, Project: project, Config: metric.Collector})
		if err != nil {
			refreshErrs = append(refreshErrs, fmt.Errorf("%s: %w", metric.ID, err))
			continue
		}
		o := Observation{
			Metric: metric.ID,
			At:     now.Format(time.RFC3339),
			Value:  sample.Value,
			Source: sample.Source,
			Fields: sample.Fields,
		}
		if err := appendObservation(filepath.Join(dir, "metrics.ndjson"), o); err != nil {
			refreshErrs = append(refreshErrs, fmt.Errorf("%s: %w", metric.ID, err))
		}
	}
	result, loadErr := Load(root, project, now)
	if loadErr != nil {
		return Result{}, loadErr
	}
	return result, errors.Join(refreshErrs...)
}

func collectGitHubStars(ctx context.Context, cc CollectCtx) (Sample, error) {
	var cfg struct {
		Repo string `yaml:"repo"`
	}
	if err := cc.Config.Decode(&cfg); err != nil {
		return Sample{}, err
	}
	if cfg.Repo == "" {
		return Sample{}, errors.New("collector github-stars: repo が未設定")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/"+cfg.Repo, nil)
	if err != nil {
		return Sample{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if token := cc.secret("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := cc.httpClient().Do(req)
	if err != nil {
		return Sample{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return Sample{}, fmt.Errorf("github returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var body struct {
		Stars float64 `json:"stargazers_count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Sample{}, err
	}
	return Sample{Value: body.Stars, Source: "github"}, nil
}

func appendObservation(path string, o Observation) error {
	existing, err := loadObservations(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	day := o.At[:10]
	for _, previous := range existing {
		if previous.Metric == o.Metric && strings.HasPrefix(previous.At, day) && previous.Value == o.Value {
			return nil
		}
	}
	b, err := json.Marshal(o)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "%s\n", b)
	return err
}

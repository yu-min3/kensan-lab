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
	ID        string          `yaml:"id"`
	Label     string          `yaml:"label"`
	Unit      string          `yaml:"unit"`
	Target    *float64        `yaml:"target"`
	Direction string          `yaml:"direction"`
	Display   string          `yaml:"display"`
	Collector CollectorConfig `yaml:"collector"`
}

type CollectorConfig struct {
	Type string `yaml:"type"`
	Repo string `yaml:"repo,omitempty"`
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
	ID        string         `json:"id"`
	Label     string         `json:"label"`
	Unit      string         `json:"unit"`
	Target    *float64       `json:"target,omitempty"`
	Direction string         `json:"direction"`
	Display   string         `json:"display"`
	Current   *float64       `json:"current,omitempty"`
	Fields    map[string]any `json:"fields,omitempty"`
	Delta     Delta          `json:"delta"`
	Best      *Best          `json:"best,omitempty"`
	UpdatedAt string         `json:"updatedAt,omitempty"`
	Stale     bool           `json:"stale"`
	Series    []Point        `json:"series"`
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
		if metric.Collector.Type == "github-stars" && metric.Collector.Repo == "" {
			return Config{}, fmt.Errorf("github-stars metric %s requires repo", metric.ID)
		}
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

// Refresh は外部 collector を実行し、成功した observation を追記する。
func Refresh(ctx context.Context, root, project string, now time.Time, client *http.Client, token string) (Result, error) {
	dir, err := projectDir(root, project)
	if err != nil {
		return Result{}, err
	}
	config, err := loadConfig(filepath.Join(dir, "metrics.yaml"))
	if err != nil {
		return Result{}, err
	}
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	var refreshErrs []error
	for _, metric := range config.Metrics {
		if metric.Collector.Type != "github-stars" {
			continue
		}
		value, err := collectGitHubStars(ctx, client, token, metric.Collector.Repo)
		if err != nil {
			refreshErrs = append(refreshErrs, fmt.Errorf("%s: %w", metric.ID, err))
			continue
		}
		o := Observation{Metric: metric.ID, At: now.Format(time.RFC3339), Value: value, Source: "github"}
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

func collectGitHubStars(ctx context.Context, client *http.Client, token, repo string) (float64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/"+repo, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return 0, fmt.Errorf("github returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var body struct {
		Stars float64 `json:"stargazers_count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return 0, err
	}
	return body.Stars, nil
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

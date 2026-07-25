package metrics

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yu-min3/kensan-lab/apps/feed/internal/application"
	"gopkg.in/yaml.v3"
)

type StarsCollector interface {
	Stars(context.Context, string) (float64, error)
}

type Runner struct {
	Root      string
	Collector StarsCollector
	Now       func() time.Time
}

type config struct {
	Version int          `yaml:"version"`
	Metrics []definition `yaml:"metrics"`
}

type definition struct {
	ID        string          `yaml:"id"`
	Label     string          `yaml:"label"`
	Unit      string          `yaml:"unit"`
	Target    *float64        `yaml:"target"`
	Direction string          `yaml:"direction"`
	Display   string          `yaml:"display"`
	Collector collectorConfig `yaml:"collector"`
}

type collectorConfig struct {
	Type string `yaml:"type"`
	Repo string `yaml:"repo,omitempty"`
}

type observation struct {
	Metric string  `json:"metric"`
	At     string  `json:"at"`
	Value  float64 `json:"value"`
	Source string  `json:"source"`
}

func (r Runner) UpdateAll(ctx context.Context) (application.MetricsResult, error) {
	if r.Root == "" || r.Collector == nil {
		return application.MetricsResult{}, fmt.Errorf("workspace root and stars collector are required")
	}
	paths, err := filepath.Glob(filepath.Join(r.Root, "projects", "*", "metrics.yaml"))
	if err != nil {
		return application.MetricsResult{}, err
	}
	now := time.Now()
	if r.Now != nil {
		now = r.Now()
	}
	result := application.MetricsResult{}
	var runErrs []error
	for _, path := range paths {
		cfg, err := loadConfig(path)
		if err != nil {
			runErrs = append(runErrs, fmt.Errorf("%s: %w", filepath.Base(filepath.Dir(path)), err))
			continue
		}
		result.Projects++
		for _, metric := range cfg.Metrics {
			if metric.Collector.Type != "github-stars" {
				continue
			}
			value, err := r.Collector.Stars(ctx, metric.Collector.Repo)
			if err != nil {
				runErrs = append(runErrs, fmt.Errorf("%s/%s: %w", filepath.Base(filepath.Dir(path)), metric.ID, err))
				continue
			}
			changed, err := appendObservation(filepath.Join(filepath.Dir(path), "metrics.ndjson"), observation{
				Metric: metric.ID, At: now.Format(time.RFC3339), Value: value, Source: "github",
			})
			if err != nil {
				runErrs = append(runErrs, fmt.Errorf("%s/%s: %w", filepath.Base(filepath.Dir(path)), metric.ID, err))
			} else if changed {
				result.Updated++
			}
		}
	}
	return result, errors.Join(runErrs...)
}

func loadConfig(path string) (config, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return config{}, err
	}
	var cfg config
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return config{}, fmt.Errorf("parse metrics.yaml: %w", err)
	}
	if cfg.Version != 1 {
		return config{}, fmt.Errorf("unsupported metrics version: %d", cfg.Version)
	}
	for _, metric := range cfg.Metrics {
		if metric.Collector.Type == "github-stars" && (metric.ID == "" || metric.Collector.Repo == "") {
			return config{}, fmt.Errorf("github-stars requires metric id and repo")
		}
	}
	return cfg, nil
}

func appendObservation(path string, next observation) (bool, error) {
	file, err := os.Open(path)
	if err == nil {
		scanner := bufio.NewScanner(file)
		day := next.At[:10]
		line := 0
		for scanner.Scan() {
			line++
			if strings.TrimSpace(scanner.Text()) == "" {
				continue
			}
			var previous observation
			if err := json.Unmarshal(scanner.Bytes(), &previous); err != nil {
				_ = file.Close()
				return false, fmt.Errorf("parse metrics.ndjson line %d: %w", line, err)
			}
			if previous.Metric == "" || previous.At == "" || previous.Source == "" {
				_ = file.Close()
				return false, fmt.Errorf("invalid metrics.ndjson line %d", line)
			}
			if previous.Metric == next.Metric && strings.HasPrefix(previous.At, day) && previous.Value == next.Value {
				_ = file.Close()
				return false, nil
			}
		}
		err = errors.Join(scanner.Err(), file.Close())
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	content, err := json.Marshal(next)
	if err != nil {
		return false, err
	}
	out, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return false, err
	}
	_, writeErr := fmt.Fprintf(out, "%s\n", content)
	return true, errors.Join(writeErr, out.Close())
}

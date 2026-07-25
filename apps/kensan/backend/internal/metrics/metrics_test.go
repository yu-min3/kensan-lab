package metrics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "projects", "demo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	config := `version: 1
metrics:
  - id: github-stars
    label: GitHub Stars
    unit: stars
    target: 100
    direction: increase
    display: integer
    collector:
      type: github-stars
      repo: example/demo
`
	if err := os.WriteFile(filepath.Join(dir, "metrics.yaml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, dir
}

func TestLoadBuildsView(t *testing.T) {
	root, dir := fixture(t)
	history := strings.Join([]string{
		`{"metric":"github-stars","at":"2026-06-15","value":2,"source":"github"}`,
		`{"metric":"github-stars","at":"2026-07-10","value":4,"source":"github"}`,
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(dir, "metrics.ndjson"), []byte(history), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	result, err := Load(root, "demo", now)
	if err != nil {
		t.Fatal(err)
	}
	metric := result.Metrics[0]
	if metric.Current == nil || *metric.Current != 4 || metric.Best == nil || metric.Best.Value != 4 {
		t.Fatalf("unexpected view: %+v", metric)
	}
	if metric.Delta.Previous == nil || *metric.Delta.Previous != 2 {
		t.Errorf("previous delta: %+v", metric.Delta)
	}
	if metric.Delta.Days30 == nil || *metric.Delta.Days30 != 2 {
		t.Errorf("30d delta: %+v", metric.Delta)
	}
	if len(metric.Series) != 2 {
		t.Errorf("series: %+v", metric.Series)
	}
}

func TestMissingConfigIsEmpty(t *testing.T) {
	result, err := Load(t.TempDir(), "demo", time.Now())
	if err != nil || len(result.Metrics) != 0 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestRefreshGitHubStarsAndDeduplicates(t *testing.T) {
	root, dir := fixture(t)
	github := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("missing token header")
		}
		_, _ = w.Write([]byte(`{"stargazers_count":7}`))
	}))
	defer github.Close()
	client := github.Client()
	client.Transport = rewriteTransport{base: github.URL, next: client.Transport}
	now := time.Date(2026, 7, 20, 8, 0, 0, 0, time.FixedZone("JST", 9*60*60))
	for range 2 {
		env := Env{HTTP: client, Secret: func(string) string { return "test-token" }}
		result, err := Refresh(context.Background(), root, "demo", now, env)
		if err != nil || result.Metrics[0].Current == nil || *result.Metrics[0].Current != 7 {
			t.Fatalf("result=%+v err=%v", result, err)
		}
	}
	b, err := os.ReadFile(filepath.Join(dir, "metrics.ndjson"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(strings.TrimSpace(string(b)), "\n") != 0 {
		t.Errorf("expected one line: %s", b)
	}
}

// HTTP を使わない collector（ベンチプレス記録のようなファイル系）が
// 同じ registry に乗ること、collector 固有キーが自前で読めることを確認する。
func TestRegisteredFileCollector(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "projects", "demo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	config := `version: 1
metrics:
  - id: best-set
    label: Best Set
    unit: kg
    direction: increase
    display: decimal
    collector:
      type: test-file
      path: log.txt
`
	if err := os.WriteFile(filepath.Join(dir, "metrics.yaml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "log.txt"), []byte("42.5"), 0o644); err != nil {
		t.Fatal(err)
	}
	Register("test-file", func(_ context.Context, cc CollectCtx) (Sample, error) {
		var cfg struct {
			Path string `yaml:"path"`
		}
		if err := cc.Config.Decode(&cfg); err != nil {
			return Sample{}, err
		}
		b, err := os.ReadFile(filepath.Join(cc.Root, "projects", cc.Project, cfg.Path))
		if err != nil {
			return Sample{}, err
		}
		v, err := strconv.ParseFloat(strings.TrimSpace(string(b)), 64)
		if err != nil {
			return Sample{}, err
		}
		return Sample{Value: v, Fields: map[string]any{"reps": 5}, Source: "log"}, nil
	})
	t.Cleanup(func() { delete(collectors, "test-file") })

	result, err := Refresh(context.Background(), root, "demo", time.Now(), Env{})
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if result.Metrics[0].Current == nil || *result.Metrics[0].Current != 42.5 {
		t.Fatalf("current: %+v", result.Metrics[0])
	}
	b, err := os.ReadFile(filepath.Join(dir, "metrics.ndjson"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"source":"log"`) || !strings.Contains(string(b), `"reps":5`) {
		t.Errorf("observation should carry collector source and fields: %s", b)
	}
}

// 未知の type は refresh でエラーになるが、既存履歴の表示は壊さない。
func TestUnknownCollectorTypeErrorsButKeepsView(t *testing.T) {
	root, dir := fixture(t)
	config := strings.Replace(
		`version: 1
metrics:
  - id: github-stars
    label: GitHub Stars
    unit: stars
    direction: increase
    display: integer
    collector:
      type: PLACEHOLDER
`, "PLACEHOLDER", "not-implemented-yet", 1)
	if err := os.WriteFile(filepath.Join(dir, "metrics.yaml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	history := `{"metric":"github-stars","at":"2026-07-10","value":4,"source":"github"}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "metrics.ndjson"), []byte(history), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Refresh(context.Background(), root, "demo", time.Now(), Env{})
	if err == nil || !strings.Contains(err.Error(), "not-implemented-yet") {
		t.Fatalf("expected unknown collector error, got %v", err)
	}
	if result.Metrics[0].Current == nil || *result.Metrics[0].Current != 4 {
		t.Errorf("existing history should still render: %+v", result.Metrics[0])
	}
}

type rewriteTransport struct {
	base string
	next http.RoundTripper
}

func (t rewriteTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r.URL.Scheme = "http"
	r.URL.Host = strings.TrimPrefix(t.base, "http://")
	return t.next.RoundTrip(r)
}

package evalrun

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/agentclient"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/evalcases"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

type fakeRuntime struct {
	mu       sync.Mutex
	creates  []agentclient.Request
	parents  []string
	outputs  map[string]json.RawMessage // by mode
	waitCase string
}

func (f *fakeRuntime) Create(ctx context.Context, r agentclient.Request) (agentclient.Job, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.creates = append(f.creates, r)
	in, _ := json.Marshal(r.Input)
	id := "job-" + string(rune('a'+len(f.creates)))
	if f.waitCase != "" && strings.Contains(string(in), f.waitCase) {
		return agentclient.Job{ID: id, State: "waiting", Reason: "quota_wait"}, nil
	}
	return agentclient.Job{ID: id, State: "queued"}, nil
}
func (f *fakeRuntime) Get(_ context.Context, id string) (agentclient.Job, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, r := range f.creates {
		if "job-"+string(rune('a'+i+1)) == id {
			in, _ := json.Marshal(r.Input)
			if f.waitCase != "" && strings.Contains(string(in), f.waitCase) {
				return agentclient.Job{ID: id, State: "waiting", Reason: "quota_wait"}, nil
			}
			return agentclient.Job{ID: id, State: "succeeded", Output: f.outputs["nikujaga"]}, nil
		}
	}
	return agentclient.Job{}, nil
}
func (f *fakeRuntime) Action(context.Context, string, string) (agentclient.Job, error) {
	return agentclient.Job{}, nil
}

type fakeLF struct {
	items  []DatasetItem
	scores []Score
}

func (f *fakeLF) EnsureDataset(context.Context, string, string, map[string]any) (string, error) {
	return "ds-1", nil
}
func (f *fakeLF) UpsertItem(_ context.Context, i DatasetItem) error {
	f.items = append(f.items, i)
	return nil
}
func (f *fakeLF) Scores(_ context.Context, s []Score) error {
	f.scores = append(f.scores, s...)
	return nil
}

func pick(t *testing.T, ids ...string) []evalcases.Case {
	s, err := evalcases.Load()
	if err != nil {
		t.Fatal(err)
	}
	var out []evalcases.Case
	for _, c := range s.Cases {
		for _, id := range ids {
			if c.ID == id {
				out = append(out, c)
			}
		}
	}
	return out
}

func TestRunGradesCachesAndRecords(t *testing.T) {
	rec := tracetest.NewSpanRecorder()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec)))
	otel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() { otel.SetTracerProvider(sdktrace.NewTracerProvider()) })
	good, _ := json.Marshal(map[string]any{"title": "肉じゃが", "servings": "2人分", "ingredients": []string{"じゃがいも 3個", "牛こま切れ肉 150g", "玉ねぎ 1個", "醤油 大さじ2", "砂糖 大さじ1", "水 200ml"}, "steps": []string{"じゃがいもと玉ねぎを一口大に切る。", "鍋で肉を炒め、野菜を加えて炒める。", "水・砂糖・醤油を入れて落とし蓋をし、15分煮る。"}, "assessment": ""})
	rt := &fakeRuntime{outputs: map[string]json.RawMessage{"nikujaga": good}, waitCase: "ほうれん草"}
	lf := &fakeLF{}
	cases := pick(t, "basic-nikujaga", "unknown-servings")
	cfg := Config{Name: "baseline", CacheDir: t.TempDir(), DatasetID: "ds-1", Poll: time.Millisecond, Timeout: 50 * time.Millisecond}
	rep, err := Run(context.Background(), rt, lf, cases, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Summary.Total != 2 || rep.Summary.Passed != 1 || rep.Summary.NotEvaluated != 1 {
		t.Fatalf("%+v", rep.Summary)
	}
	roots := map[string]sdktrace.ReadOnlySpan{}
	for _, sp := range rec.Ended() {
		if sp.Name() == "konro.eval.import" {
			roots[sp.SpanContext().TraceID().String()] = sp
		}
	}
	if len(roots) != 2 {
		t.Fatalf("want one trace per case, got %d", len(roots))
	}
	for _, sc := range lf.scores {
		root, ok := roots[sc.TraceID]
		if !ok || sc.ObservationID != root.SpanContext().SpanID().String() || !sc.Timestamp.Equal(rep.StartedAt) {
			t.Fatalf("score not bound to its root span: %+v", sc)
		}
	}
	if len(lf.scores) != 3 { // status for both, numeric only for the evaluated case
		t.Fatalf("scores %d", len(lf.scores))
	}
	for _, r := range rt.creates {
		if r.Traceparent != "" {
			t.Fatal("runner must not set traceparent itself; the client derives it from ctx")
		}
	}
	// Second run with the same prompt reuses the succeeded output without a model call.
	before := len(rt.creates)
	rep2, err := Run(context.Background(), rt, nil, pick(t, "basic-nikujaga"), cfg)
	if err != nil || len(rt.creates) != before || !rep2.Results[0].Cached || rep2.Results[0].Status != "passed" {
		t.Fatalf("cache: creates %d→%d %+v %v", before, len(rt.creates), rep2.Results, err)
	}
	// A different prompt is a different experiment input: no cache hit.
	cfg.Prompts = map[string]string{"steps": "別の指示"}
	if _, err = Run(context.Background(), rt, nil, pick(t, "basic-nikujaga"), cfg); err != nil || len(rt.creates) != before+1 {
		t.Fatal("candidate prompt reused the baseline cache")
	}
	if !strings.Contains(Compare(rep, rep2), "basic-nikujaga") {
		t.Fatal("compare")
	}
}

func TestSyncUsesStableItemIDs(t *testing.T) {
	s, _ := evalcases.Load()
	lf := &fakeLF{}
	if _, err := Sync(context.Background(), lf, s); err != nil {
		t.Fatal(err)
	}
	if len(lf.items) != 20 || lf.items[0].ID != "konro-import/basic-nikujaga" {
		t.Fatalf("%d %+v", len(lf.items), lf.items[0].ID)
	}
}

func TestHTTPLangfuseShapes(t *testing.T) {
	var paths []string
	var scoreBody []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok || u != "pk" || p != "sk" {
			w.WriteHeader(401)
			return
		}
		paths = append(paths, r.Method+" "+r.URL.Path)
		switch {
		case r.Method == "GET":
			w.WriteHeader(404)
		case r.URL.Path == "/api/public/v2/datasets":
			_, _ = w.Write([]byte(`{"id":"ds-9"}`))
		case r.URL.Path == "/api/public/scores":
			_ = json.NewDecoder(r.Body).Decode(&scoreBody)
			w.WriteHeader(202)
			_, _ = w.Write([]byte(`{"message":"Accepted"}`))
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()
	lf, err := NewHTTPLangfuse(srv.URL, "pk", "sk")
	if err != nil {
		t.Fatal(err)
	}
	id, err := lf.EnsureDataset(context.Background(), "konro-import", "", nil)
	if err != nil || id != "ds-9" {
		t.Fatal(id, err)
	}
	if err = lf.Scores(context.Background(), []Score{{ID: "x", TraceID: "t", Name: "status", Value: "passed", DataType: "CATEGORICAL"}}); err != nil {
		t.Fatal(err)
	}
	if len(scoreBody) != 1 || scoreBody[0]["value"] != "passed" {
		t.Fatalf("scores must be sent as an array: %v", scoreBody)
	}
	want := "GET /api/public/v2/datasets/konro-import,POST /api/public/v2/datasets,POST /api/public/scores"
	if strings.Join(paths, ",") != want {
		t.Fatalf("%v", paths)
	}
}

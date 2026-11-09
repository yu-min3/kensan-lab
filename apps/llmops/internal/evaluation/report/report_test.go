package report

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

func example(t *testing.T) Comparison {
	t.Helper()
	f, e := os.Open("testdata/comparison.json")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	c, e := Decode(f)
	if e != nil {
		t.Fatal(e)
	}
	return c
}
func TestFixtureOutputAndEscape(t *testing.T) {
	c := example(t)
	var output bytes.Buffer
	if e := Write(&output, c); e != nil {
		t.Fatal(e)
	}
	html := output.String()
	for _, want := range []string{"回帰", "改善", "結果未記録", "人の確認待ち", "全 4 件 / 採点対象 4 件", "全 4 件 / 採点対象 2 件", "合成fixture注釈", "未確認注釈", "quota_wait", "grader-demo", "67890", "未取得", "fixture実行", "&lt;script&gt;"} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %q", want)
		}
	}
	for _, bad := range []string{"<script>", `<img src=`, "<iframe", "<link ", "<script src="} {
		if strings.Contains(html, bad) {
			t.Errorf("active content %s", bad)
		}
	}
	if path := os.Getenv("LLMOPS_REPORT_EXPORT"); path != "" {
		if e := os.WriteFile(path, output.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
	}
}
func TestManifestMismatchIsRejectedBeforeWriting(t *testing.T) {
	for _, mutate := range []func(*Comparison){
		func(c *Comparison) { c.SchemaVersion = "2" }, func(c *Comparison) { c.Candidate.DatasetReleaseID = "different" }, func(c *Comparison) { c.Candidate.EvaluatorVersion = "different" }, func(c *Comparison) { c.Cases[0].Candidate.RunID = "wrong-run" }, func(c *Comparison) { c.Cases[0].Current.Evaluation.AttemptID = "wrong-attempt" }, func(c *Comparison) { c.Cases[0].Current.Evaluation.Mode = "raw" }, func(c *Comparison) { c.Cases[0].Current.Evaluation.ExecutionState = "waiting" }, func(c *Comparison) { c.Dataset.Snapshots[0].ID = "latest" }, func(c *Comparison) { c.Cases = append(c.Cases, c.Cases[0]) }, func(c *Comparison) { c.Cases[3].Current.Evaluation.Status = "passed" },
	} {
		c := example(t)
		mutate(&c)
		var output bytes.Buffer
		if e := Write(&output, c); e == nil || output.Len() != 0 {
			t.Fatal("invalid manifest rendered")
		}
	}
}
func TestDecodeRejectsUnknownAndTrailingJSON(t *testing.T) {
	b, e := os.ReadFile("testdata/comparison.json")
	if e != nil {
		t.Fatal(e)
	}
	var m map[string]any
	json.Unmarshal(b, &m)
	m["command"] = "shell"
	unknown, _ := json.Marshal(m)
	for _, raw := range [][]byte{unknown, []byte(strings.Replace(string(b), `"schema_version": "1"`, `"schema_version":"2","schema_version":"1"`, 1)), append(b, []byte(` {}`)...), bytes.Repeat([]byte("x"), MaxManifestBytes+1)} {
		if _, e := Decode(bytes.NewReader(raw)); e == nil {
			t.Fatal("invalid JSON accepted")
		}
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
func TestOutputWriteFailure(t *testing.T) {
	if e := Write(failingWriter{}, example(t)); e == nil {
		t.Fatal("write failure hidden")
	}
}

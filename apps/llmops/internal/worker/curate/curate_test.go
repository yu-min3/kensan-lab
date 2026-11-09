package curate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/yu-min3/kensan-lab/apps/llmops/internal/objects"
	"github.com/yu-min3/kensan-lab/apps/llmops/internal/worker"
	"strings"
	"testing"
)

type reader struct {
	capture objects.Capture
	blob    objects.Blob
}

func (r reader) AppID() string                                               { return "konro" }
func (r reader) GetCapture(context.Context, string) (objects.Capture, error) { return r.capture, nil }
func (r reader) GetArtifact(context.Context, string) (objects.Blob, error)   { return r.blob, nil }

type caller struct {
	role   string
	calls  int
	result json.RawMessage
}

func (c *caller) App() string  { return "konro" }
func (c *caller) Role() string { return c.role }
func (c *caller) Call(_ context.Context, _ string, r worker.Request) (worker.Response, error) {
	c.calls++
	return worker.Response{ProtocolVersion: "1", RequestID: r.RequestID, Result: c.result}, nil
}
func fixture() (reader, *caller, worker.Adapter, []Input) {
	raw := []byte("original HTML")
	hash := sha256.Sum256(raw)
	id := strings.Repeat("a", 32)
	r := reader{objects.Capture{SchemaVersion: "1", AppID: "konro", CaptureID: id, MIME: "text/html", Object: objects.Reference{ObjectID: "raw_" + id, AppID: "konro", SHA256: hex.EncodeToString(hash[:]), Size: int64(len(raw))}}, objects.Blob{Bytes: raw, MIME: "text/html"}}
	c := &caller{role: "transform", result: json.RawMessage(`{"text":"text","text_bytes":4,"text_truncated":true,"content":{"title":"Soup","ingredients":["onion 1/2"],"steps":["Slice"]},"output_schema":{"type":"object"},"warnings":[]}`)}
	return r, c, worker.Adapter{DomainSchemaVersion: "konro.recipe-import.v1", SHA256: strings.Repeat("b", 64)}, []Input{{CaptureID: id}}
}
func TestProvenanceAndReplayIdentity(t *testing.T) {
	r, c, a, in := fixture()
	rows, e := Prepare(context.Background(), c, r, "konro", a, "v1", in)
	if e != nil {
		t.Fatal(e)
	}
	if len(rows) != 1 || rows[0].RawSHA256 != r.capture.Object.SHA256 || rows[0].TextTruncated != true || rows[0].CaptureID != in[0].CaptureID || rows[0].SourceGroup == "" {
		t.Fatal("lineage missing")
	}
	again, e := Prepare(context.Background(), c, r, "konro", a, "v1", in)
	if e != nil || again[0].RecordID != rows[0].RecordID {
		t.Fatal("replay identity changed")
	}
	changed, _ := Prepare(context.Background(), c, r, "konro", a, "v2", in)
	if changed[0].RecordID == rows[0].RecordID {
		t.Fatal("transform versions conflated")
	}
}
func TestCorruptRawCannotReachAdapter(t *testing.T) {
	r, c, a, in := fixture()
	r.blob.Bytes = []byte("tampered raw")
	_, e := Prepare(context.Background(), c, r, "konro", a, "v1", in)
	if !errors.Is(e, objects.ErrCorrupt) || c.calls != 0 {
		t.Fatal("corrupt raw reached adapter")
	}
}
func TestWrongAppAndRoleRejected(t *testing.T) {
	r, c, a, in := fixture()
	r.capture.AppID = "sample"
	if _, e := Prepare(context.Background(), c, r, "konro", a, "v1", in); e == nil || c.calls != 0 {
		t.Fatal("other app accepted")
	}
	r, c, a, in = fixture()
	c.role = "eval"
	if _, e := Prepare(context.Background(), c, r, "konro", a, "v1", in); e == nil {
		t.Fatal("eval can curate")
	}
}
func TestMalformedPreparedOutputRejected(t *testing.T) {
	r, c, a, in := fixture()
	c.result = json.RawMessage(`{"text":"x","text_bytes":100,"content":{},"output_schema":{}}`)
	if _, e := Prepare(context.Background(), c, r, "konro", a, "v1", in); e == nil {
		t.Fatal("wrong byte count accepted")
	}
	r, c, a, in = fixture()
	in = append(in, in[0])
	if _, e := Prepare(context.Background(), c, r, "konro", a, "v1", in); e == nil {
		t.Fatal("duplicate capture accepted")
	}
}

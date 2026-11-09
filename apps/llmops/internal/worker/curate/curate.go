// Package curate builds provenance-preserving rows from published captures.
package curate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	"github.com/yu-min3/kensan-lab/apps/llmops/internal/objects"
	"github.com/yu-min3/kensan-lab/apps/llmops/internal/worker"
	"github.com/yu-min3/kensan-lab/apps/llmops/internal/worker/catalog"
)

var ErrPrepare = errors.New("curation failed")

type Input struct {
	CaptureID   string `json:"capture_id"`
	SourceGroup string `json:"source_group"`
}
type CaptureReader interface {
	GetCapture(context.Context, string) (objects.Capture, error)
	GetArtifact(context.Context, string) (objects.Blob, error)
	AppID() string
}
type AdapterCaller interface {
	Call(context.Context, string, worker.Request) (worker.Response, error)
	App() string
	Role() string
}

func Prepare(ctx context.Context, r AdapterCaller, reader CaptureReader, adapterID string, adapter worker.Adapter, transformVersion string, inputs []Input) ([]catalog.Row, error) {
	if r.Role() != "transform" || reader.AppID() != r.App() || strings.TrimSpace(transformVersion) == "" || strings.ContainsRune(transformVersion, 0) || len(transformVersion) > 256 || adapter.DomainSchemaVersion != "konro.recipe-import.v1" || len(inputs) == 0 || len(inputs) > 100 {
		return nil, worker.ErrConfig
	}
	if b, e := hex.DecodeString(adapter.SHA256); e != nil || len(b) != 32 {
		return nil, worker.ErrConfig
	}
	rows := make([]catalog.Row, 0, len(inputs))
	seen := map[string]bool{}
	for _, in := range inputs {
		if seen[in.CaptureID] || len(in.SourceGroup) > 256 || strings.ContainsRune(in.SourceGroup, 0) {
			return nil, worker.ErrRequest
		}
		seen[in.CaptureID] = true
		capture, e := reader.GetCapture(ctx, in.CaptureID)
		if e != nil {
			return nil, e
		}
		if capture.AppID != r.App() || capture.Object.AppID != r.App() || capture.CaptureID != in.CaptureID {
			return nil, ErrPrepare
		}
		raw, e := reader.GetArtifact(ctx, capture.Object.ObjectID)
		if e != nil {
			return nil, e
		}
		digest := sha256.Sum256(raw.Bytes)
		if hex.EncodeToString(digest[:]) != capture.Object.SHA256 || int64(len(raw.Bytes)) != capture.Object.Size || raw.MIME != capture.MIME {
			return nil, objects.ErrCorrupt
		}
		payload, _ := json.Marshal(map[string]any{"source_uri": capture.SourceURI, "mime_type": capture.MIME, "raw": string(raw.Bytes)})
		res, e := r.Call(ctx, adapterID, worker.Request{ProtocolVersion: "1", RequestID: "capture-" + capture.CaptureID, Operation: "prepare", DomainSchemaVersion: adapter.DomainSchemaVersion, Payload: payload})
		if e != nil {
			return nil, e
		}
		if res.Error != nil {
			return nil, ErrPrepare
		}
		var prepared struct {
			Text          string          `json:"text"`
			TextBytes     int             `json:"text_bytes"`
			TextTruncated bool            `json:"text_truncated"`
			Content       json.RawMessage `json:"content"`
			Warnings      []string        `json:"warnings"`
			OutputSchema  json.RawMessage `json:"output_schema"`
		}
		if json.Unmarshal(res.Result, &prepared) != nil || prepared.TextBytes != len(prepared.Text) || !json.Valid(prepared.Content) || !json.Valid(prepared.OutputSchema) {
			return nil, ErrPrepare
		}
		if strings.TrimSpace(string(prepared.Content)) == "null" || strings.TrimSpace(string(prepared.OutputSchema)) == "null" {
			return nil, ErrPrepare
		}
		group := in.SourceGroup
		if group == "" {
			group = "sha256-" + capture.Object.SHA256
		}
		version := adapter.DomainSchemaVersion
		identity := sha256.Sum256([]byte(strings.Join([]string{r.App(), capture.CaptureID, capture.Object.SHA256, group, transformVersion, adapter.DomainSchemaVersion, adapter.SHA256}, "\x00")))
		if prepared.Warnings == nil {
			prepared.Warnings = []string{}
		}
		rows = append(rows, catalog.Row{RecordID: hex.EncodeToString(identity[:]), AppID: r.App(), CaptureID: capture.CaptureID, RawObjectID: capture.Object.ObjectID, RawSHA256: capture.Object.SHA256, SourceURI: capture.SourceURI, SourceGroup: group, SchemaVersion: version, TransformVersion: transformVersion, AdapterSHA256: adapter.SHA256, PreparedText: prepared.Text, TextBytes: prepared.TextBytes, TextTruncated: prepared.TextTruncated, Content: prepared.Content, OutputSchema: prepared.OutputSchema, Warnings: prepared.Warnings})
	}
	return rows, nil
}

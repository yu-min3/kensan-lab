package api

import (
	"encoding/json"
	"io"
	"mime"
	"net/http"

	"github.com/yu-min3/kensan-lab/apps/llmops/internal/objects"
	"github.com/yu-min3/kensan-lab/apps/llmops/internal/policy"
)

const MaxFeedbackBytes int64 = 256 << 10

func (h *CaptureFeedback) feedback(w http.ResponseWriter, r *http.Request) {
	p, ok := Principal(r)
	if !ok {
		writeError(w, 401, "unauthorized")
		return
	}
	typ, _, e := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if e != nil || typ != "application/json" {
		writeError(w, 400, "invalid_request")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, MaxFeedbackBytes)
	b, e := io.ReadAll(r.Body)
	if e != nil {
		partError(w, e)
		return
	}
	body, e := decodeObject(b, "feedback-request")
	if e != nil {
		writeError(w, 400, "invalid_request")
		return
	}
	if _, e = h.policy.AuthorizeCreate(r.Context(), p, policy.FeedbackCreate, body); e != nil {
		storageError(w, e)
		return
	}
	store, ok := h.stores[p.AppID]
	if !ok {
		writeError(w, 503, "unavailable")
		return
	}
	// Verify byte references, in addition to policy's resource ownership lookup.
	for _, field := range []string{"before", "after"} {
		if raw, ok := body[field]; ok {
			encoded, _ := json.Marshal(raw)
			var ref objects.Reference
			if json.Unmarshal(encoded, &ref) != nil {
				writeError(w, 400, "invalid_request")
				return
			}
			blob, e := store.GetArtifact(r.Context(), ref.ObjectID)
			if e != nil {
				storageError(w, e)
				return
			}
			if ref.AppID != p.AppID || ref.SHA256 != hashBytes(blob.Bytes) || ref.Size != int64(len(blob.Bytes)) {
				writeError(w, 400, "invalid_request")
				return
			}
		}
	}
	delete(body, "app_id")
	requestBytes, _ := json.Marshal(struct {
		Subject, Role string
		Body          map[string]any
	}{p.Subject, p.Role, body})
	record := make(map[string]any, len(body)+5)
	for k, v := range body {
		record[k] = v
	}
	record["schema_version"] = "1"
	record["app_id"] = p.AppID
	record["annotation_status"] = "unreviewed"
	record["created_at"] = h.now().UTC()
	data, _ := json.Marshal(record)
	published, _, e := store.PublishRecord(r.Context(), "feedback", body["event_id"].(string), requestBytes, data)
	if e != nil {
		storageError(w, e)
		return
	}
	writeJSON(w, 202, json.RawMessage(published.Data))
}

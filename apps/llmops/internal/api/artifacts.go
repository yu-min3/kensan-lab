package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/yu-min3/kensan-lab/apps/llmops/internal/identity"
	"github.com/yu-min3/kensan-lab/apps/llmops/internal/jobs"
	"github.com/yu-min3/kensan-lab/apps/llmops/internal/objects"
	"github.com/yu-min3/kensan-lab/apps/llmops/internal/policy"
	runtimeclient "github.com/yu-min3/kensan-lab/apps/llmops/internal/runtime"
)

type storedResult struct {
	Owner     jobs.Owner           `json:"owner"`
	Kind      string               `json:"kind"`
	JobID     string               `json:"job_id"`
	RuntimeID string               `json:"runtime_id"`
	Output    json.RawMessage      `json:"output,omitempty"`
	Model     *string              `json:"model"`
	Usage     *runtimeclient.Usage `json:"usage"`
}

func (h *JobAPI) saveResult(ctx context.Context, r jobs.Record, j runtimeclient.Job) error {
	store, ok := h.objects[r.Admission.Owner.App]
	if !ok {
		return objects.ErrUnavailable
	}
	request, _ := json.Marshal(map[string]string{"job_id": r.ID, "runtime_id": j.ID})
	data, _ := json.Marshal(storedResult{r.Admission.Owner, r.Admission.Kind, r.ID, j.ID, j.Output, j.Model, j.Usage})
	_, _, e := store.PublishRecord(ctx, "job-results", r.ID, request, data)
	return e
}
func (h *JobAPI) result(ctx context.Context, p identity.Principal, id string) (storedResult, error) {
	if !strings.HasPrefix(id, "job_") {
		return storedResult{}, policy.ErrNotFound
	}
	jobID := strings.TrimPrefix(id, "job_")
	r, e := h.store.Get(owner(p), jobID)
	if e != nil {
		if errors.Is(e, jobs.ErrNotFound) {
			return storedResult{}, policy.ErrNotFound
		}
		return storedResult{}, policy.ErrUnavailable
	}
	store, ok := h.objects[p.AppID]
	if !ok {
		return storedResult{}, policy.ErrNotFound
	}
	record, e := store.GetRecord(ctx, "job-results", jobID)
	if e != nil {
		if errors.Is(e, objects.ErrNotFound) {
			return storedResult{}, policy.ErrNotFound
		}
		return storedResult{}, policy.ErrUnavailable
	}
	var result storedResult
	if json.Unmarshal(record.Data, &result) != nil || result.Owner != r.Admission.Owner || result.Kind != r.Admission.Kind || result.JobID != r.ID || result.RuntimeID != r.RuntimeJobID {
		return storedResult{}, policy.ErrUnavailable
	}
	return result, nil
}

// Lookup handles only stored job-result artifacts; chain it after the raw/job
// metadata resolver. IDs in requests never become object storage paths.
func (h *JobAPI) Lookup(ctx context.Context, p identity.Principal, typ, id string) (policy.Resource, error) {
	if typ != "artifact" {
		return policy.Resource{}, policy.ErrNotFound
	}
	result, e := h.result(ctx, p, id)
	if e != nil {
		return policy.Resource{}, e
	}
	return policy.Resource{ID: id, Type: "artifact", AppID: result.Owner.App, OwnerRole: result.Owner.Role, OwnerSubject: result.Owner.Subject, Kind: result.Kind}, nil
}
func (h *JobAPI) artifact(w http.ResponseWriter, r *http.Request) {
	p, _ := Principal(r)
	id := r.PathValue("id")
	if _, e := h.policy.AuthorizeResource(r.Context(), p, policy.ArtifactRead, id); e != nil {
		jobError(w, e)
		return
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.Header().Set("Content-Disposition", "attachment")
	if strings.HasPrefix(id, "job_") {
		result, e := h.result(r.Context(), p, id)
		if e != nil {
			jobError(w, e)
			return
		}
		writeJSON(w, 200, map[string]any{"job_id": result.JobID, "output": result.Output, "model": result.Model, "usage": result.Usage})
		return
	}
	store, ok := h.objects[p.AppID]
	if !ok {
		writeError(w, 404, "not_found")
		return
	}
	blob, e := store.GetArtifact(r.Context(), id)
	if e != nil {
		storageError(w, e)
		return
	}
	w.Header().Set("Content-Type", blob.MIME)
	w.WriteHeader(200)
	_, _ = w.Write(blob.Bytes)
}

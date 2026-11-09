package api

import (
	"bytes"
	"errors"
	"io"
	"mime"
	"net/http"

	"github.com/yu-min3/kensan-lab/apps/llmops/internal/objects"
	"github.com/yu-min3/kensan-lab/apps/llmops/internal/policy"
)

const metadataLimit int64 = 16 << 10
const captureRequestLimit = objects.MaxRawBytes + (64 << 10)

func (h *CaptureFeedback) capture(w http.ResponseWriter, r *http.Request) {
	p, ok := Principal(r)
	if !ok {
		writeError(w, 401, "unauthorized")
		return
	}
	typ, _, e := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if e != nil || typ != "multipart/form-data" {
		writeError(w, 400, "invalid_request")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, captureRequestLimit)
	reader, e := r.MultipartReader()
	if e != nil {
		writeError(w, 400, "invalid_request")
		return
	}
	parts := map[string][]byte{}
	for {
		part, e := reader.NextPart()
		if e == io.EOF {
			break
		}
		if e != nil {
			partError(w, e)
			return
		}
		name := part.FormName()
		limit := objects.MaxRawBytes
		if name == "metadata" {
			limit = metadataLimit
			partType, _, e := mime.ParseMediaType(part.Header.Get("Content-Type"))
			if e != nil || partType != "application/json" {
				part.Close()
				writeError(w, 400, "invalid_request")
				return
			}
		} else if name != "source" {
			part.Close()
			writeError(w, 400, "invalid_request")
			return
		}
		if _, exists := parts[name]; exists {
			part.Close()
			writeError(w, 400, "invalid_request")
			return
		}
		b, e := io.ReadAll(io.LimitReader(part, limit+1))
		part.Close()
		if e != nil {
			partError(w, e)
			return
		}
		if int64(len(b)) > limit {
			writeError(w, 413, "too_large")
			return
		}
		parts[name] = b
	}
	meta, ok := parts["metadata"]
	if !ok {
		writeError(w, 400, "invalid_request")
		return
	}
	source, ok := parts["source"]
	if !ok {
		writeError(w, 400, "invalid_request")
		return
	}
	body, e := decodeObject(meta, "capture-metadata")
	if e != nil {
		writeError(w, 400, "invalid_request")
		return
	}
	if _, e = h.policy.AuthorizeCreate(r.Context(), p, policy.CaptureCreate, body); e != nil {
		storageError(w, e)
		return
	}
	store, ok := h.stores[p.AppID]
	if !ok {
		writeError(w, 503, "unavailable")
		return
	}
	in := objects.CaptureInput{Key: body["capture_key"].(string), MIME: body["mime"].(string), SHA256: body["sha256"].(string), OwnerSubject: p.Subject, OwnerRole: p.Role}
	if v, ok := body["source_uri"].(string); ok {
		in.SourceURI = v
	}
	record, _, e := store.Capture(r.Context(), in, bytes.NewReader(source))
	if e != nil {
		storageError(w, e)
		return
	}
	writeJSON(w, 202, record)
}
func partError(w http.ResponseWriter, e error) {
	var tooLarge *http.MaxBytesError
	if errors.As(e, &tooLarge) {
		writeError(w, 413, "too_large")
	} else {
		writeError(w, 400, "invalid_request")
	}
}

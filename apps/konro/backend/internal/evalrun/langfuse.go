package evalrun

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Langfuse is the small part of the Langfuse public API the runner needs.
// Traces do not go through it: they are exported over OTLP via the collector.
type Langfuse interface {
	EnsureDataset(ctx context.Context, name, description string, metadata map[string]any) (string, error)
	UpsertItem(ctx context.Context, item DatasetItem) error
	Scores(ctx context.Context, scores []Score) error
}

type DatasetItem struct {
	ID             string         `json:"id"`
	DatasetName    string         `json:"datasetName"`
	Input          any            `json:"input"`
	ExpectedOutput any            `json:"expectedOutput"`
	Metadata       map[string]any `json:"metadata"`
}

// Score overwrites an earlier score only when id, name and date all match, so
// callers pass a stable id and the run's start time.
type Score struct {
	ID            string         `json:"id"`
	TraceID       string         `json:"traceId"`
	ObservationID string         `json:"observationId,omitempty"`
	Name          string         `json:"name"`
	Value         any            `json:"value"`
	DataType      string         `json:"dataType"`
	Comment       string         `json:"comment,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
	Timestamp     time.Time      `json:"timestamp"`
	Environment   string         `json:"environment,omitempty"`
}

type HTTPLangfuse struct {
	Base, Public, Secret string
	Client               *http.Client
}

func NewHTTPLangfuse(base, public, secret string) (*HTTPLangfuse, error) {
	u, err := url.Parse(base)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || public == "" || secret == "" {
		return nil, errors.New("LANGFUSE_HOST, LANGFUSE_PUBLIC_KEY and LANGFUSE_SECRET_KEY are required")
	}
	return &HTTPLangfuse{Base: strings.TrimRight(base, "/"), Public: public, Secret: secret, Client: &http.Client{Timeout: 20 * time.Second}}, nil
}

func (l *HTTPLangfuse) do(ctx context.Context, method, path string, in, out any) (int, error) {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return 0, err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, l.Base+path, body)
	if err != nil {
		return 0, err
	}
	req.SetBasicAuth(l.Public, l.Secret)
	req.Header.Set("Content-Type", "application/json")
	res, err := l.Client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("langfuse %s %s: %w", method, path, err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 300 {
		return res.StatusCode, fmt.Errorf("langfuse %s %s: HTTP %d", method, path, res.StatusCode)
	}
	if out != nil && len(b) > 0 {
		if err := json.Unmarshal(b, out); err != nil {
			return res.StatusCode, err
		}
	}
	return res.StatusCode, nil
}

// EnsureDataset reads the dataset by name and creates it only when missing;
// the create endpoint is not an upsert.
func (l *HTTPLangfuse) EnsureDataset(ctx context.Context, name, description string, metadata map[string]any) (string, error) {
	var d struct {
		ID string `json:"id"`
	}
	status, err := l.do(ctx, "GET", "/api/public/v2/datasets/"+url.PathEscape(name), nil, &d)
	if err == nil && d.ID != "" {
		return d.ID, nil
	}
	if status != http.StatusNotFound {
		return "", err
	}
	if _, err = l.do(ctx, "POST", "/api/public/v2/datasets", map[string]any{"name": name, "description": description, "metadata": metadata}, &d); err != nil {
		return "", err
	}
	if d.ID == "" {
		return "", errors.New("langfuse returned no dataset id")
	}
	return d.ID, nil
}

func (l *HTTPLangfuse) UpsertItem(ctx context.Context, item DatasetItem) error {
	_, err := l.do(ctx, "POST", "/api/public/dataset-items", item, nil)
	return err
}

// Scores sends one batch. A 207 partial result is reported, not retried.
func (l *HTTPLangfuse) Scores(ctx context.Context, scores []Score) error {
	if len(scores) == 0 {
		return nil
	}
	var res struct {
		Rejected int `json:"rejected"`
	}
	status, err := l.do(ctx, "POST", "/api/public/scores", scores, &res)
	if err != nil {
		return err
	}
	if status == http.StatusMultiStatus || res.Rejected > 0 {
		return fmt.Errorf("langfuse rejected %d scores", res.Rejected)
	}
	return nil
}

// A non-recipe adapter proving that the shared runner has no recipe schema.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/yu-min3/kensan-lab/apps/llmops/contracts"
	"github.com/yu-min3/kensan-lab/apps/llmops/internal/worker"
	"io"
	"os"
	"strings"
)

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, "sample adapter rejected input")
		os.Exit(1)
	}
}
func run() error {
	b, e := io.ReadAll(io.LimitReader(os.Stdin, 1<<20+1))
	if e != nil || len(b) > 1<<20 {
		return worker.ErrRequest
	}
	var envelope any
	if json.Unmarshal(b, &envelope) != nil || contracts.Validate("adapter-request", envelope) != nil {
		return worker.ErrRequest
	}
	var req worker.Request
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&req) != nil || d.Decode(new(any)) != io.EOF || req.ProtocolVersion != "1" || req.DomainSchemaVersion != "sample-text/1" {
		return worker.ErrRequest
	}
	var payload struct {
		Text          string `json:"text"`
		ExpectedWords *int   `json:"expected_words,omitempty"`
	}
	d = json.NewDecoder(bytes.NewReader(req.Payload))
	d.DisallowUnknownFields()
	if d.Decode(&payload) != nil || d.Decode(new(any)) != io.EOF {
		return worker.ErrRequest
	}
	count := len(strings.Fields(payload.Text))
	var result any
	switch req.Operation {
	case "prepare":
		result = map[string]any{"word_count": count, "text": payload.Text}
	case "grade":
		if payload.ExpectedWords == nil {
			return worker.ErrRequest
		}
		result = map[string]any{"passed": count == *payload.ExpectedWords, "word_count": count}
	default:
		return worker.ErrRequest
	}
	out, e := json.Marshal(result)
	if e != nil {
		return e
	}
	return json.NewEncoder(os.Stdout).Encode(worker.Response{ProtocolVersion: "1", RequestID: req.RequestID, Result: out})
}

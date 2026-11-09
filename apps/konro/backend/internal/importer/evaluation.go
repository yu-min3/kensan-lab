package importer

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"unicode/utf8"

	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/webpage"
)

// SourceFromHTML prepares an already captured page without fetching its URL.
// The same bounded extraction is used by online imports and offline evaluation.
func SourceFromHTML(sourceURI, raw string) (Source, error) {
	if len(raw) > MaxSource || !utf8.ValidString(raw) {
		return Source{}, errors.New("invalid captured HTML")
	}
	return sourceFromPage(webpage.Page{URL: sourceURI, HTML: raw})
}

// DecodeAgentOutput is the pure output check shared with online job application.
// Completeness is checked separately by Validate before formal recipe saving.
func DecodeAgentOutput(raw []byte) (Content, string, error) {
	var output struct {
		Content
		Assessment string `json:"assessment"`
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(raw, &fields)
	for _, name := range []string{"title", "servings", "ingredients", "steps"} {
		value, ok := fields[name]
		if !ok || string(bytes.TrimSpace(value)) == "null" {
			return Content{}, "", errors.New("missing recipe field")
		}
	}
	if dec.Decode(&output) != nil || dec.Decode(new(any)) != io.EOF || validateContent(output.Content, false) != nil || utf8.RuneCountInString(output.Assessment) > 600 {
		return Content{}, "", errors.New("invalid recipe output")
	}
	return output.Content, output.Assessment, nil
}

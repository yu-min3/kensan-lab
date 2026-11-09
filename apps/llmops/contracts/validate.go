// Package contracts embeds versioned wire and persisted-record schemas.
package contracts

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"strings"
	"sync"
)

//go:embed schemas/*.json
var schemas embed.FS
var once sync.Once
var compiled map[string]*jsonschema.Schema
var compileErr error

type offline struct{}

func (offline) Load(string) (any, error) { return nil, errors.New("external schema loading disabled") }
func initialize() {
	compiled = make(map[string]*jsonschema.Schema)
	c := jsonschema.NewCompiler()
	c.UseLoader(offline{})
	c.AssertFormat()
	entries, err := schemas.ReadDir("schemas")
	if err != nil {
		compileErr = err
		return
	}
	for _, e := range entries {
		b, err := schemas.ReadFile("schemas/" + e.Name())
		if err != nil {
			compileErr = err
			return
		}
		var v any
		if err = json.Unmarshal(b, &v); err != nil {
			compileErr = err
			return
		}
		if err = c.AddResource("https://llmops.example/contracts/v1/"+e.Name(), v); err != nil {
			compileErr = err
			return
		}
	}
	for _, e := range entries {
		s, err := c.Compile("https://llmops.example/contracts/v1/" + e.Name())
		if err != nil {
			compileErr = err
			return
		}
		compiled[strings.TrimSuffix(e.Name(), ".schema.json")] = s
	}
}

// Validate checks decoded JSON. Authentication, ownership, byte limits, and
// cross-record consistency must additionally be checked by the service.
func Validate(name string, value any) error {
	once.Do(initialize)
	if compileErr != nil {
		return fmt.Errorf("compile contracts: %w", compileErr)
	}
	s, ok := compiled[name]
	if !ok {
		return errors.New("unknown contract")
	}
	return s.Validate(value)
}

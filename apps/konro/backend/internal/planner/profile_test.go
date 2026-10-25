package planner

import (
	"encoding/json"
	"testing"
)

func TestGenericTextContract(t *testing.T) {
	r := TextRequest("stable-key", Conditions{Count: 4, Servings: "4"}, nil, nil, nil)
	if r.Profile != "text" || r.IdempotencyKey != "stable-key" || !json.Valid(r.OutputSchema) || r.Input == nil {
		t.Fatal(r)
	}
	var schema map[string]any
	json.Unmarshal(r.OutputSchema, &schema)
	if schema["additionalProperties"] != false {
		t.Fatal("unbounded output")
	}
}

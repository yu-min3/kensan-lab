package evalcases

import (
	"strings"
	"testing"
)

func TestEmbeddedSetIsValid(t *testing.T) {
	s, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Select("dev")) != 15 || len(s.Select("holdout")) != 5 || len(s.Select("")) != 20 {
		t.Fatal("split sizes changed; update the dataset version and the docs together")
	}
	if len(Hash()) != 12 {
		t.Fatal("hash")
	}
}

func TestParseRejectsBrokenCases(t *testing.T) {
	good := `{"version":1,"note":"","cases":[{"id":"a","split":"dev","mode":"steps","tags":[],"text":"x","expect":{"title_contains":"x"}}]}`
	if _, err := Parse([]byte(good)); err != nil {
		t.Fatal(err)
	}
	for name, bad := range map[string]string{
		"unknown field":  strings.Replace(good, `"tags"`, `"oops":1,"tags"`, 1),
		"bad split":      strings.Replace(good, `"dev"`, `"train"`, 1),
		"empty forbid":   strings.Replace(good, `"title_contains":"x"`, `"title_contains":"x","forbid":[""]`, 1),
		"duplicate id":   strings.Replace(good, `]}`, `,{"id":"a","split":"dev","mode":"steps","tags":[],"text":"x","expect":{"title_contains":"x"}}]}`, 1),
		"inverted range": strings.Replace(good, `"title_contains":"x"`, `"title_contains":"x","steps_min":3,"steps_max":1`, 1),
	} {
		if _, err := Parse([]byte(bad)); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
}

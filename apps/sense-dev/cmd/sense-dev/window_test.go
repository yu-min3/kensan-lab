package main

import (
	"testing"
	"time"
)

func TestRunWindowInJST(t *testing.T) {
	w, err := parseRunWindow("01:30-04:00")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		utc  string
		want bool
	}{{"2026-09-26T16:29:00Z", false}, {"2026-09-26T16:30:00Z", true}, {"2026-09-26T18:59:00Z", true}, {"2026-09-26T19:00:00Z", false}} {
		now, err := time.Parse(time.RFC3339, test.utc)
		if err != nil {
			t.Fatal(err)
		}
		if got := w.contains(now); got != test.want {
			t.Errorf("%s: got %v, want %v", test.utc, got, test.want)
		}
	}
}

func TestOvernightWindowAndInvalidInputs(t *testing.T) {
	w, err := parseRunWindow("23:00-02:00")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		local string
		want  bool
	}{{"2026-09-27T22:59:00+09:00", false}, {"2026-09-27T23:00:00+09:00", true}, {"2026-09-28T01:59:00+09:00", true}, {"2026-09-28T02:00:00+09:00", false}} {
		now, err := time.Parse(time.RFC3339, test.local)
		if err != nil {
			t.Fatal(err)
		}
		if got := w.contains(now); got != test.want {
			t.Errorf("%s: got %v, want %v", test.local, got, test.want)
		}
	}
	for _, value := range []string{"", "00:00-00:00", "24:00-01:00", "10:00", "10:00-11:00-12:00"} {
		if _, err := parseRunWindow(value); err == nil {
			t.Errorf("accepted invalid inference window %q", value)
		}
	}
}

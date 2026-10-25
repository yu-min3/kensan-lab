package mealfeedback

import (
	"errors"
	"testing"
)

func TestReplayAndCorrections(t *testing.T) {
	s := New(t.TempDir())
	in := Event{ID: "event-1", Type: "swap", Reason: "量が多い", ProposalID: "proposal", File: "recipe"}
	e, err := s.Record(in)
	if err != nil {
		t.Fatal(err)
	}
	same, err := s.Record(in)
	if err != nil || same.Revision != e.Revision {
		t.Fatal("replay", err)
	}
	in.Revision = e.Revision
	in.Cancelled = true
	if _, err = s.Record(in); err != nil {
		t.Fatal(err)
	}
	in.Revision = 1
	in.Reason = "stale"
	if _, err = s.Record(in); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	record := CookRecord{ID: "session-1", Date: "2026-10-25", Recipes: []CookedRecipe{{File: "recipe", Version: "v1", Scale: "2", Made: true}}, Amount: "too_much"}
	r, err := s.Cook(record)
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.Cook(record)
	if err != nil || again.Revision != r.Revision {
		t.Fatal(err)
	}
	record.Revision = r.Revision
	record.Cancelled = true
	if _, err = s.Cook(record); err != nil {
		t.Fatal(err)
	}
	p, err := s.Preferences()
	if err != nil {
		t.Fatal(err)
	}
	p.Avoid = []string{"きのこ"}
	saved, err := s.SavePreferences(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.SavePreferences(p); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	restored, err := New(s.root).Preferences()
	if err != nil || restored.Revision != saved.Revision || restored.Avoid[0] != "きのこ" {
		t.Fatal(err)
	}
}

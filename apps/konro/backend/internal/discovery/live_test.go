package discovery

import (
	"context"
	"os"
	"testing"
)

func TestLiveSearch(t *testing.T) {
	if os.Getenv("KONRO_LIVE_TEST") != "1" {
		t.Skip("set KONRO_LIVE_TEST=1")
	}
	s := New(nil)
	for _, site := range Sites {
		t.Run(site.ID, func(t *testing.T) {
			r, err := s.Search(context.Background(), "肉じゃが", site.ID)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("results=%d warnings=%v", len(r.Results), r.Warnings)
			if len(r.Results) == 0 {
				t.Error("no live results")
			}
		})
	}
}

func TestLiveNoMatchDoesNotReturnRecommendations(t *testing.T) {
	if os.Getenv("KONRO_LIVE_TEST") != "1" {
		t.Skip("set KONRO_LIVE_TEST=1")
	}
	s := New(nil)
	for _, site := range []string{"delish", "kurashiru", "sirogohan", "nhk", "ajinomoto", "cookpad", "macaroni"} {
		t.Run(site, func(t *testing.T) {
			r, err := s.Search(context.Background(), "zzkonronomatch810732991", site)
			if err != nil {
				t.Fatal(err)
			}
			if len(r.Results) != 0 {
				t.Fatalf("recommendations mistaken for search results: %+v", r.Results)
			}
		})
	}
}

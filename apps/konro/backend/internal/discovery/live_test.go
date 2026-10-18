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
			images := 0
			for _, result := range r.Results {
				if result.Image != "" {
					images++
				}
			}
			t.Logf("results=%d images=%d warnings=%v", len(r.Results), images, r.Warnings)
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

func TestLiveCreatorMetadata(t *testing.T) {
	if os.Getenv("KONRO_LIVE_TEST") != "1" {
		t.Skip("opt in")
	}
	s := New(nil)
	r, err := s.Search(context.Background(), "なす", "all")
	if err != nil {
		t.Fatal(err)
	}
	creators := map[string]int{}
	for _, result := range r.Results {
		if result.Preferred && result.Info != nil && result.Info.Creator != nil {
			creators[result.Info.Creator.ID]++
		}
	}
	t.Logf("preferred=%v results=%d warnings=%v", creators, len(r.Results), r.Warnings)
	for _, c := range favoriteCreators {
		if creators[c.ID] == 0 {
			t.Errorf("no live favorite: %s", c.Name)
		}
	}
	info, err := s.Preview(context.Background(), "https://www.kurashiru.com/recipes/6938e7e7-dbcc-4da0-8760-f1286f19684e")
	if err != nil || info == nil || info.Creator == nil || info.Creator.ID != "kurashiru:chef:6" || info.Rating == nil {
		t.Fatalf("preview %+v err=%v", info, err)
	}
	t.Logf("creator=%+v rating=%+v publisher=%+v", info.Creator, info.Rating, info.Publisher)
}

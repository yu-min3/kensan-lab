package api

import (
	"context"
	"errors"
	"fmt"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/webpage"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

// Opt-in real API/filesystem process. Only external public HTML is substituted;
// the browser, resolver, preference store and ranking use production code.
func TestFavoritesHTTPProcess(t *testing.T) {
	addr := os.Getenv("KONRO_FAVORITES_TEST_ADDR")
	if addr == "" {
		t.Skip("opt-in HTTP process")
	}
	fetch := func(ctx context.Context, raw string) (webpage.Page, error) {
		u, _ := url.Parse(raw)
		body := ""
		switch {
		case u.Hostname() == "www.youtube.com" && u.Path == "/results":
			body = `<script>var ytInitialData={"contents":{"twoColumnSearchResultsRenderer":{"primaryContents":{"sectionListRenderer":{"contents":[{"itemSectionRenderer":{"contents":[`
			items := []string{}
			for _, c := range []struct{ id, name, video string }{{"UCabcdefghijklmnopqrstuv", "テストの料理人", "abcdefghijk"}, {"UCLcqrpFhRlJnm6etuLQblWg", "笠原将弘", "h67vdKlPGzg"}, {"UCzyxwvutsrqponmlkjihgfe", "もう一人の料理人", "a1b2c3d4e5f"}} {
				items = append(items, fmt.Sprintf(`{"videoRenderer":{"videoId":%q,"title":{"simpleText":%q},"longBylineText":{"runs":[{"text":%q,"navigationEndpoint":{"browseEndpoint":{"browseId":%q}}}]}}}`, c.video, "なすの料理・"+c.name, c.name, c.id))
			}
			body += strings.Join(items, ",") + `]}}]}}}}};</script>`
		case u.Hostname() == "www.youtube.com" && u.Path != "/results":
			id, name := "UCabcdefghijklmnopqrstuv", "テストの料理人"
			if u.Path == "/@slow" {
				select {
				case <-time.After(500 * time.Millisecond):
				case <-ctx.Done():
					return webpage.Page{}, ctx.Err()
				}
			}
			if u.Path == "/@missing" {
				return webpage.Page{}, errors.New("unavailable")
			}
			if strings.Contains(u.Path, "otherchef") || strings.Contains(u.Path, "UCzyxwvutsrqponmlkjihgfe") {
				id = "UCzyxwvutsrqponmlkjihgfe"
				name = "もう一人の料理人"
			}
			if strings.Contains(u.Path, "UCLcqrpFhRlJnm6etuLQblWg") {
				id = "UCLcqrpFhRlJnm6etuLQblWg"
				name = "笠原将弘"
			}
			if strings.Contains(u.Path, "UCP2gnyy_-ToZeIDw6qeI6HA") {
				id = "UCP2gnyy_-ToZeIDw6qeI6HA"
				name = "ジョージ"
			}
			body = fmt.Sprintf(`<script>var ytInitialData={"metadata":{"channelMetadataRenderer":{"externalId":%q,"title":%q}}};</script>`, id, name)
		case u.Hostname() == "www.kurashiru.com" && strings.HasPrefix(u.Path, "/chefs/"):
			body = `<h1 class="ChefsShow-name">中村和成</h1><ul class="ChefsShow-videosList"></ul>`
		default:
			body = `<html></html>`
		}
		return webpage.Page{URL: raw, HTML: body}, nil
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	handler := New(os.Getenv("KONRO_FAVORITES_TEST_DATA"), os.Getenv("KONRO_FAVORITES_TEST_STATIC"), WithSearchFetcher(fetch))
	if err = http.Serve(listener, handler); err != nil {
		t.Fatal(err)
	}
}

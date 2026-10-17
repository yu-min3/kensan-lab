// Package discovery owns public recipe search and source identity, never writes recipes.
package discovery

import (
	"net/url"
	"regexp"
	"strings"
)

type Site struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

var Sites = []Site{{"delish", "デリッシュキッチン"}, {"kurashiru", "クラシル"}, {"sirogohan", "白ごはん.com"}, {"youtube", "YouTube"}, {"nhk", "きょうの料理"}, {"ajinomoto", "味の素パーク"}, {"kikkoman", "キッコーマン"}, {"cookpad", "Cookpad"}, {"macaroni", "macaroni"}}
var videoID = regexp.MustCompile(`^[a-zA-Z0-9_-]{11}$`)
var recipePaths = map[string]*regexp.Regexp{
	"delish":    regexp.MustCompile(`^/recipes/[0-9]+/?$`),
	"kurashiru": regexp.MustCompile(`^/(?:recipes|recipe_cards)/[a-f0-9-]{36}/?$`),
	"sirogohan": regexp.MustCompile(`^/recipe/[a-zA-Z0-9_-]+/?$`),
	"nhk":       regexp.MustCompile(`^/recipe/[0-9]+_.+\.html$`),
	"ajinomoto": regexp.MustCompile(`^/recipe/card/[0-9]+/(?:print/)?$`),
	"kikkoman":  regexp.MustCompile(`^/homecook/search/recipe/[0-9]+/?$`),
	"cookpad":   regexp.MustCompile(`^/(?:jp/)?recipes/[0-9]+(?:-[^/]*)?/?$`),
	"macaroni":  regexp.MustCompile(`^/[0-9]+/?$`),
}

func SiteID(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	switch strings.ToLower(u.Hostname()) {
	case "delishkitchen.tv", "www.delishkitchen.tv":
		return "delish"
	case "kurashiru.com", "www.kurashiru.com":
		return "kurashiru"
	case "sirogohan.com", "www.sirogohan.com":
		return "sirogohan"
	case "youtube.com", "www.youtube.com", "m.youtube.com", "youtu.be":
		return "youtube"
	case "www.kyounoryouri.jp", "kyounoryouri.jp":
		return "nhk"
	case "park.ajinomoto.co.jp":
		return "ajinomoto"
	case "www.kikkoman.co.jp", "kikkoman.co.jp":
		return "kikkoman"
	case "cookpad.com", "www.cookpad.com":
		return "cookpad"
	case "macaro-ni.jp", "www.macaro-ni.jp":
		return "macaroni"
	}
	return ""
}
func Canonical(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "//") {
		raw = "https:" + raw
	} else if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.Port() != "" && u.Port() != "443" && u.Port() != "80" || (u.Scheme != "https" && u.Scheme != "http") {
		return "", false
	}
	site := SiteID(u.String())
	if site == "" {
		return "", false
	}
	if site == "youtube" {
		id := u.Query().Get("v")
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if u.Hostname() == "youtu.be" && len(parts) == 1 {
			id = parts[0]
		} else if len(parts) == 2 && (parts[0] == "shorts" || parts[0] == "embed" || parts[0] == "live") {
			id = parts[1]
		}
		if !videoID.MatchString(id) {
			return "", false
		}
		return "https://www.youtube.com/watch?v=" + id, true
	}
	if site == "sirogohan" && strings.Trim(u.Path, "/") == "recipe/index" {
		return "", false
	}
	if !recipePaths[site].MatchString(u.Path) || strings.Contains(u.Path, "..") {
		return "", false
	}
	u.Scheme = "https"
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), true
}
func Name(id string) string {
	for _, s := range Sites {
		if s.ID == id {
			return s.Name
		}
	}
	return ""
}

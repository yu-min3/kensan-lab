package discovery

import (
	"golang.org/x/net/html"
	"io"
	"strings"
)

// Stream links inside the site's result container. Avoid constructing a DOM for
// search pages, and never fall back to recommendations when results are empty.
func resultLinks(raw, site string, add func(string, string), done func() bool) {
	z := html.NewTokenizer(strings.NewReader(raw))
	z.SetMaxBuf(2 << 20)
	type frame struct {
		tag   string
		scope bool
	}
	stack := []frame{}
	scopeDepth := 0
	href, title, alt := "", "", ""
	anchor := false
	for !done() {
		kind := z.Next()
		if kind == html.ErrorToken {
			if z.Err() == io.EOF {
				return
			}
			return
		}
		switch kind {
		case html.StartTagToken, html.SelfClosingTagToken:
			t := z.Token()
			attrs := map[string]string{}
			for _, a := range t.Attr {
				attrs[a.Key] = a.Val
			}
			scoped := resultScope(site, attrs)
			void := strings.Contains(" area base br col embed hr img input link meta param source track wbr ", " "+t.Data+" ")
			if kind == html.StartTagToken && !void {
				if len(stack) >= 256 {
					return
				}
				stack = append(stack, frame{t.Data, scoped})
				if scoped {
					scopeDepth++
				}
			}
			if t.Data == "a" && scopeDepth > 0 {
				href = attrs["href"]
				title = ""
				alt = ""
				anchor = true
			}
			if t.Data == "img" && anchor && attrs["alt"] != "" {
				alt = attrs["alt"]
			}
		case html.TextToken:
			if anchor && len(title) < 2000 {
				title += string(z.Text()) + " "
			}
		case html.EndTagToken:
			t := z.Token()
			if t.Data == "a" && anchor {
				if alt != "" {
					title = alt
				}
				add(href, title)
				anchor = false
			}
			for i := len(stack) - 1; i >= 0; i-- {
				if stack[i].tag == t.Data {
					for _, f := range stack[i:] {
						if f.scope {
							scopeDepth--
						}
					}
					stack = stack[:i]
					break
				}
			}
		}
	}
}
func resultScope(site string, a map[string]string) bool {
	class := func(want string) bool {
		for _, v := range strings.Fields(a["class"]) {
			if v == want {
				return true
			}
		}
		return false
	}
	switch site {
	case "delish":
		return class("delish-search")
	case "kurashiru":
		return class("SearchIndex-DlyMasonry")
	case "sirogohan":
		return a["id"] == "recipe" && class("recipe_list")
	case "nhk":
		return class("recipe--sort-row")
	case "ajinomoto":
		return class("search-results-area")
	case "cookpad":
		return strings.HasPrefix(a["id"], "recipe_")
	case "macaroni":
		return class("search")
	}
	return false
}

package importer

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/recipe"
)

const MaxSource = 5 << 20
const MaxText = 128 << 10

type Source struct {
	URL  string `json:"url,omitempty"`
	Raw  string `json:"raw"`
	Text string `json:"text"`
}
type Content = recipe.Content

// PublicIP excludes local/private/link-local and transition ranges which can
// otherwise reach metadata endpoints or tunnel an embedded private IPv4.
func PublicIP(ip net.IP) bool {
	a, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	a = a.Unmap()
	if !a.IsGlobalUnicast() || a.IsPrivate() || a.IsLoopback() || a.IsLinkLocalUnicast() {
		return false
	}
	for _, s := range []string{"100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "2001::/23", "2002::/16", "64:ff9b::/96", "64:ff9b:1::/48"} {
		if netip.MustParsePrefix(s).Contains(a) {
			return false
		}
	}
	return true
}
func validURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "//") {
		raw = "https:" + raw
	} else if raw != "" && !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, e := url.Parse(raw)
	if e != nil || u.User != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, errors.New("http/httpsのURLを入力してください")
	}
	if port := u.Port(); port != "" && port != "80" && port != "443" {
		return nil, errors.New("標準ポートのURLのみ取り込めます")
	}
	u.Fragment = ""
	return u, nil
}
func Fetch(ctx context.Context, raw string) (Source, error) {
	u, e := validURL(raw)
	if e != nil {
		return Source{}, e
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	transport := &http.Transport{Proxy: nil, TLSHandshakeTimeout: 5 * time.Second, ResponseHeaderTimeout: 8 * time.Second, MaxResponseHeaderBytes: 32768, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, e := net.SplitHostPort(address)
		if e != nil {
			return nil, e
		}
		ips, e := net.DefaultResolver.LookupIPAddr(ctx, host)
		if e != nil || len(ips) == 0 {
			return nil, errors.New("名前解決できません")
		}
		for _, ip := range ips {
			if !PublicIP(ip.IP) {
				return nil, errors.New("ローカルネットワークのURLは取り込めません")
			}
		}
		var last error
		for _, ip := range ips {
			c, e := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", net.JoinHostPort(ip.IP.String(), port))
			if e == nil {
				return c, nil
			}
			last = e
		}
		return nil, last
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if len(via) >= 4 {
			return errors.New("リダイレクトが多すぎます")
		}
		_, e := validURL(r.URL.String())
		return e
	}}
	req, e := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if e != nil {
		return Source{}, e
	}
	req.Header.Set("User-Agent", "Konro/1.0 Recipe Import")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	res, e := client.Do(req)
	if e != nil {
		return Source{}, errors.New("サイトを取得できません。本文を貼り付けて取り込めます")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return Source{}, errors.New("サイトが取得を許可していません。本文を貼り付けてください")
	}
	ct := strings.ToLower(res.Header.Get("Content-Type"))
	if !strings.Contains(ct, "text/html") && !strings.Contains(ct, "application/xhtml+xml") {
		return Source{}, errors.New("HTMLページのみ取り込めます")
	}
	b, e := io.ReadAll(io.LimitReader(res.Body, MaxSource+1))
	if e != nil || len(b) > MaxSource {
		return Source{}, errors.New("ページが大きすぎます（最大5MiB）")
	}
	s := Source{URL: res.Request.URL.String(), Raw: string(b)}
	doc, e := goquery.NewDocumentFromReader(strings.NewReader(s.Raw))
	if e != nil {
		return Source{}, e
	}
	doc.Find("script,style,nav,header,footer,iframe,noscript").Remove()
	selection := doc.Find("main,article").First()
	if selection.Length() == 0 {
		selection = doc.Find("body")
	}
	s.Text = clean(selection.Text())
	if len(s.Text) > MaxText {
		s.Text = string([]rune(s.Text)[:min(len([]rune(s.Text)), MaxText/4)])
	}
	return s, nil
}
func clean(s string) string {
	doc, e := goquery.NewDocumentFromReader(strings.NewReader("<body>" + s + "</body>"))
	if e != nil {
		return strings.Join(strings.Fields(s), " ")
	}
	return strings.Join(strings.Fields(doc.Find("body").Text()), " ")
}
func stringsOf(v any) []string {
	var out []string
	switch x := v.(type) {
	case string:
		if s := clean(x); s != "" {
			out = append(out, s)
		}
	case []any:
		for _, v := range x {
			out = append(out, stringsOf(v)...)
		}
	case map[string]any:
		if sub, ok := x["itemListElement"]; ok {
			out = append(out, stringsOf(sub)...)
		} else if sub, ok := x["text"]; ok {
			out = append(out, stringsOf(sub)...)
		}
	}
	return out
}
func Extract(s Source) (Content, []string) {
	c := Content{Ingredients: []string{}, Steps: []string{}}
	warnings := []string{}
	if s.URL == "" {
		return c, []string{"本文をAIで抽出するか、材料と手順を入力してください"}
	}
	doc, e := goquery.NewDocumentFromReader(strings.NewReader(s.Raw))
	if e != nil {
		return c, []string{"ページを解析できません"}
	}
	var recipes []map[string]any
	var walk func(any, int)
	walk = func(v any, depth int) {
		if depth > 32 {
			return
		}
		switch x := v.(type) {
		case []any:
			for _, v := range x {
				walk(v, depth+1)
			}
		case map[string]any:
			isRecipe := false
			for _, t := range stringsOf(x["@type"]) {
				if t == "Recipe" || t == "https://schema.org/Recipe" || t == "http://schema.org/Recipe" {
					isRecipe = true
				}
			}
			if isRecipe {
				recipes = append(recipes, x)
				return
			}
			for _, v := range x {
				walk(v, depth+1)
			}
		}
	}
	doc.Find(`script[type="application/ld+json"]`).Each(func(_ int, el *goquery.Selection) {
		var v any
		if json.Unmarshal([]byte(el.Text()), &v) == nil {
			walk(v, 0)
		}
	})
	if len(recipes) == 0 {
		c.Title = clean(doc.Find("h1").First().Text())
		return c, []string{"構造化レシピがありません。AIで抽出するか、本文から入力してください"}
	}
	if len(recipes) > 1 {
		warnings = append(warnings, "複数のレシピが見つかりました。最初のレシピを表示しています。原文を確認してください")
	}
	r := recipes[0]
	if v, ok := r["name"].(string); ok {
		c.Title = clean(v)
	}
	c.Servings = strings.Join(stringsOf(r["recipeYield"]), " / ")
	if n, ok := r["recipeYield"].(float64); ok {
		b, _ := json.Marshal(n)
		c.Servings = string(b)
	}
	c.Ingredients = stringsOf(r["recipeIngredient"])
	c.Steps = stringsOf(r["recipeInstructions"])
	return c, warnings
}

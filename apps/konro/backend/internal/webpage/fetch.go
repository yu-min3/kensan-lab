// Package webpage owns bounded public HTTP access, shared by search and import.
package webpage

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

const MaxSource = 5 << 20

type Page struct {
	URL  string
	HTML string
}

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
func NormalizeURL(raw string) (*url.URL, error) {
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
func Fetch(ctx context.Context, raw string) (Page, error) { return fetch(ctx, raw, false, MaxSource) }

// FetchSearch shares public IP, redirect and size checks; only discovery uses XML feeds.
func FetchSearch(ctx context.Context, raw string, xml bool) (Page, error) {
	return fetch(ctx, raw, xml, 2<<20)
}
func fetch(ctx context.Context, raw string, allowXML bool, maxBytes int64) (Page, error) {
	u, e := NormalizeURL(raw)
	if e != nil {
		return Page{}, e
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
		_, e := NormalizeURL(r.URL.String())
		return e
	}}
	req, e := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if e != nil {
		return Page{}, e
	}
	req.Header.Set("User-Agent", "Konro/1.0 Recipe Import")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	res, e := client.Do(req)
	if e != nil {
		return Page{}, errors.New("サイトを取得できません。本文を貼り付けて取り込めます")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return Page{}, errors.New("サイトが取得を許可していません。本文を貼り付けてください")
	}
	ct := strings.ToLower(res.Header.Get("Content-Type"))
	if !strings.Contains(ct, "text/html") && !strings.Contains(ct, "application/xhtml+xml") && !(allowXML && (strings.Contains(ct, "text/xml") || strings.Contains(ct, "application/xml") || strings.Contains(ct, "application/rss+xml"))) {
		return Page{}, errors.New("HTMLページのみ取り込めます")
	}
	b, e := io.ReadAll(io.LimitReader(res.Body, maxBytes+1))
	if e != nil || int64(len(b)) > maxBytes {
		return Page{}, errors.New("ページが大きすぎます。URLか本文からの取り込みを試してください")
	}
	return Page{URL: res.Request.URL.String(), HTML: string(b)}, nil
}

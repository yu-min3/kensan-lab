// Package packageauth confines the owner's classic read:packages credential to
// host package observation. Repository mutation keeps its separate token.
package packageauth

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"syscall"
	"time"
)

// OwnerToken verifies the account without returning or logging its response.
// API and Client overrides are for loopback-only tests.
func OwnerToken(ctx context.Context, repoFile, packageFile string, client *http.Client, api string) (string, error) {
	info, err := os.Lstat(packageFile)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		return "", errors.New("package token requires an owner-only mode0600 regular file")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Geteuid()) {
		return "", errors.New("package token file owner differs from host process")
	}
	repoInfo, err := os.Stat(repoFile)
	if err != nil || os.SameFile(info, repoInfo) {
		return "", errors.New("separate repository and package token files required")
	}
	data, err := os.ReadFile(packageFile)
	if err != nil {
		return "", errors.New("package token unavailable")
	}
	token := strings.TrimSpace(string(data))
	repoData, err := os.ReadFile(repoFile)
	if err != nil || token == "" || strings.ContainsAny(token, "\r\n") || token == strings.TrimSpace(string(repoData)) {
		return "", errors.New("separate package credential required")
	}
	endpoint := "https://api.github.com"
	if api != "" && api != endpoint {
		u, err := url.Parse(api)
		if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.Port() == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
			return "", errors.New("package account API outside trusted endpoint")
		}
		endpoint = api
	}
	c := &http.Client{Timeout: 20 * time.Second, Transport: &http.Transport{Proxy: nil}}
	if client != nil {
		*c = *client
	}
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return errors.New("package account redirect rejected") }
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"/user", nil)
	if err != nil {
		return "", errors.New("package account request invalid")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	resp, err := c.Do(req)
	if err != nil {
		return "", errors.New("package account verification unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || strings.TrimSpace(resp.Header.Get("X-OAuth-Scopes")) != "read:packages" {
		return "", errors.New("owner classic read:packages-only credential required")
	}
	var owner struct{ Login string }
	if json.NewDecoder(io.LimitReader(resp.Body, 16<<10)).Decode(&owner) != nil || owner.Login != "yu-min3" {
		return "", errors.New("package credential owner differs from approved account")
	}
	return token, nil
}

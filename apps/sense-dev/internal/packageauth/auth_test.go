package packageauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestOwnerPackageCredentialRequiresSeparatePrivateReadonlyOwnerToken(t *testing.T) {
	for _, tc := range []struct {
		name, login, scope           string
		mode                         os.FileMode
		sameFile, sameToken, symlink bool
		want                         bool
	}{
		{name: "valid", login: "yu-min3", scope: "read:packages", mode: 0600, want: true},
		{name: "missing scope", login: "yu-min3", mode: 0600},
		{name: "another owner", login: "other", scope: "read:packages", mode: 0600},
		{name: "repository scope", login: "yu-min3", scope: "read:packages, repo", mode: 0600},
		{name: "write scope", login: "yu-min3", scope: "read:packages, write:packages", mode: 0600},
		{name: "world readable", login: "yu-min3", scope: "read:packages", mode: 0644},
		{name: "same file", login: "yu-min3", scope: "read:packages", mode: 0600, sameFile: true},
		{name: "same credential", login: "yu-min3", scope: "read:packages", mode: 0600, sameToken: true},
		{name: "symlink", login: "yu-min3", scope: "read:packages", mode: 0600, symlink: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			repo, pkg := filepath.Join(dir, "repo"), filepath.Join(dir, "package")
			os.WriteFile(repo, []byte("repo-only"), 0600)
			value := "package-read"
			if tc.sameToken {
				value = "repo-only"
			}
			os.WriteFile(pkg, []byte(value), tc.mode)
			if tc.sameFile {
				pkg = repo
			}
			if tc.symlink {
				link := filepath.Join(dir, "link")
				os.Symlink(pkg, link)
				pkg = link
			}
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "GET" || r.URL.Path != "/user" || r.Header.Get("Authorization") != "Bearer package-read" {
					t.Error("package auth escaped owner GET")
				}
				w.Header().Set("X-OAuth-Scopes", tc.scope)
				json.NewEncoder(w).Encode(map[string]string{"login": tc.login})
			}))
			defer server.Close()
			token, err := OwnerToken(context.Background(), repo, pkg, server.Client(), server.URL)
			if (err == nil) != tc.want {
				t.Fatalf("accepted=%v want=%v", err == nil, tc.want)
			}
			if tc.want && token != "package-read" {
				t.Fatal("incorrect credential")
			}
			if tc.mode != 0600 || tc.sameFile || tc.sameToken || tc.symlink {
				if calls != 0 {
					t.Fatal("invalid local token reached API")
				}
			}
		})
	}
}

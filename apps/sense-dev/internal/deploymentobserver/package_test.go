package deploymentobserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestObserverPackagePrivacyUsesSeparateOwnerCredentialAndAuthenticatedPath(t *testing.T) {
	dir := t.TempDir()
	repo, pkg := filepath.Join(dir, "repo"), filepath.Join(dir, "package")
	os.WriteFile(repo, []byte("repository-read"), 0600)
	os.WriteFile(pkg, []byte("package-read"), 0600)
	paths := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.EscapedPath())
		if r.Method != "GET" || r.Header.Get("Authorization") != "Bearer package-read" {
			t.Error("observer package credential differs")
		}
		switch r.URL.EscapedPath() {
		case "/user":
			w.Header().Set("X-OAuth-Scopes", "read:packages")
			json.NewEncoder(w).Encode(map[string]string{"login": "yu-min3"})
		case "/user/packages/container/kensan-lab%2Fcanary":
			json.NewEncoder(w).Encode(map[string]any{"visibility": "private", "repository": map[string]string{"full_name": repository}})
		default:
			t.Error("package API path escaped fixed scope")
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	g := GitHubRelease{TokenFile: repo, PackageTokenFile: pkg, Client: server.Client(), API: server.URL}
	token, err := g.packageToken(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	private, err := g.packagePrivate(context.Background(), token)
	if err != nil || !private {
		t.Fatalf("privacy proof failed: %v", err)
	}
	before := len(paths)
	g.PackageTokenFile = ""
	if _, err := g.packageToken(context.Background()); err == nil {
		t.Fatal("missing package credential fell back to repository token")
	}
	if len(paths) != before {
		t.Fatal("fallback reached API")
	}
}

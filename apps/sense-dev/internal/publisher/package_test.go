package publisher

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestPublisherPackageAndRepositoryCredentialsCannotCrossRoutes(t *testing.T) {
	f := newImageFixture(t)
	ctx := context.Background()
	if _, err := f.g.packageRequest(ctx, "/repos/"+repository+"/pulls", nil); err == nil {
		t.Fatal("package token used for repo API")
	}
	if _, err := f.g.request(ctx, "GET", "/user/packages/container/kensan-lab%2Fcanary", nil, nil); err == nil {
		t.Fatal("repo token used for package API")
	}
	f.g.PackageTokenFile = ""
	if _, err := f.g.Execute(ctx, f.intent); err == nil || f.posts != 0 {
		t.Fatal("missing package token dispatched workflow")
	}
	for _, mutation := range []string{"package-owner", "package-scope"} {
		f := newImageFixture(t)
		f.mutation = mutation
		if _, err := f.g.Execute(ctx, f.intent); err == nil || f.posts != 0 {
			t.Fatal("wrong account credential dispatched workflow")
		}
	}
}

func fixturePackageToken(t *testing.T) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "owner-package-token")
	if err := os.WriteFile(file, []byte("package-token"), 0600); err != nil {
		t.Fatal(err)
	}
	return file
}

package catalog

import (
	"github.com/yu-min3/kensan-lab/apps/llmops/internal/worker"
	"testing"
)

func TestCatalogIdentityAndInjectionBoundary(t *testing.T) {
	c := worker.CatalogConfig{Name: "konro", Namespace: "curated", Table: "recipes", URI: "http://127.0.0.1:19381/api/catalog", TokenURL: "https://id.localhost:19443/realms/llmops/protocol/openid-connect/token", ClientID: "catalog-konro-transform", ClientSecretFile: "/private/catalog-konro-transform.secret", CAFile: "/private/ca.pem", PythonBinary: "/private/venv/bin/python"}
	if e := Validate(&c, "konro", "transform"); e != nil {
		t.Fatal(e)
	}
	for _, mutate := range []func(*worker.CatalogConfig){func(c *worker.CatalogConfig) { c.Name = "sample" }, func(c *worker.CatalogConfig) { c.Table = "recipes; DROP TABLE x" }, func(c *worker.CatalogConfig) { c.URI = "http://external.example/api/catalog" }, func(c *worker.CatalogConfig) { c.ClientID = "catalog-konro-eval" }, func(c *worker.CatalogConfig) { c.TokenURL = "http://id.localhost/token" }} {
		bad := c
		mutate(&bad)
		if Validate(&bad, "konro", "transform") == nil {
			t.Fatal("unsafe config accepted")
		}
	}
}

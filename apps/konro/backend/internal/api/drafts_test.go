package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/agentclient"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/importer"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/recipe"
)

func TestDraftHTTPContractAndZipIsolation(t *testing.T) {
	var calls atomic.Int32
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer service-test-token" {
			t.Error("missing service credential")
		}
		if r.Method == "POST" {
			var in map[string]any
			if json.NewDecoder(r.Body).Decode(&in) != nil {
				t.Error("bad request")
			}
			if in["profile"] != "text" || in["output_schema"] == nil || in["input"] == nil || in["idempotency_key"] == nil || len(in) != 5 {
				t.Errorf("unexpected job contract: %v", in)
			}
			calls.Add(1)
			w.WriteHeader(202)
			_, _ = w.Write([]byte(`{"id":"remote-job","state":"queued"}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"remote-job","state":"succeeded","output":{"title":"魚","servings":"2人分","ingredients":["魚 2切れ"],"steps":["10分煮る"]}}`))
	}))
	defer remote.Close()
	gateway, e := agentclient.New(remote.URL, func(context.Context) (string, error) { return "service-test-token", nil })
	if e != nil {
		t.Fatal(e)
	}
	dir := t.TempDir()
	app := httptest.NewServer(New(dir, "", WithAgent(gateway)))
	defer app.Close()
	request := func(method, path string, body any) (int, []byte) {
		t.Helper()
		b, _ := json.Marshal(body)
		r, _ := http.NewRequest(method, app.URL+path, bytes.NewReader(b))
		r.Header.Set("Content-Type", "application/json")
		res, e := http.DefaultClient.Do(r)
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		var out bytes.Buffer
		out.ReadFrom(res.Body)
		return res.StatusCode, out.Bytes()
	}
	read := func(status int, b []byte) *importer.Draft {
		t.Helper()
		if status != 200 {
			t.Fatalf("%d %s", status, b)
		}
		var d importer.Draft
		if e := json.Unmarshal(b, &d); e != nil {
			t.Fatal(e)
		}
		return &d
	}
	d := read(request("POST", "/api/v1/import/drafts", map[string]string{"text": "魚 2人分 魚2切れ 10分煮る"}))
	base := "/api/v1/import/drafts/" + d.ID
	if d.Job == nil || calls.Load() != 1 {
		t.Fatal("automatic fallback not started", d)
	}
	d = read(request("GET", base, nil))
	if d.Job.State != "succeeded" || d.Content.Title != "魚" || calls.Load() != 1 {
		t.Fatalf("%+v calls %d", d, calls.Load())
	}
	d = read(request("POST", base+"/save", map[string]int{"revision": d.Revision}))
	status, b := request("GET", "/api/v1/recipes/"+url.PathEscape(d.SavedFile), nil)
	if status != 200 {
		t.Fatalf("encoded imported path: %d %s", status, b)
	}
	var rec recipe.Recipe
	_ = json.Unmarshal(b, &rec)
	if rec.File != d.SavedFile || rec.Steps[0] != "10分煮る" {
		t.Fatal(rec)
	}
	path := filepath.Join(dir, filepath.FromSlash(d.SavedFile)+".md")
	before, _ := os.ReadFile(path)
	res, e := http.Post(app.URL+"/api/v1/import", "application/zip", bytes.NewReader(exportZip(t)))
	if e != nil {
		t.Fatal(e)
	}
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatal(res.StatusCode)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("ZIP replaced URL recipe")
	}
	status, b = request("GET", "/api/v1/recipes", nil)
	var metas []recipe.Meta
	_ = json.Unmarshal(b, &metas)
	if status != 200 || len(metas) != 3 {
		t.Fatalf("%d %s", status, b)
	}
	status, _ = request("PATCH", base, map[string]any{"revision": 1, "content": d.Content})
	if status != 409 {
		t.Fatal("stale write accepted")
	}
	for _, bad := range []string{"file:///etc/passwd", "http://127.0.0.1", "http://169.254.169.254"} {
		status, _ = request("POST", "/api/v1/import/drafts", map[string]string{"url": bad})
		if status != 400 {
			t.Fatal("unsafe URL accepted", bad)
		}
	}
	r, _ := http.NewRequest("POST", app.URL+"/api/v1/import/drafts", strings.NewReader(`{"text":"a"}`))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", "https://attacker.example")
	res, e = http.DefaultClient.Do(r)
	if e != nil {
		t.Fatal(e)
	}
	res.Body.Close()
	if res.StatusCode != 403 {
		t.Fatal("cross-origin write accepted")
	}
}

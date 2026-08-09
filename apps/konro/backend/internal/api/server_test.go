package api

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func exportZip(t *testing.T) []byte {
	t.Helper()
	htmlSrc, err := os.ReadFile("../rkimport/testdata/recipes.html")
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("recipekeeperhtml/recipes.html")
	w.Write(htmlSrc)
	w, _ = zw.Create("recipekeeperhtml/images/nanban.jpg")
	w.Write([]byte("fake-jpeg"))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestImportEndpoint(t *testing.T) {
	dataDir := t.TempDir()
	srv := httptest.NewServer(New(dataDir, ""))
	defer srv.Close()

	post := func() (int, importReport) {
		t.Helper()
		resp, err := http.Post(srv.URL+"/api/v1/import", "application/zip", bytes.NewReader(exportZip(t)))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var rep importReport
		if resp.StatusCode == http.StatusOK {
			if err := json.NewDecoder(resp.Body).Decode(&rep); err != nil {
				t.Fatal(err)
			}
		}
		return resp.StatusCode, rep
	}

	status, rep := post()
	if status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	if rep.Recipes != 2 || rep.Images != 1 {
		t.Fatalf("report = %+v", rep)
	}
	if len(rep.UnknownProps) != 1 { // testdata plants recipeFutureUnknownField
		t.Errorf("unknownProps = %#v", rep.UnknownProps)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "鶏むね肉の南蛮漬け.md")); err != nil {
		t.Error(err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "images", "nanban.jpg")); err != nil {
		t.Error(err)
	}

	// the list API must see the imported recipes without any restart
	resp, err := http.Get(srv.URL + "/api/v1/recipes")
	if err != nil {
		t.Fatal(err)
	}
	var metas []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&metas); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if len(metas) != 2 {
		t.Fatalf("recipes after import = %d, want 2", len(metas))
	}

	// re-import = upsert by filename: same count, no -2 duplicates
	if status, _ := post(); status != http.StatusOK {
		t.Fatalf("re-import status = %d", status)
	}
	files, err := filepath.Glob(filepath.Join(dataDir, "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("md files after re-import = %d, want 2 (%v)", len(files), files)
	}
}

func TestImportEndpointRejectsGarbage(t *testing.T) {
	srv := httptest.NewServer(New(t.TempDir(), ""))
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/api/v1/import", "application/zip", bytes.NewReader([]byte("not a zip")))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

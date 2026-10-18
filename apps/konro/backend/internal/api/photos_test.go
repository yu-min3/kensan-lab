package api

import (
	"bytes"
	"encoding/json"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/recipe"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPhotoUpload(t *testing.T) {
	dir := t.TempDir()
	original := "---\ntitle: 魚\nimages: [images/old.jpg, images/other.jpg]\nrk_id: keep-id\ncustom: keep-value\nsource: 本\nrating: 4\n---\n\n## 材料\n- 魚\n\n## 手順\n1. 焼く\n\n## 独自の補足\nこのまま保持\n"
	if err := os.WriteFile(filepath.Join(dir, "fish.md"), []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	handler := New(dir, "")
	before, err := recipe.Load(dir, "fish")
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 48, 32))
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})
	var photo bytes.Buffer
	if err = png.Encode(&photo, img); err != nil {
		t.Fatal(err)
	}
	put := func(name, version string, body []byte) *httptest.ResponseRecorder {
		req := httptest.NewRequest("PUT", "/api/v1/recipe-photos/"+name, bytes.NewReader(body))
		req.Header.Set("If-Match", version)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w
	}
	w := put("fish", before.Version, photo.Bytes())
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	var after recipe.Recipe
	if err = json.Unmarshal(w.Body.Bytes(), &after); err != nil {
		t.Fatal(err)
	}
	if after.Images[0] == before.Images[0] || after.Images[1] != before.Images[1] || after.Version == before.Version || after.RKID != before.RKID || after.Rating != 4 || after.Source != "本" || after.Ingredients[0] != "魚" || after.Steps[0] != "焼く" {
		t.Fatalf("bad update: %+v", after)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "fish.md"))
	if !strings.Contains(string(raw), "custom: keep-value") || !strings.HasSuffix(string(raw), strings.SplitN(original, "---\n", 3)[2]) {
		t.Fatalf("metadata/body changed: %s", raw)
	}
	iw := httptest.NewRecorder()
	handler.ServeHTTP(iw, httptest.NewRequest("GET", "/"+after.Images[0], nil))
	if iw.Code != 200 {
		t.Fatal(iw.Code)
	}
	if _, format, e := image.Decode(bytes.NewReader(iw.Body.Bytes())); e != nil || format != "jpeg" {
		t.Fatalf("saved image %s %v", format, e)
	}
	for _, tc := range []struct {
		name, version string
		body          []byte
		status        int
	}{{"fish", before.Version, photo.Bytes(), 409}, {"fish", "", photo.Bytes(), 409}, {"missing", after.Version, photo.Bytes(), 404}, {"fish", after.Version, []byte("<svg/>"), 400}, {"fish", after.Version, photo.Bytes()[:30], 400}, {"fish", after.Version, make([]byte, recipe.MaxPhotoBytes+1), 413}, {"%2e%2e%2fescape", after.Version, photo.Bytes(), 400}} {
		w := put(tc.name, tc.version, tc.body)
		if w.Code != tc.status {
			t.Errorf("%s: want %d got %d %s", tc.name, tc.status, w.Code, w.Body)
		}
	}
	unchanged, _ := os.ReadFile(filepath.Join(dir, "fish.md"))
	if !bytes.Equal(raw, unchanged) {
		t.Fatal("rejected request changed recipe")
	}
	img.Set(1, 1, color.RGBA{0, 255, 0, 255})
	photo.Reset()
	_ = png.Encode(&photo, img)
	w = put("fish", after.Version, photo.Bytes())
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var replaced recipe.Recipe
	_ = json.Unmarshal(w.Body.Bytes(), &replaced)
	if replaced.Images[0] == after.Images[0] {
		t.Fatal("replacement reused URL")
	}
	if _, err = os.Stat(filepath.Join(dir, after.Images[0])); err != nil {
		t.Fatal("removed prior image", err)
	}
}
func TestPhotoRejectsDimensionsAndSymlink(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "fish.md"), []byte("---\ntitle: 魚\n---\n"), 0600)
	r, _ := recipe.Load(dir, "fish")
	var photo bytes.Buffer
	_ = png.Encode(&photo, image.NewGray(image.Rect(0, 0, 4097, 1)))
	if _, e := recipe.UpdatePhoto(dir, "fish", r.Version, photo.Bytes()); e != recipe.ErrPhoto {
		t.Fatal(e)
	}
	photo.Reset()
	_ = png.Encode(&photo, image.NewGray(image.Rect(0, 0, 2, 2)))
	if err := os.Symlink(outside, filepath.Join(dir, "images")); err != nil {
		t.Fatal(err)
	}
	if _, e := recipe.UpdatePhoto(dir, "fish", r.Version, photo.Bytes()); e != recipe.ErrInvalid {
		t.Fatal(e)
	}
	entries, _ := os.ReadDir(outside)
	if len(entries) != 0 {
		t.Fatal("wrote through symlink")
	}
}

// A request blocked on its body must occupy the upload slot. The rejected
// request's body must not be read at all, so it cannot allocate another 8MB.
type heldPhotoBody struct {
	started chan struct{}
	release chan struct{}
}

func (b *heldPhotoBody) Read(p []byte) (int, error) { close(b.started); <-b.release; return 0, io.EOF }

type unexpectedPhotoBody struct{ t *testing.T }

func (b unexpectedPhotoBody) Read(p []byte) (int, error) {
	b.t.Error("busy upload body was read")
	return 0, io.EOF
}
func TestPhotoUploadAdmissionBeforeBody(t *testing.T) {
	handler := New(t.TempDir(), "")
	held := &heldPhotoBody{make(chan struct{}), make(chan struct{})}
	done := make(chan struct{})
	go func() {
		defer close(done)
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("PUT", "/api/v1/recipe-photos/fish", held))
	}()
	<-held.started
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("PUT", "/api/v1/recipe-photos/fish", unexpectedPhotoBody{t}))
	if w.Code != 429 || w.Header().Get("Retry-After") != "1" {
		t.Errorf("busy response: %d %v", w.Code, w.Header())
	}
	close(held.release)
	<-done
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("PUT", "/api/v1/recipe-photos/fish", nil))
	if w.Code != 400 {
		t.Fatalf("slot not released: %d", w.Code)
	}
}

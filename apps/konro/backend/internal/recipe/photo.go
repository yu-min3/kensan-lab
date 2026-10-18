package recipe

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"time"
)

const MaxPhotoBytes = 8 << 20

var ErrPhoto = fmt.Errorf("JPEG・PNGの写真を選んでください（8MB以下、最大2048×2048ピクセル）")

// UpdatePhoto changes only the first image and edit metadata. Unique image names
// leave existing images intact, including references from other recipes.
func UpdatePhoto(dir, name, expected string, input []byte) (*Recipe, error) {
	if len(input) == 0 || len(input) > MaxPhotoBytes {
		return nil, ErrPhoto
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(input))
	if err != nil || (format != "jpeg" && format != "png") || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 2048 || cfg.Height > 2048 {
		return nil, ErrPhoto
	}
	writes.Lock()
	defer writes.Unlock()
	old, err := readSource(dir, name)
	if err != nil {
		return nil, err
	}
	if expected == "" || expected != version(old) {
		return nil, ErrConflict
	}
	img, _, err := image.Decode(bytes.NewReader(input))
	if err != nil {
		return nil, ErrPhoto
	}
	// Re-encode actual pixels: strip metadata and never serve uploaded active content.
	flat := image.NewRGBA(img.Bounds())
	draw.Draw(flat, flat.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(flat, flat.Bounds(), img, img.Bounds().Min, draw.Over)
	var encoded bytes.Buffer
	if err = jpeg.Encode(&encoded, flat, &jpeg.Options{Quality: 90}); err != nil {
		return nil, err
	}
	if encoded.Len() > MaxPhotoBytes {
		return nil, ErrPhoto
	}
	doc, err := DecodeDocument(old)
	if err != nil {
		return nil, err
	}
	images := append([]string(nil), doc.Recipe.Images...)
	digest := sha256.Sum256(encoded.Bytes())
	ref := fmt.Sprintf("images/uploads/%x.jpg", digest)
	if len(images) == 0 {
		images = []string{ref}
	} else {
		images[0] = ref
	}
	if err = doc.set("images", images, true); err != nil {
		return nil, err
	}
	if err = doc.set("updated", time.Now().Format("2006-01-02"), false); err != nil {
		return nil, err
	}
	if err = doc.set("konro_edited", true, false); err != nil {
		return nil, err
	}
	data, err := doc.encode(doc.body)
	if err != nil {
		return nil, err
	}
	// Check each directory so existing symlinks cannot redirect an upload outside dataDir.
	for _, part := range []string{"images", "images/uploads"} {
		p := filepath.Join(dir, part)
		if err = os.Mkdir(p, 0755); err != nil && !os.IsExist(err) {
			return nil, err
		}
		info, e := os.Lstat(p)
		if e != nil {
			return nil, e
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return nil, ErrInvalid
		}
	}
	target := filepath.Join(dir, ref)
	// Identical content is safe to replace after an interrupted commit. Atomic
	// rename also replaces a symlink rather than following its target.
	if err = atomicReplace(target, encoded.Bytes()); err != nil {
		return nil, err
	}
	if err = atomicReplace(filepath.Join(dir, name+".md"), data); err != nil {
		return nil, err
	}
	return Load(dir, name)
}

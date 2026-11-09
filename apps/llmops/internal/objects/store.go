// Package objects stores original bytes under a trusted application boundary.
package objects

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrNotFound    = errors.New("object not found")
	ErrExists      = errors.New("object already exists")
	ErrConflict    = errors.New("capture key conflicts with existing content")
	ErrInvalid     = errors.New("invalid capture")
	ErrTooLarge    = errors.New("capture too large")
	ErrUnavailable = errors.New("object storage unavailable")
	ErrCorrupt     = errors.New("stored object failed integrity validation")
)

const MaxRawBytes int64 = 8 << 20
const maxManifestBytes int64 = 16 << 10

// Blob is a bounded, fully read object. ETags are deliberately not checksums.
type Blob struct {
	Bytes []byte
	MIME  string
}

// Backend implementations must provide atomic create-if-absent. Keys are
// internal storage paths, never accepted from the public HTTP request.
type Backend interface {
	AppID() string
	PutIfAbsent(context.Context, string, []byte, string) error
	Get(context.Context, string, int64) (Blob, error)
}
type CaptureInput struct {
	Key       string
	MIME      string
	SourceURI string
	SHA256    string
}
type Reference struct {
	ObjectID string `json:"object_id"`
	SHA256   string `json:"sha256"`
	Size     int64  `json:"size"`
	AppID    string `json:"app_id"`
}
type Capture struct {
	SchemaVersion string    `json:"schema_version"`
	AppID         string    `json:"app_id"`
	CaptureID     string    `json:"capture_id"`
	CaptureKey    string    `json:"capture_key"`
	MIME          string    `json:"mime"`
	SourceURI     string    `json:"source_uri"`
	CapturedAt    time.Time `json:"captured_at"`
	Object        Reference `json:"object"`
}
type publication struct {
	Version     int     `json:"version"`
	RequestHash string  `json:"request_hash"`
	Capture     Capture `json:"capture"`
}
type Store struct {
	app     string
	backend Backend
	now     func() time.Time
}

func appID(v string) bool {
	if v == "" || len(v) > 63 || v[0] < 'a' || v[0] > 'z' {
		return false
	}
	for _, r := range v {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
			return false
		}
	}
	return true
}
func keyID(v string) bool {
	if v == "" || len(v) > 128 {
		return false
	}
	for i, r := range v {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || i > 0 && (r == '-' || r == '_')) {
			return false
		}
	}
	return true
}
func captureID(v string) bool {
	if len(v) != 32 {
		return false
	}
	_, e := hex.DecodeString(v)
	return e == nil && strings.ToLower(v) == v
}
func NewStore(app string, backend Backend) (*Store, error) {
	if !appID(app) || backend == nil || backend.AppID() != app {
		return nil, ErrInvalid
	}
	return &Store{app: app, backend: backend, now: time.Now}, nil
}
func sum(b []byte) string              { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
func (s *Store) base(id string) string { return "apps/" + s.app + "/raw/" + id + "/" }
func requestDigest(in CaptureInput, size int64) string {
	b, _ := json.Marshal(struct {
		Key, MIME, SourceURI, SHA256 string
		Size                         int64
	}{in.Key, in.MIME, in.SourceURI, in.SHA256, size})
	return sum(b)
}
func validateInput(in CaptureInput, b []byte) error {
	if !keyID(in.Key) || len(in.SourceURI) > 2048 || len(in.SHA256) != 64 || in.SHA256 != sum(b) {
		return ErrInvalid
	}
	if in.SourceURI != "" {
		u, e := url.Parse(in.SourceURI)
		if e != nil || u.Host == "" || u.User != nil || (u.Scheme != "https" && u.Scheme != "http") {
			return ErrInvalid
		}
	}
	switch in.MIME {
	case "text/plain", "text/html", "text/markdown":
		if !utf8.Valid(b) {
			return ErrInvalid
		}
	case "image/jpeg":
		if len(b) < 3 || !bytes.Equal(b[:3], []byte{0xff, 0xd8, 0xff}) {
			return ErrInvalid
		}
	case "image/png":
		if !bytes.HasPrefix(b, []byte{0x89, 'P', 'N', 'G', 13, 10, 26, 10}) {
			return ErrInvalid
		}
	case "application/pdf":
		if !bytes.HasPrefix(b, []byte("%PDF-")) {
			return ErrInvalid
		}
	default:
		return ErrInvalid
	}
	return nil
}

// Capture publishes a manifest only after the original object is present and
// its bytes, MIME, size, and SHA-256 have been read back and verified.
func (s *Store) Capture(ctx context.Context, in CaptureInput, body io.Reader) (Capture, bool, error) {
	if body == nil {
		return Capture{}, false, ErrInvalid
	}
	b, e := io.ReadAll(io.LimitReader(body, MaxRawBytes+1))
	if e != nil {
		return Capture{}, false, ErrInvalid
	}
	if int64(len(b)) > MaxRawBytes {
		return Capture{}, false, ErrTooLarge
	}
	if e = validateInput(in, b); e != nil {
		return Capture{}, false, e
	}
	id := sum([]byte(s.app + "\x00" + in.Key))[:32]
	digest := requestDigest(in, int64(len(b)))
	old, e := s.publication(ctx, id)
	if e == nil {
		if old.RequestHash != digest {
			return Capture{}, false, ErrConflict
		}
		if _, e = s.verifiedSource(ctx, old.Capture); e != nil {
			return Capture{}, false, e
		}
		return old.Capture, true, nil
	}
	if !errors.Is(e, ErrNotFound) {
		return Capture{}, false, e
	}
	sourceKey := s.base(id) + "source"
	e = s.backend.PutIfAbsent(ctx, sourceKey, b, in.MIME)
	if e != nil && !errors.Is(e, ErrExists) {
		return Capture{}, false, e
	}
	// Detect a concurrent same-key/different-body writer before publishing.
	actual, e := s.backend.Get(ctx, sourceKey, MaxRawBytes)
	if e != nil {
		return Capture{}, false, e
	}
	if sum(actual.Bytes) != in.SHA256 || actual.MIME != in.MIME || len(actual.Bytes) != len(b) {
		return Capture{}, false, ErrConflict
	}
	c := Capture{SchemaVersion: "1", AppID: s.app, CaptureID: id, CaptureKey: in.Key, MIME: in.MIME, SourceURI: in.SourceURI, CapturedAt: s.now().UTC(), Object: Reference{ObjectID: "raw_" + id, SHA256: in.SHA256, Size: int64(len(b)), AppID: s.app}}
	manifest, _ := json.Marshal(publication{Version: 1, RequestHash: digest, Capture: c})
	e = s.backend.PutIfAbsent(ctx, s.base(id)+"manifest.json", manifest, "application/json")
	if e != nil && !errors.Is(e, ErrExists) {
		return Capture{}, false, e
	}
	// Return the winning immutable timestamp/metadata, including concurrent replay.
	published, readErr := s.publication(ctx, id)
	if readErr != nil {
		return Capture{}, false, readErr
	}
	if published.RequestHash != digest {
		return Capture{}, false, ErrConflict
	}
	if _, readErr = s.verifiedSource(ctx, published.Capture); readErr != nil {
		return Capture{}, false, readErr
	}
	return published.Capture, errors.Is(e, ErrExists), nil
}
func (s *Store) publication(ctx context.Context, id string) (publication, error) {
	if !captureID(id) {
		return publication{}, ErrNotFound
	}
	blob, e := s.backend.Get(ctx, s.base(id)+"manifest.json", maxManifestBytes)
	if e != nil {
		return publication{}, e
	}
	if blob.MIME != "application/json" {
		return publication{}, ErrCorrupt
	}
	var p publication
	d := json.NewDecoder(bytes.NewReader(blob.Bytes))
	d.DisallowUnknownFields()
	if d.Decode(&p) != nil || d.Decode(new(any)) != io.EOF {
		return publication{}, ErrCorrupt
	}
	c := p.Capture
	if p.Version != 1 || c.SchemaVersion != "1" || c.AppID != s.app || c.CaptureID != id || c.Object.AppID != s.app || c.Object.ObjectID != "raw_"+id || c.Object.Size < 0 || c.Object.Size > MaxRawBytes || c.CapturedAt.IsZero() || !keyID(c.CaptureKey) || len(c.Object.SHA256) != 64 {
		return publication{}, ErrCorrupt
	}
	in := CaptureInput{Key: c.CaptureKey, MIME: c.MIME, SourceURI: c.SourceURI, SHA256: c.Object.SHA256}
	if sum([]byte(s.app + "\x00" + c.CaptureKey))[:32] != id || p.RequestHash != requestDigest(in, c.Object.Size) {
		return publication{}, ErrCorrupt
	}
	return p, nil
}
func (s *Store) verifiedSource(ctx context.Context, c Capture) (Blob, error) {
	b, e := s.backend.Get(ctx, s.base(c.CaptureID)+"source", MaxRawBytes)
	if e != nil {
		return Blob{}, e
	}
	in := CaptureInput{Key: c.CaptureKey, MIME: c.MIME, SourceURI: c.SourceURI, SHA256: c.Object.SHA256}
	if int64(len(b.Bytes)) != c.Object.Size || b.MIME != c.MIME || validateInput(in, b.Bytes) != nil {
		return Blob{}, ErrCorrupt
	}
	return b, nil
}
func (s *Store) GetCapture(ctx context.Context, id string) (Capture, error) {
	p, e := s.publication(ctx, id)
	if e != nil {
		return Capture{}, e
	}
	if _, e = s.verifiedSource(ctx, p.Capture); e != nil {
		return Capture{}, e
	}
	return p.Capture, nil
}

// GetArtifact only resolves published raw IDs within this store's app. It
// never accepts a bucket, object path, or external URI from a caller.
func (s *Store) GetArtifact(ctx context.Context, objectID string) (Blob, error) {
	if !strings.HasPrefix(objectID, "raw_") {
		return Blob{}, ErrNotFound
	}
	p, e := s.publication(ctx, strings.TrimPrefix(objectID, "raw_"))
	if e != nil {
		return Blob{}, e
	}
	return s.verifiedSource(ctx, p.Capture)
}

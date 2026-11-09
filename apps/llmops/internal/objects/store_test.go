package objects

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

type memory struct {
	mu           sync.Mutex
	app          string
	items        map[string]Blob
	failManifest bool
}

func (m *memory) AppID() string { return m.app }
func (m *memory) PutIfAbsent(_ context.Context, key string, b []byte, mime string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failManifest && strings.HasSuffix(key, "manifest.json") {
		return ErrUnavailable
	}
	if _, ok := m.items[key]; ok {
		return ErrExists
	}
	m.items[key] = Blob{Bytes: append([]byte(nil), b...), MIME: mime}
	return nil
}
func (m *memory) Get(_ context.Context, key string, max int64) (Blob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.items[key]
	if !ok {
		return Blob{}, ErrNotFound
	}
	if int64(len(b.Bytes)) > max {
		return Blob{}, ErrTooLarge
	}
	b.Bytes = append([]byte(nil), b.Bytes...)
	return b, nil
}
func fixture(t *testing.T) (*Store, *memory) {
	t.Helper()
	m := &memory{app: "example", items: map[string]Blob{}}
	s, e := NewStore("example", m)
	if e != nil {
		t.Fatal(e)
	}
	return s, m
}
func input(key string, b []byte) CaptureInput {
	return CaptureInput{Key: key, MIME: "text/plain", SourceURI: "https://example.test/source", SHA256: sum(b)}
}
func TestOriginalBytesReplayAndMetadataConflict(t *testing.T) {
	s, _ := fixture(t)
	ctx := context.Background()
	b := []byte("材料\r\n玉ねぎ 1/2個\n")
	in := input("capture-1", b)
	c, reused, e := s.Capture(ctx, in, bytes.NewReader(b))
	if e != nil || reused {
		t.Fatalf("capture %v %v", reused, e)
	}
	again, reused, e := s.Capture(ctx, in, bytes.NewReader(b))
	if e != nil || !reused || again != c {
		t.Fatalf("replay %v %v", reused, e)
	}
	got, e := s.GetArtifact(ctx, c.Object.ObjectID)
	if e != nil || !bytes.Equal(got.Bytes, b) || got.MIME != in.MIME {
		t.Fatal("original bytes not preserved")
	}
	for _, change := range []func(*CaptureInput){func(i *CaptureInput) { i.SourceURI = "https://example.test/other" }, func(i *CaptureInput) { i.MIME = "text/markdown" }} {
		other := in
		change(&other)
		if _, _, e = s.Capture(ctx, other, bytes.NewReader(b)); !errors.Is(e, ErrConflict) {
			t.Fatal(e)
		}
	}
	b2 := []byte("different")
	if _, _, e = s.Capture(ctx, input(in.Key, b2), bytes.NewReader(b2)); !errors.Is(e, ErrConflict) {
		t.Fatal(e)
	}
}
func TestConcurrentImmutableCapture(t *testing.T) {
	s, m := fixture(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	var success, conflict atomic.Int32
	for n := 0; n < 30; n++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			b := []byte("first")
			if n%2 == 1 {
				b = []byte("second")
			}
			_, _, e := s.Capture(ctx, input("same", b), bytes.NewReader(b))
			if e == nil {
				success.Add(1)
			} else if errors.Is(e, ErrConflict) {
				conflict.Add(1)
			} else {
				t.Errorf("unexpected %v", e)
			}
		}(n)
	}
	wg.Wait()
	if success.Load() != 15 || conflict.Load() != 15 {
		t.Fatalf("success %d conflict %d", success.Load(), conflict.Load())
	}
	if len(m.items) != 2 {
		t.Fatal("duplicate files")
	}
}
func TestUnpublishedSourceIsNotReadable(t *testing.T) {
	s, m := fixture(t)
	ctx := context.Background()
	b := []byte("sample")
	in := input("same", b)
	m.failManifest = true
	if _, _, e := s.Capture(ctx, in, bytes.NewReader(b)); !errors.Is(e, ErrUnavailable) {
		t.Fatal(e)
	}
	id := sum([]byte("example\x00same"))[:32]
	if _, e := s.GetCapture(ctx, id); !errors.Is(e, ErrNotFound) {
		t.Fatal(e)
	}
	if _, e := s.GetArtifact(ctx, "raw_"+id); !errors.Is(e, ErrNotFound) {
		t.Fatal(e)
	}
	m.failManifest = false
	c, _, e := s.Capture(ctx, in, bytes.NewReader(b))
	if e != nil || c.CaptureID != id {
		t.Fatal(e)
	}
}
func TestInvalidInputAndForeignArtifact(t *testing.T) {
	s, _ := fixture(t)
	ctx := context.Background()
	b := []byte("sample")
	for _, change := range []func(*CaptureInput){func(i *CaptureInput) { i.Key = "../other" }, func(i *CaptureInput) { i.SHA256 = "bad" }, func(i *CaptureInput) { i.MIME = "application/javascript" }, func(i *CaptureInput) { i.MIME = "image/png" }, func(i *CaptureInput) { i.SourceURI = "file:///etc/passwd" }, func(i *CaptureInput) { i.SourceURI = "https://u:p@example.test/" }} {
		in := input("good", b)
		change(&in)
		if _, _, e := s.Capture(ctx, in, bytes.NewReader(b)); !errors.Is(e, ErrInvalid) {
			t.Fatal(e)
		}
	}
	huge := bytes.Repeat([]byte("x"), int(MaxRawBytes+1))
	if _, _, e := s.Capture(ctx, input("huge", huge), bytes.NewReader(huge)); !errors.Is(e, ErrTooLarge) {
		t.Fatal(e)
	}
	for _, id := range []string{"s3://bucket/path", "raw_../../other", "apps/another/raw/source", "raw_a"} {
		if _, e := s.GetArtifact(ctx, id); !errors.Is(e, ErrNotFound) {
			t.Fatal(e)
		}
	}
	if _, e := NewStore("other", s.backend); !errors.Is(e, ErrInvalid) {
		t.Fatal("backend app mismatch accepted")
	}
}
func TestStoredCorruptionDetected(t *testing.T) {
	for _, mode := range []string{"source", "manifest", "owner"} {
		t.Run(mode, func(t *testing.T) {
			s, m := fixture(t)
			ctx := context.Background()
			b := []byte("sample")
			c, _, e := s.Capture(ctx, input("same", b), bytes.NewReader(b))
			if e != nil {
				t.Fatal(e)
			}
			switch mode {
			case "source":
				m.items[s.base(c.CaptureID)+"source"] = Blob{Bytes: []byte("tampered"), MIME: c.MIME}
			case "manifest":
				m.items[s.base(c.CaptureID)+"manifest.json"] = Blob{Bytes: []byte("not json"), MIME: "application/json"}
			case "owner":
				key := s.base(c.CaptureID) + "manifest.json"
				v := m.items[key]
				v.Bytes = bytes.ReplaceAll(v.Bytes, []byte(`"app_id":"example"`), []byte(`"app_id":"another"`))
				m.items[key] = v
			}
			if _, e = s.GetArtifact(ctx, c.Object.ObjectID); !errors.Is(e, ErrCorrupt) {
				t.Fatalf("%s: %v", mode, e)
			}
		})
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }
func TestReaderFailureNeverPublishes(t *testing.T) {
	s, m := fixture(t)
	if _, _, e := s.Capture(context.Background(), CaptureInput{}, failingReader{}); e == nil {
		t.Fatal("read failed but accepted")
	}
	if len(m.items) != 0 {
		t.Fatal("published read failure")
	}
}

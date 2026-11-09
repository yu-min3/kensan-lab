package objects

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
)

func s3Config(endpoint string) S3Config {
	return S3Config{App: "example", Bucket: "test-bucket", Region: "test-region", Endpoint: endpoint, Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
		return aws.Credentials{AccessKeyID: "test-access", SecretAccessKey: "test-secret", Source: "test"}, nil
	})}
}
func TestS3ConditionalHeaderAndOriginalBytes(t *testing.T) {
	var mu sync.Mutex
	items := map[string]Blob{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256") {
			t.Error("request not signed")
		}
		if !strings.HasPrefix(r.URL.Path, "/test-bucket/apps/example/") {
			t.Error("unexpected bucket/prefix")
		}
		mu.Lock()
		defer mu.Unlock()
		switch r.Method {
		case "PUT":
			if r.Header.Get("If-None-Match") != "*" {
				t.Error("conditional create not sent")
			}
			if _, ok := items[r.URL.Path]; ok {
				w.WriteHeader(412)
				return
			}
			b, _ := io.ReadAll(r.Body)
			items[r.URL.Path] = Blob{Bytes: b, MIME: r.Header.Get("Content-Type")}
			w.WriteHeader(200)
		case "GET":
			v, ok := items[r.URL.Path]
			if !ok {
				w.WriteHeader(404)
				return
			}
			w.Header().Set("Content-Type", v.MIME)
			w.Write(v.Bytes)
		default:
			t.Error("unexpected operation")
			w.WriteHeader(405)
		}
	}))
	defer srv.Close()
	backend, e := NewS3(s3Config(srv.URL))
	if e != nil {
		t.Fatal(e)
	}
	store, e := NewStore("example", backend)
	if e != nil {
		t.Fatal(e)
	}
	b := []byte("original\r\nbytes")
	in := input("one", b)
	c, _, e := store.Capture(context.Background(), in, bytes.NewReader(b))
	if e != nil {
		t.Fatal(e)
	}
	_, reused, e := store.Capture(context.Background(), in, bytes.NewReader(b))
	if e != nil || !reused {
		t.Fatal(e)
	}
	blob, e := store.GetArtifact(context.Background(), c.Object.ObjectID)
	if e != nil || !bytes.Equal(blob.Bytes, b) {
		t.Fatal(e)
	}
	if e = backend.PutIfAbsent(context.Background(), "apps/example/raw/one/source", []byte("x"), "text/plain"); e != nil {
		t.Fatal(e)
	}
	if e = backend.PutIfAbsent(context.Background(), "apps/example/raw/one/source", []byte("y"), "text/plain"); e != ErrExists {
		t.Fatalf("412 mapping %v", e)
	}
	if _, e = backend.Get(context.Background(), "apps/example/raw/missing", 10); e != ErrNotFound {
		t.Fatalf("404 mapping %v", e)
	}
	for _, key := range []string{"apps/other/raw/one", "apps/example/../other/key", "s3://bucket/key", "apps/example/raw/%2fother"} {
		if _, e = backend.Get(context.Background(), key, 10); e != ErrInvalid {
			t.Fatal(e)
		}
	}
}
func TestS3FaultsAndBounds(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		max    int64
		want   error
	}{
		{"forbidden", 403, "secret upstream error", 10, ErrUnavailable},
		{"missing", 404, "", 10, ErrNotFound},
		{"too-large", 200, "12345678901", 10, ErrTooLarge},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status); w.Write([]byte(tc.body)) }))
			defer srv.Close()
			c, e := NewS3(s3Config(srv.URL))
			if e != nil {
				t.Fatal(e)
			}
			_, e = c.Get(context.Background(), "apps/example/raw/key", tc.max)
			if e != tc.want {
				t.Fatalf("got %v want %v", e, tc.want)
			}
			if e != nil && strings.Contains(e.Error(), "secret") {
				t.Fatal("upstream leaked")
			}
		})
	}
	for _, endpoint := range []string{"http://localhost:9000", "http://object.example", "https://u:p@object.example", "https://object.example/prefix", "https://object.example?secret=x"} {
		if _, e := NewS3(s3Config(endpoint)); e != ErrInvalid {
			t.Fatal("unsafe endpoint accepted")
		}
	}
}

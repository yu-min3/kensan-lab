// Verify failure replay and retention guards against the isolated real-service stack.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/yu-min3/kensan-lab/apps/llmops/internal/objects"
	"github.com/yu-min3/kensan-lab/apps/llmops/internal/worker"
	"github.com/yu-min3/kensan-lab/apps/llmops/internal/worker/dataset"
	"os"
	"path/filepath"
	"strings"
)

type failOnce struct {
	objects.Backend
	failed bool
}

func (b *failOnce) PutIfAbsent(ctx context.Context, key string, body []byte, mime string) error {
	if strings.HasSuffix(key, "/manifest.json") && !b.failed {
		b.failed = true
		return objects.ErrUnavailable
	}
	return b.Backend.PutIfAbsent(ctx, key, body, mime)
}

type corrupt struct{ objects.Backend }

func (b corrupt) Get(ctx context.Context, key string, max int64) (objects.Blob, error) {
	blob, e := b.Backend.Get(ctx, key, max)
	if e == nil && strings.Contains(key, "/raw/") && strings.HasSuffix(key, "/source") && len(blob.Bytes) > 0 {
		blob.Bytes[0] ^= 1
	}
	return blob, e
}
func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) != 3 {
		return errors.New("config/spec required")
	}
	ctx := context.Background()
	raw, e := os.ReadFile(os.Args[1])
	if e != nil {
		return e
	}
	cfg, e := worker.DecodeConfig(raw)
	if e != nil {
		return e
	}
	cfg.Role = "transform"
	cfg.Storage.CredentialFile = filepath.Join(filepath.Dir(cfg.Storage.CredentialFile), "transform-konro.json")
	cfg.Catalog.ClientID = "catalog-konro-transform"
	cfg.Catalog.ClientSecretFile = filepath.Join(filepath.Dir(cfg.Catalog.ClientSecretFile), "catalog-konro-transform.secret")
	r, e := worker.New(cfg)
	if e != nil {
		return e
	}
	backend, e := r.Objects()
	if e != nil {
		return e
	}
	raw, e = os.ReadFile(os.Args[2])
	if e != nil {
		return e
	}
	var spec dataset.Spec
	if json.Unmarshal(raw, &spec) != nil {
		return errors.New("invalid fixture")
	}
	spec.ReleaseID += "-replay"
	wrapped := &failOnce{Backend: backend}
	if _, e = dataset.Build(ctx, r, cfg, wrapped, spec); !errors.Is(e, objects.ErrUnavailable) || !wrapped.failed {
		return errors.New("manifest failpoint did not fire")
	}
	if _, e = dataset.Read(ctx, r, cfg, backend, spec.ReleaseID); !errors.Is(e, objects.ErrNotFound) {
		return errors.New("unpublished release readable")
	}
	release, e := dataset.Build(ctx, r, cfg, wrapped, spec)
	if e != nil {
		return e
	}
	cases, e := dataset.Read(ctx, r, cfg, backend, spec.ReleaseID)
	if e != nil || len(cases) != 20 {
		return errors.New("replay failed")
	}
	if e = dataset.GuardRawRemoval(ctx, backend, release.Raw[0]); !errors.Is(e, dataset.ErrPinned) {
		return errors.New("Raw guard failed")
	}
	if e = dataset.GuardSnapshotRemoval(ctx, r, cfg.Catalog, release.Tables[1].SnapshotID); !errors.Is(e, dataset.ErrPinned) {
		return errors.New("snapshot guard failed")
	}
	if _, e = dataset.Read(ctx, r, cfg, corrupt{backend}, spec.ReleaseID); !errors.Is(e, objects.ErrCorrupt) {
		return errors.New("corrupt Raw was accepted")
	}
	fmt.Println(`{"failed_publication_replay":true,"unpublished_release_rejected":true,"raw_pin_guard":true,"snapshot_tag_guard":true,"corrupt_raw_rejected":true}`)
	return nil
}

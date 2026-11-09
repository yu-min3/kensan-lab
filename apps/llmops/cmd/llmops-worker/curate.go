package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"github.com/yu-min3/kensan-lab/apps/llmops/internal/objects"
	"github.com/yu-min3/kensan-lab/apps/llmops/internal/worker"
	"github.com/yu-min3/kensan-lab/apps/llmops/internal/worker/catalog"
	"github.com/yu-min3/kensan-lab/apps/llmops/internal/worker/curate"
	"io"
	"os"
)

func runCurate(args []string) error {
	flags := flag.NewFlagSet("curate", flag.ContinueOnError)
	path := flags.String("config", "", "operator config")
	id := flags.String("adapter", "", "registered adapter")
	version := flags.String("transform-version", "", "transform release")
	if flags.Parse(args) != nil {
		return worker.ErrConfig
	}
	b, e := os.ReadFile(*path)
	if e != nil || len(b) > 1<<20 {
		return worker.ErrConfig
	}
	cfg, e := worker.DecodeConfig(b)
	if e != nil {
		return e
	}
	runner, e := worker.New(cfg)
	if e != nil {
		return e
	}
	if cfg.Catalog == nil {
		return worker.ErrConfig
	}
	adapter, ok := cfg.Adapters[*id]
	if !ok {
		return worker.ErrConfig
	}
	backend, e := runner.Objects()
	if e != nil {
		return e
	}
	store, e := objects.NewStore(runner.App(), backend)
	if e != nil {
		return e
	}
	var inputs []curate.Input
	input, readErr := io.ReadAll(io.LimitReader(os.Stdin, (1<<20)+1))
	if readErr != nil || len(input) > 1<<20 {
		return worker.ErrRequest
	}
	decoder := json.NewDecoder(bytes.NewReader(input))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&inputs) != nil || decoder.Decode(new(any)) != io.EOF {
		return worker.ErrRequest
	}
	rows, e := curate.Prepare(context.Background(), runner, store, *id, adapter, *version, inputs)
	if e != nil {
		return e
	}
	result, e := catalog.Commit(context.Background(), runner, cfg.Catalog, rows)
	if e != nil {
		return e
	}
	return json.NewEncoder(os.Stdout).Encode(result)
}

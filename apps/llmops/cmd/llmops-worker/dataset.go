package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"github.com/yu-min3/kensan-lab/apps/llmops/internal/worker"
	"github.com/yu-min3/kensan-lab/apps/llmops/internal/worker/dataset"
	"io"
	"os"
)

func runDataset(args []string) error {
	if len(args) == 0 {
		return worker.ErrRequest
	}
	operation := args[0]
	flags := flag.NewFlagSet("dataset", flag.ContinueOnError)
	path := flags.String("config", "", "operator config")
	releaseID := flags.String("release", "", "fixed release ID")
	if flags.Parse(args[1:]) != nil {
		return worker.ErrConfig
	}
	body, e := os.ReadFile(*path)
	if e != nil || len(body) > 1<<20 {
		return worker.ErrConfig
	}
	cfg, e := worker.DecodeConfig(body)
	if e != nil {
		return e
	}
	r, e := worker.New(cfg)
	if e != nil {
		return e
	}
	backend, e := r.Objects()
	if e != nil {
		return e
	}
	ctx := context.Background()
	switch operation {
	case "build":
		input, e := io.ReadAll(io.LimitReader(os.Stdin, (1<<20)+1))
		if e != nil || len(input) > 1<<20 {
			return worker.ErrRequest
		}
		var spec dataset.Spec
		d := json.NewDecoder(bytes.NewReader(input))
		d.DisallowUnknownFields()
		if d.Decode(&spec) != nil || d.Decode(new(any)) != io.EOF {
			return worker.ErrRequest
		}
		out, e := dataset.Build(ctx, r, cfg, backend, spec)
		if e != nil {
			return e
		}
		return json.NewEncoder(os.Stdout).Encode(out)
	case "read":
		out, e := dataset.Read(ctx, r, cfg, backend, *releaseID)
		if e != nil {
			return e
		}
		return json.NewEncoder(os.Stdout).Encode(out)
	default:
		return worker.ErrRequest
	}
}

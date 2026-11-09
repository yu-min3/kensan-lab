// llmops-worker invokes a configured, pinned adapter. Requests cannot choose binaries.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/yu-min3/kensan-lab/apps/llmops/internal/worker"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "curate" {
		if e := runCurate(os.Args[2:]); e != nil {
			fmt.Fprintln(os.Stderr, e)
			os.Exit(1)
		}
		return
	}
	config := flag.String("config", "", "operator configuration file")
	adapter := flag.String("adapter", "", "registered adapter identifier")
	flag.Parse()
	if e := run(*config, *adapter); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run(path, id string) error {
	b, e := os.ReadFile(path)
	if e != nil || len(b) > 1<<20 {
		return worker.ErrConfig
	}
	c, e := worker.DecodeConfig(b)
	if e != nil {
		return e
	}
	r, e := worker.New(c)
	if e != nil {
		return e
	}
	max := c.MaxInputBytes
	if max == 0 {
		max = 8 << 20
	}
	input, e := io.ReadAll(io.LimitReader(os.Stdin, int64(max)+1))
	if e != nil || len(input) > max {
		return worker.ErrRequest
	}
	var request worker.Request
	d := json.NewDecoder(bytes.NewReader(input))
	d.DisallowUnknownFields()
	if d.Decode(&request) != nil || d.Decode(new(any)) != io.EOF {
		return worker.ErrRequest
	}
	response, e := r.Call(context.Background(), id, request)
	if e != nil {
		return e
	}
	return json.NewEncoder(os.Stdout).Encode(response)
}

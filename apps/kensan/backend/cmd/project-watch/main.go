// project-watch is a short-lived scheduler client; provider credentials stay on agent-runtime.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yu-min3/kensan-lab/apps/kensan/backend/internal/agentclient"
	"github.com/yu-min3/kensan-lab/apps/kensan/backend/internal/projectwatch"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	dry := flag.Bool("dry-run", false, "collect inputs without contacting runtime or writing state")
	reset := flag.Bool("reset-failed", false, "start a new revision after verifying the previous job has terminated")
	flag.Parse()
	root := os.Getenv("KENSAN_DATA_DIR")
	if root == "" {
		return fmt.Errorf("KENSAN_DATA_DIR is required")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	if *dry {
		snap, e := projectwatch.Collect(root)
		if e != nil {
			return e
		}
		return json.NewEncoder(os.Stdout).Encode(map[string]any{"inputHash": snap.Hash, "sources": len(snap.Sources), "projects": len(snap.Projects)})
	}
	g, e := agentclient.FromEnvironment(ctx)
	if e != nil {
		return e
	}
	if *reset {
		if e = projectwatch.ResetFailed(ctx, root, g); e != nil {
			return e
		}
	}
	for {
		state, e := projectwatch.Step(ctx, root, g, time.Now())
		if e != nil {
			return e
		}
		// Never print prompts, source text or provider errors.
		if e = json.NewEncoder(os.Stdout).Encode(map[string]any{"status": state.Status, "reason": state.Reason, "checkedAt": state.CheckedAt}); e != nil {
			return e
		}
		switch state.Status {
		case "queued", "running", "connection_error":
			select {
			case <-ctx.Done():
				return fmt.Errorf("waiting stopped; next run resumes the same job")
			case <-time.After(5 * time.Second):
			}
		case "failed", "cancelled", "invalid_state":
			return fmt.Errorf("project-watch stopped: %s", state.Reason)
		default:
			return nil
		}
	}
}

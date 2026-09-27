package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/core"
	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/isolation"
	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/mock"
	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/web"
	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/workerclient"
	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/worktree"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	addr := flag.String("listen", "127.0.0.1:8787", "numeric loopback address")
	data := flag.String("data", "", "absolute directory for canonical state")
	token := flag.String("admin-token-file", "", "mode 0600 file with local admin token")
	tokensCSS := flag.String("tokens-css", "", "path to kensan-lab design tokens.css")
	mockWorker := flag.Bool("mock-worker", false, "run simulation-only worker; never invokes models or publishes")
	isolatedWorker := flag.Bool("isolated-worker", false, "opt in to sandboxed subscription model turns; requires host preflight")
	bubblewrap := flag.String("bwrap", "", "absolute bubblewrap binary path")
	runtimeRoot := flag.String("worker-rootfs", "", "dedicated non-secret Linux worker rootfs")
	workerProgram := flag.String("worker-program", "/usr/local/bin/sense-dev-worker", "worker binary path inside rootfs")
	worktreeRoot := flag.String("worker-worktree-root", "", "private root for task-scoped Git worktrees")
	sourceRepo := flag.String("source-repo", "", "local trusted Git repository used to create task worktrees")
	claudeAuth := flag.String("claude-auth-home", "", "private Claude subscription configuration directory")
	codexAuth := flag.String("codex-auth-home", "", "private Codex subscription configuration directory")
	turnTimeout := flag.Duration("turn-timeout", 45*time.Minute, "maximum duration of one isolated model turn")
	inferenceWindow := flag.String("inference-window", "", "required JST HH:MM-HH:MM interval for isolated model dispatch")
	flag.Parse()
	if *data == "" || *token == "" || *tokensCSS == "" || !filepath.IsAbs(*data) {
		return errors.New("-data, -admin-token-file and -tokens-css are required; -data must be absolute")
	}
	if err := web.LoopbackOnly(*addr); err != nil {
		return err
	}
	if *mockWorker && *isolatedWorker {
		return errors.New("mock and isolated workers are mutually exclusive")
	}
	store, err := core.Open(*data)
	if err != nil {
		return err
	}
	defer store.Close()
	if err := store.SeedKnowledge(); err != nil {
		return err
	}
	if err := store.RecoverInterrupted(); err != nil {
		return err
	}
	mode := "off"
	var runner core.Runner
	var scope []string
	var allowedNow func(time.Time) bool
	var worktrees worktree.Manager
	if *mockWorker {
		mode, runner, scope = "mock", mock.Runner{}, []string{"simulation-only"}
	}
	if *isolatedWorker {
		if err := rejectSimulationHistory(store.Snapshot()); err != nil {
			return err
		}
		if *bubblewrap == "" || *runtimeRoot == "" || *worktreeRoot == "" || *sourceRepo == "" || *claudeAuth == "" || *codexAuth == "" || *turnTimeout <= 0 {
			return errors.New("isolated worker paths and positive timeout are required")
		}
		window, err := parseRunWindow(*inferenceWindow)
		if err != nil {
			return err
		}
		canonicalState, err := filepath.EvalSymlinks(*data)
		if err != nil {
			return err
		}
		canonicalToken, err := filepath.EvalSymlinks(*token)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(canonicalState, canonicalToken)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return errors.New("admin token must be inside controller state when isolated workers are enabled")
		}
		worktrees = worktree.Manager{Source: *sourceRepo, Root: *worktreeRoot}
		base := isolation.Config{Bubblewrap: *bubblewrap, RuntimeRoot: *runtimeRoot, Worktree: *worktreeRoot, ControllerState: *data}
		claudeConfig, codexConfig := base, base
		claudeConfig.AuthHome, codexConfig.AuthHome = *claudeAuth, *codexAuth
		isolated := workerclient.Runner{Claude: claudeConfig, Codex: codexConfig, WorkerProgram: *workerProgram, Timeout: *turnTimeout, Worktrees: worktrees}
		if err := isolated.Preflight(context.Background()); err != nil {
			return fmt.Errorf("isolated worker disabled: %w", err)
		}
		mode, runner, scope = "isolated", isolated, []string{"isolated-model-worker"}
		allowedNow = window.contains
	}
	handler, err := web.New(store, *token, *tokensCSS, mode, allowedNow)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		return err
	}
	defer listener.Close()
	server := &http.Server{Handler: handler.Handler(), ReadHeaderTimeout: 5 * time.Second, MaxHeaderBytes: 1 << 20}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if runner != nil {
		go func() {
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if allowedNow != nil && !allowedNow(time.Now()) {
						continue
					}
					if mode == "isolated" {
						if err := prepareReadyWorktrees(ctx, store, worktrees); err != nil {
							log.Print("task worktree preparation needs operator inspection")
						}
					}
					if _, err := store.Tick(ctx, runner, scope); err != nil && !errors.Is(err, context.Canceled) {
						log.Print("worker dispatch needs operator inspection")
					}
				}
			}
		}()
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	fmt.Printf("sense-dev private listener: %s\n", listener.Addr())
	err = server.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

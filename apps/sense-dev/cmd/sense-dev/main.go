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
	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/reportdelivery"
	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/verifier"
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
	codexBubblewrap := flag.String("codex-bwrap", "", "optional separately confined Codex bubblewrap launcher")
	runtimeRoot := flag.String("worker-rootfs", "", "dedicated non-secret Linux worker rootfs")
	verifierRoot := flag.String("verifier-rootfs", "", "separate credentialless test rootfs")
	workerProgram := flag.String("worker-program", "/usr/local/bin/sense-dev-worker", "worker binary path inside rootfs")
	worktreeRoot := flag.String("worker-worktree-root", "", "private root for task-scoped Git worktrees")
	sourceRepo := flag.String("source-repo", "", "local trusted Git repository used to create task worktrees")
	claudeAuth := flag.String("claude-auth-home", "", "private Claude subscription configuration directory")
	codexAuth := flag.String("codex-auth-home", "", "private Codex subscription configuration directory")
	turnTimeout := flag.Duration("turn-timeout", 45*time.Minute, "maximum duration of one isolated model turn")
	inferenceWindow := flag.String("inference-window", "", "required JST HH:MM-HH:MM interval for isolated model dispatch")
	verificationPlan := flag.String("verification-plan", "", "operator-owned JSON plan for credentialless tests")
	modelLimit := flag.Int("model-attempt-limit", 0, "optional finite-run model budget; stops on failures and decisions")
	reportChannel := flag.String("report-slack-channel", "", "fixed Slack channel ID for daily report")
	reportToken := flag.String("report-slack-token-file", "", "mode 0600 Slack bot token file")
	reportBaseURL := flag.String("report-base-url", "", "HTTPS mobile link base for daily report")
	flag.Parse()
	if *modelLimit < 0 {
		return errors.New("model-attempt-limit must not be negative")
	}
	if *data == "" || *token == "" || *tokensCSS == "" || !filepath.IsAbs(*data) {
		return errors.New("-data, -admin-token-file and -tokens-css are required; -data must be absolute")
	}
	if err := web.LoopbackOnly(*addr); err != nil {
		return err
	}
	if *mockWorker && *isolatedWorker {
		return errors.New("mock and isolated workers are mutually exclusive")
	}
	reportConfigured := *reportChannel != "" || *reportToken != "" || *reportBaseURL != ""
	reportSender := reportdelivery.Slack{TokenFile: *reportToken, BaseURL: *reportBaseURL}
	if reportConfigured {
		if *reportChannel == "" || *reportToken == "" || *reportBaseURL == "" {
			return errors.New("Slack report channel, token file and HTTPS base URL are required together")
		}
		if err := reportSender.Validate(); err != nil {
			return err
		}
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
	if err := store.RecoverSendingReports(); err != nil {
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
		if *bubblewrap == "" || *runtimeRoot == "" || *verifierRoot == "" || *worktreeRoot == "" || *sourceRepo == "" || *claudeAuth == "" || *codexAuth == "" || *verificationPlan == "" || *turnTimeout <= 0 {
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
		if *codexBubblewrap != "" {
			codexConfig.Bubblewrap = *codexBubblewrap
		}
		isolated := workerclient.Runner{Claude: claudeConfig, Codex: codexConfig, WorkerProgram: *workerProgram, Timeout: *turnTimeout, Worktrees: worktrees}
		if err := isolated.Preflight(context.Background()); err != nil {
			return fmt.Errorf("isolated worker disabled: %w", err)
		}
		plan, err := verifier.LoadPlan(*verificationPlan, *data, *worktreeRoot, *claudeAuth, *codexAuth)
		if err != nil {
			return fmt.Errorf("verification plan rejected: %w", err)
		}
		verifierConfig := claudeConfig
		verifierConfig.RuntimeRoot = *verifierRoot
		checks := verifier.Runner{Sandbox: verifierConfig, Worktrees: worktrees, WorkerProgram: *workerProgram, Plan: plan}
		if err := checks.Preflight(context.Background()); err != nil {
			return fmt.Errorf("credentialless verifier disabled: %w", err)
		}
		mode, runner, scope = "isolated", stageRunner{models: isolated, checks: checks}, []string{"isolated-model-worker"}
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
	go func() {
		queue := func() {
			entry, _, err := store.QueueDailyReport(time.Now())
			if err != nil {
				log.Print("daily report outbox needs operator inspection")
				return
			}
			if !reportConfigured || entry.Date == "" {
				return
			}
			if entry.Status == "waiting_destination" {
				if err := store.ConfigureReportDestination(entry.Date, *reportChannel); err != nil {
					log.Print("daily report destination rejected")
					return
				}
				entry = store.Snapshot().ReportOutbox[entry.Date]
			}
			if entry.Status == "queued" {
				if entry.Destination != *reportChannel {
					log.Print("daily report destination differs from configured channel; inspect outbox")
					return
				}
				ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
				defer cancel()
				if err := store.DeliverQueuedReport(ctx, entry.Date, reportSender, time.Now()); err != nil {
					log.Print("daily report delivery unknown; inspect outbox before retry")
				}
			}
		}
		queue()
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				queue()
			}
		}
	}()
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
					// 固まった worker が provider を握ったままにならないよう、
					// 配車の前に lease 切れを回収する。再実行はしない。
					if n, err := store.ExpireLeases(time.Now()); err != nil {
						log.Print("lease expiry needs operator inspection")
					} else if n > 0 {
						log.Printf("released %d expired attempt lease(s); inspect worker output before retry", n)
					}
					if mode == "isolated" {
						if err := prepareReadyWorktrees(ctx, store, worktrees); err != nil {
							log.Print("task worktree preparation needs operator inspection")
						}
					}
					if _, err := store.TickBounded(ctx, runner, scope, *modelLimit); err != nil && !errors.Is(err, context.Canceled) {
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

type stageRunner struct {
	models workerclient.Runner
	checks verifier.Runner
}

func (r stageRunner) Run(ctx context.Context, dispatch core.Dispatch) (core.RunResult, error) {
	if dispatch.Attempt.Role == "verification" {
		return r.checks.Run(ctx, dispatch)
	}
	return r.models.Run(ctx, dispatch)
}

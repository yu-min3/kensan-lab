// kensan — ファイルベースのナレッジ & ゴール管理アプリのバックエンド。
//
// 使い方:
//
//	kensan            # serve（既定）
//	kensan serve      # API サーバー起動
//
// Phase 2 で task move 等の CLI サブコマンドが同じバイナリに追加される。
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/yu-min3/kensan-lab/apps/kensan-next/backend/internal/api"
	"github.com/yu-min3/kensan-lab/apps/kensan-next/backend/internal/telemetry"
)

func main() {
	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	switch cmd {
	case "serve":
		if err := serve(); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\nusage: kensan [serve]\n", cmd)
		os.Exit(2)
	}
}

func serve() error {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))

	root := os.Getenv("KENSAN_DATA_DIR")
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		root = filepath.Join(home, "kensan-workspace")
	}
	if _, err := os.Stat(root); err != nil {
		return fmt.Errorf("KENSAN_DATA_DIR not accessible: %w", err)
	}
	addr := os.Getenv("KENSAN_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdown, otelEnabled, err := telemetry.Setup(ctx, "kensan")
	if err != nil {
		return err
	}
	defer func() {
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdown(sctx)
	}()

	handler := otelhttp.NewHandler(api.New(root, log).Handler(), "kensan-api")
	srv := &http.Server{Addr: addr, Handler: handler}

	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(sctx)
	}()

	log.Info("kensan backend listening", "addr", addr, "dataDir", root, "otel", otelEnabled)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

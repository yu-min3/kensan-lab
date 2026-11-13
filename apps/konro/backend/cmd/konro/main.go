// konro server: recipe API + SPA. See apps/konro/README.md.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/agentclient"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/api"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/telemetry"
)

func main() {
	dataDir := envOr("KONRO_DATA_DIR", os.Getenv("HOME")+"/konro-data")
	staticDir := os.Getenv("KONRO_STATIC_DIR") // empty = API only (vite dev serves the UI)
	addr := envOr("KONRO_ADDR", ":8090")

	// Tracing is optional; a misconfigured exporter must not stop the app.
	shutdown, err := telemetry.Setup(context.Background())
	if err != nil {
		log.Printf("konro: tracing disabled: %v", err)
		shutdown = func(context.Context) error { return nil }
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdown(ctx)
	}()

	agent, err := agentclient.FromEnvironment(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("konro: data=%s static=%s addr=%s", dataDir, staticDir, addr)
	server := &http.Server{Addr: addr, Handler: api.New(dataDir, staticDir, api.WithAgent(agent))}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(c)
	}()
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

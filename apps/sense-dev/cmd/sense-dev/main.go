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
	"syscall"
	"time"

	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/core"
	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/web"
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
	flag.Parse()
	if *data == "" || *token == "" || *tokensCSS == "" || !filepath.IsAbs(*data) {
		return errors.New("-data, -admin-token-file and -tokens-css are required; -data must be absolute")
	}
	if err := web.LoopbackOnly(*addr); err != nil {
		return err
	}
	store, err := core.Open(*data)
	if err != nil {
		return err
	}
	defer store.Close()
	if err := store.SeedKnowledge(); err != nil {
		return err
	}
	handler, err := web.New(store, *token, *tokensCSS)
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

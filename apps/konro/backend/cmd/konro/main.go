// konro server: recipe API + SPA. See apps/konro/README.md.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/api"
)

func main() {
	dataDir := envOr("KONRO_DATA_DIR", os.Getenv("HOME")+"/konro-data")
	staticDir := os.Getenv("KONRO_STATIC_DIR") // empty = API only (vite dev serves the UI)
	addr := envOr("KONRO_ADDR", ":8090")

	log.Printf("konro: data=%s static=%s addr=%s", dataDir, staticDir, addr)
	if err := http.ListenAndServe(addr, api.New(dataDir, staticDir)); err != nil {
		log.Fatal(err)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// konro server: recipe API + SPA. See apps/konro/README.md.
package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/agentclient"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/api"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/importer"
)

func main() {
	dataDir := envOr("KONRO_DATA_DIR", os.Getenv("HOME")+"/konro-data")
	staticDir := os.Getenv("KONRO_STATIC_DIR") // empty = API only (vite dev serves the UI)
	addr := envOr("KONRO_ADDR", ":8090")

	agent, err := agentclient.FromEnvironment(context.Background())
	if err != nil {
		log.Printf("direct AI接続を利用できません。手入力と保存済みデータは利用できます")
		agent = nil
	}
	routing, err := importRouting(context.Background(), agent)
	if err != nil {
		log.Fatal(err)
	}
	imports, err := importer.NewRouted(dataDir, routing)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("konro: data=%s static=%s addr=%s", dataDir, staticDir, addr)
	if err := http.ListenAndServe(addr, api.New(dataDir, staticDir, api.WithAgent(agent), api.WithImportService(imports))); err != nil {
		log.Fatal(err)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

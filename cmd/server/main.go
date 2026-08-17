// Command server is the BFF's entrypoint. Vercel's Go Framework Preset
// auto-detects this path (cmd/server/main.go) and runs the resulting
// binary as a long-lived process listening on $PORT.
package main

import (
	"log"
	"net/http"

	"github.com/joecris/tdff-bff/internal/config"
	"github.com/joecris/tdff-bff/internal/router"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	handler := router.New(cfg)

	addr := ":" + cfg.Port
	log.Printf("tdff-bff listening on %s (env=%s)", addr, cfg.AppEnv)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("server: %v", err)
	}
}

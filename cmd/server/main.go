// Command server is the BFF's entrypoint. Vercel's Go Framework Preset
// auto-detects this path (cmd/server/main.go) and runs the resulting
// binary as a long-lived process listening on $PORT.
package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/joecris/tdff-bff/internal/auth"
	"github.com/joecris/tdff-bff/internal/config"
	"github.com/joecris/tdff-bff/internal/router"
	"github.com/joecris/tdff-bff/internal/store/memory"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	deps := router.Deps{Config: cfg}

	if err := cfg.RequireAuth0(); err != nil {
		log.Printf("auth: disabled (%v) — only /healthz is mounted", err)
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		authClient, err := auth.NewClient(ctx, cfg)
		cancel()
		if err != nil {
			log.Fatalf("auth: %v", err)
		}
		deps.Auth = authClient

		// Phase 3 replaces this with a Redis-backed store; see
		// internal/store/memory's doc comment for why it can't stay this
		// way past local dev / this phase's manual testing.
		deps.Store = memory.New()
		log.Printf("auth: enabled against %s (session store: in-memory, Phase 3 replaces this with Redis)", cfg.Auth0Domain)
	}

	handler := router.New(deps)

	addr := ":" + cfg.Port
	log.Printf("tdff-bff listening on %s (env=%s)", addr, cfg.AppEnv)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("server: %v", err)
	}
}

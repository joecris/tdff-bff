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
	redisstore "github.com/joecris/tdff-bff/internal/store/redis"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	deps := router.Deps{Config: cfg}

	switch {
	case cfg.RequireAuth0() != nil:
		log.Printf("auth: disabled (%v) — only /healthz is mounted", cfg.RequireAuth0())
	case cfg.RequireRedis() != nil:
		log.Printf("auth: disabled (%v) — only /healthz is mounted", cfg.RequireRedis())
	default:
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		authClient, err := auth.NewClient(ctx, cfg)
		cancel()
		if err != nil {
			log.Fatalf("auth: %v", err)
		}

		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		store, err := redisstore.New(ctx, cfg.RedisURL)
		cancel()
		if err != nil {
			log.Fatalf("redis: %v", err)
		}

		deps.Auth = authClient
		deps.Store = store
		log.Printf("auth: enabled against %s (session store: redis)", cfg.Auth0Domain)
	}

	handler := router.New(deps)

	addr := ":" + cfg.Port
	log.Printf("tdff-bff listening on %s (env=%s)", addr, cfg.AppEnv)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("server: %v", err)
	}
}

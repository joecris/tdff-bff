//go:build integration

// This file only builds with `go test -tags=integration`. It's the real-
// Redis counterpart to redis_test.go's miniredis-backed tests — same
// assertions, but against an actual redis-server, to catch anything a
// from-scratch RESP reimplementation like miniredis might not: real
// timing/expiry behavior, real TLS/auth handshakes if REDIS_URL carries
// them, real server responses. CI runs this against a redis:7-alpine
// service container (see .github/workflows/ci.yml's `integration` job);
// locally, point REDIS_URL at `make docker-up`'s Redis (or any reachable
// one) and run `make test-integration`.

package redis_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/joecris/tdff-bff/internal/session"
	redisstore "github.com/joecris/tdff-bff/internal/store/redis"
)

func realRedisURL() string {
	if v := os.Getenv("REDIS_URL"); v != "" {
		return v
	}
	return "redis://localhost:6379"
}

func newRealTestStore(t *testing.T) *redisstore.Store {
	t.Helper()
	s, err := redisstore.New(context.Background(), realRedisURL())
	if err != nil {
		t.Fatalf("redis.New against %s: %v (is a real Redis reachable? see this file's doc comment)", realRedisURL(), err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestIntegrationSaveGetDelete(t *testing.T) {
	s := newRealTestStore(t)
	ctx := context.Background()
	// Unique-ish key so a run against a shared/persistent Redis (e.g. local
	// docker-compose) doesn't collide with leftovers from a prior run.
	id := "integration-test-" + time.Now().Format(time.RFC3339Nano)
	data := session.Data{Subject: "user-1", Email: "user@example.com", AccessToken: "at", RefreshToken: "rt"}

	if err := s.Save(ctx, id, data, time.Minute); err != nil {
		t.Fatalf("Save: %v", err)
	}
	t.Cleanup(func() { _ = s.Delete(context.Background(), id) })

	got, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != data {
		t.Errorf("expected %+v, got %+v", data, got)
	}

	if err := s.Delete(ctx, id); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Get(ctx, id); err != session.ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestIntegrationExpiryIsEnforcedByRealServer(t *testing.T) {
	s := newRealTestStore(t)
	ctx := context.Background()
	id := "integration-test-ttl-" + time.Now().Format(time.RFC3339Nano)

	if err := s.Save(ctx, id, session.Data{}, 500*time.Millisecond); err != nil {
		t.Fatalf("Save: %v", err)
	}
	t.Cleanup(func() { _ = s.Delete(context.Background(), id) })

	time.Sleep(1500 * time.Millisecond) // real wall-clock wait — no virtual clock to fast-forward here

	if _, err := s.Get(ctx, id); err != session.ErrNotFound {
		t.Errorf("expected ErrNotFound for an expired entry, got %v", err)
	}
}

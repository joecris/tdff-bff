// Package redis implements session.Store on Redis. Standard go-redis over
// TCP+TLS is used against Upstash in non-prod/prod and against the plain
// redis container in local dev — the Go Framework Preset runs a real
// long-lived server process (not the Edge Runtime), so there's no need for
// Upstash's REST API to work around short-lived-invocation connection
// overhead; see the plan's "Redis" decision for the full reasoning.
package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/joecris/tdff-bff/internal/session"
)

const keyPrefix = "tdff-bff:session:"

// Store is a Redis-backed session.Store.
type Store struct {
	client *redis.Client
}

// New parses redisURL (redis:// or rediss://, e.g. Upstash's TLS endpoint),
// connects, and verifies reachability with a PING before returning — same
// fail-fast-at-startup posture as auth.NewClient's OIDC discovery call.
func New(ctx context.Context, redisURL string) (*Store, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("redis: parse REDIS_URL: %w", err)
	}

	client := redis.NewClient(opts)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		return nil, fmt.Errorf("redis: ping %s: %w", opts.Addr, err)
	}

	return &Store{client: client}, nil
}

// Close releases the underlying connection pool.
func (s *Store) Close() error {
	return s.client.Close()
}

func (s *Store) Save(ctx context.Context, id string, data session.Data, ttl time.Duration) error {
	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("redis: marshal session data: %w", err)
	}
	if err := s.client.Set(ctx, keyPrefix+id, b, ttl).Err(); err != nil {
		return fmt.Errorf("redis: save session: %w", err)
	}
	return nil
}

func (s *Store) Get(ctx context.Context, id string) (session.Data, error) {
	b, err := s.client.Get(ctx, keyPrefix+id).Bytes()
	if err != nil {
		if err == redis.Nil {
			return session.Data{}, session.ErrNotFound
		}
		return session.Data{}, fmt.Errorf("redis: get session: %w", err)
	}

	var data session.Data
	if err := json.Unmarshal(b, &data); err != nil {
		return session.Data{}, fmt.Errorf("redis: unmarshal session data: %w", err)
	}
	return data, nil
}

func (s *Store) Delete(ctx context.Context, id string) error {
	if err := s.client.Del(ctx, keyPrefix+id).Err(); err != nil {
		return fmt.Errorf("redis: delete session: %w", err)
	}
	return nil
}

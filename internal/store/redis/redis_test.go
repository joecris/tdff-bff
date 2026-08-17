package redis_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"

	"github.com/joecris/tdff-bff/internal/session"
	redisstore "github.com/joecris/tdff-bff/internal/store/redis"
)

// newTestStore spins up an in-process fake Redis server (miniredis) so
// these tests don't depend on Docker/a real Redis being up — CI can run
// them the same as any other unit test. It returns the miniredis handle too,
// since TTL tests need to advance its virtual clock (FastForward) rather
// than relying on wall-clock sleeps.
func newTestStore(t *testing.T) (*redisstore.Store, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)

	s, err := redisstore.New(context.Background(), "redis://"+mr.Addr())
	if err != nil {
		t.Fatalf("redis.New: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, mr
}

func TestSaveGetDelete(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()
	data := session.Data{Subject: "user-1", Email: "user@example.com", AccessToken: "at", RefreshToken: "rt"}

	if err := s.Save(ctx, "id-1", data, time.Minute); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := s.Get(ctx, "id-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != data {
		t.Errorf("expected %+v, got %+v", data, got)
	}

	if err := s.Delete(ctx, "id-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Get(ctx, "id-1"); err != session.ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestGetUnknown(t *testing.T) {
	s, _ := newTestStore(t)
	if _, err := s.Get(context.Background(), "missing"); err != session.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestGetExpired(t *testing.T) {
	s, mr := newTestStore(t)
	ctx := context.Background()

	if err := s.Save(ctx, "id-1", session.Data{}, time.Minute); err != nil {
		t.Fatalf("Save: %v", err)
	}
	mr.FastForward(2 * time.Minute)

	if _, err := s.Get(ctx, "id-1"); err != session.ErrNotFound {
		t.Errorf("expected ErrNotFound for expired entry, got %v", err)
	}
}

func TestNewInvalidURL(t *testing.T) {
	if _, err := redisstore.New(context.Background(), "not-a-url"); err == nil {
		t.Fatal("expected an error for an invalid REDIS_URL")
	}
}

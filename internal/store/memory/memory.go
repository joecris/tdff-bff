// Package memory implements session.Store in-process. It was the Phase 2
// stand-in before Redis landed (internal/store/redis); main.go no longer
// wires it up. It stays around as a dependency-free fake for tests that
// exercise session.Store-consuming code (handlers, etc.) without needing a
// real or fake Redis. Not suitable for running the server: state is lost on
// restart and isn't shared across instances, which breaks the moment
// there's more than one BFF process running (Vercel does not guarantee a
// single instance).
package memory

import (
	"context"
	"sync"
	"time"

	"github.com/joecris/tdff-bff/internal/session"
)

type entry struct {
	data      session.Data
	expiresAt time.Time
}

// Store is a goroutine-safe, in-memory session.Store.
type Store struct {
	mu   sync.Mutex
	data map[string]entry
}

// New returns an empty Store.
func New() *Store {
	return &Store{data: make(map[string]entry)}
}

func (s *Store) Save(_ context.Context, id string, data session.Data, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[id] = entry{data: data, expiresAt: time.Now().Add(ttl)}
	return nil
}

func (s *Store) Get(_ context.Context, id string) (session.Data, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.data[id]
	if !ok || time.Now().After(e.expiresAt) {
		delete(s.data, id)
		return session.Data{}, session.ErrNotFound
	}
	return e.data, nil
}

func (s *Store) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, id)
	return nil
}

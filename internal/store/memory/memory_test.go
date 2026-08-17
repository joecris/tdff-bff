package memory_test

import (
	"context"
	"testing"
	"time"

	"github.com/joecris/tdff-bff/internal/session"
	"github.com/joecris/tdff-bff/internal/store/memory"
)

func TestSaveGetDelete(t *testing.T) {
	s := memory.New()
	ctx := context.Background()
	data := session.Data{Subject: "user-1", Email: "user@example.com"}

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

func TestGetExpired(t *testing.T) {
	s := memory.New()
	ctx := context.Background()

	if err := s.Save(ctx, "id-1", session.Data{}, -time.Second); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := s.Get(ctx, "id-1"); err != session.ErrNotFound {
		t.Errorf("expected ErrNotFound for expired entry, got %v", err)
	}
}

func TestGetUnknown(t *testing.T) {
	s := memory.New()
	if _, err := s.Get(context.Background(), "missing"); err != session.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

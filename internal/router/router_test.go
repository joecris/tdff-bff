package router_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/joecris/tdff-bff/internal/config"
	"github.com/joecris/tdff-bff/internal/router"
)

func TestHealthz(t *testing.T) {
	cfg := &config.Config{AppEnv: "test"}
	h := router.New(cfg)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body struct {
		Status string `json:"status"`
		Env    string `json:"env"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Status != "ok" {
		t.Errorf("expected status %q, got %q", "ok", body.Status)
	}
	if body.Env != "test" {
		t.Errorf("expected env %q, got %q", "test", body.Env)
	}
}

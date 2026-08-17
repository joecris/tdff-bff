package proxy_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/oauth2"

	"github.com/joecris/tdff-bff/internal/config"
	"github.com/joecris/tdff-bff/internal/proxy"
	"github.com/joecris/tdff-bff/internal/session"
	"github.com/joecris/tdff-bff/internal/store/memory"
)

type fakeRefresher struct {
	called bool
	token  *oauth2.Token
	err    error
}

func (f *fakeRefresher) Refresh(_ context.Context, _ string) (*oauth2.Token, error) {
	f.called = true
	return f.token, f.err
}

func testConfig(backendURL string) *config.Config {
	return &config.Config{
		SessionCookieName: "__Host-test-session",
		SessionTTLSeconds: 3600,
		BackendAPIBaseURL: backendURL,
	}
}

func withSessionCookie(t *testing.T, req *http.Request, cfg *config.Config, sessionID string) {
	t.Helper()
	rec := httptest.NewRecorder()
	session.SetSessionCookie(rec, sessionID, cfg)
	for _, c := range rec.Result().Cookies() {
		req.AddCookie(c)
	}
}

func TestNoSessionCookieIsUnauthorized(t *testing.T) {
	cfg := testConfig("http://unused.invalid")
	h, err := proxy.Handler(cfg, &fakeRefresher{}, memory.New())
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/things", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestUnknownSessionIsUnauthorized(t *testing.T) {
	cfg := testConfig("http://unused.invalid")
	h, err := proxy.Handler(cfg, &fakeRefresher{}, memory.New())
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/things", nil)
	withSessionCookie(t, req, cfg, "does-not-exist")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestDisallowedMethod(t *testing.T) {
	cfg := testConfig("http://unused.invalid")
	h, err := proxy.Handler(cfg, &fakeRefresher{}, memory.New())
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodTrace, "/things", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

func TestForwardsWithFreshAuthHeaderAndStripsClientHeaders(t *testing.T) {
	var gotAuth, gotCookie, gotPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotCookie = r.Header.Get("Cookie")
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("upstream response"))
	}))
	defer upstream.Close()

	cfg := testConfig(upstream.URL)
	store := memory.New()
	sessionID := "sess-1"
	if err := store.Save(context.Background(), sessionID, session.Data{
		AccessToken: "valid-access-token",
		ExpiresAt:   time.Now().Add(time.Hour),
	}, time.Hour); err != nil {
		t.Fatalf("Save: %v", err)
	}

	h, err := proxy.Handler(cfg, &fakeRefresher{}, store)
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/things/42", nil)
	req.Header.Set("Authorization", "Bearer client-supplied-token")
	req.Header.Set("Cookie", "some=leftover")
	withSessionCookie(t, req, cfg, sessionID)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusTeapot {
		t.Errorf("expected upstream's status 418, got %d", rec.Code)
	}
	body, _ := io.ReadAll(rec.Body)
	if string(body) != "upstream response" {
		t.Errorf("expected proxied body, got %q", body)
	}
	if gotAuth != "Bearer valid-access-token" {
		t.Errorf("expected upstream to see the session's access token, got %q", gotAuth)
	}
	if gotCookie != "" {
		t.Errorf("expected the client's Cookie header to be stripped, got %q", gotCookie)
	}
	if gotPath != "/things/42" {
		t.Errorf("expected path /things/42, got %q", gotPath)
	}
}

func TestRefreshesNearExpiryTokenBeforeForwarding(t *testing.T) {
	var gotAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	cfg := testConfig(upstream.URL)
	store := memory.New()
	sessionID := "sess-1"
	if err := store.Save(context.Background(), sessionID, session.Data{
		AccessToken:  "stale-access-token",
		RefreshToken: "refresh-token",
		ExpiresAt:    time.Now().Add(-time.Minute), // already expired
	}, time.Hour); err != nil {
		t.Fatalf("Save: %v", err)
	}

	refresher := &fakeRefresher{token: &oauth2.Token{
		AccessToken:  "new-access-token",
		RefreshToken: "new-refresh-token",
		Expiry:       time.Now().Add(time.Hour),
	}}

	h, err := proxy.Handler(cfg, refresher, store)
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/things", nil)
	withSessionCookie(t, req, cfg, sessionID)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if !refresher.called {
		t.Fatal("expected Refresh to be called for a near-expiry token")
	}
	if gotAuth != "Bearer new-access-token" {
		t.Errorf("expected upstream to see the refreshed token, got %q", gotAuth)
	}

	updated, err := store.Get(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if updated.AccessToken != "new-access-token" || updated.RefreshToken != "new-refresh-token" {
		t.Errorf("expected session to be updated with refreshed tokens, got %+v", updated)
	}
}

func TestFailedRefreshInvalidatesSession(t *testing.T) {
	cfg := testConfig("http://unused.invalid")
	store := memory.New()
	sessionID := "sess-1"
	if err := store.Save(context.Background(), sessionID, session.Data{
		AccessToken:  "stale-access-token",
		RefreshToken: "bad-refresh-token",
		ExpiresAt:    time.Now().Add(-time.Minute),
	}, time.Hour); err != nil {
		t.Fatalf("Save: %v", err)
	}

	refresher := &fakeRefresher{err: context.DeadlineExceeded}
	h, err := proxy.Handler(cfg, refresher, store)
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/things", nil)
	withSessionCookie(t, req, cfg, sessionID)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
	if _, err := store.Get(context.Background(), sessionID); err != session.ErrNotFound {
		t.Errorf("expected session to be deleted, got err=%v", err)
	}
}

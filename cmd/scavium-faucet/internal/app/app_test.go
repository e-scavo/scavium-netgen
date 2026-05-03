package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"scavium-netgen/cmd/scavium-faucet/internal/config"
	"scavium-netgen/cmd/scavium-faucet/internal/faucet"
)

func TestCloseCancelsRuntimeContext(t *testing.T) {
	application := newTestApp(t, testConfig(t))

	if err := application.Close(context.Background()); err != nil {
		t.Fatalf("close: %v", err)
	}

	select {
	case <-application.ctx.Done():
	default:
		t.Fatal("runtime context was not cancelled")
	}
}

func TestCloseRunsRegisteredClosers(t *testing.T) {
	application := newTestApp(t, testConfig(t))
	called := false
	application.closeFuncs = append(application.closeFuncs, func(context.Context) error {
		called = true
		return nil
	})

	if err := application.Close(context.Background()); err != nil {
		t.Fatalf("close: %v", err)
	}
	if !called {
		t.Fatal("registered closer was not called")
	}
}

func TestNewUsesPersistentStore(t *testing.T) {
	cfg := testConfig(t)
	application := newTestApp(t, cfg)
	defer application.Close(context.Background()) //nolint:errcheck

	created := createClaim(t, application, "0x52908400098527886E0F7030069857D2E4169EE7")

	reopened := newTestApp(t, cfg)
	defer reopened.Close(context.Background()) //nolint:errcheck

	req := httptest.NewRequest(http.MethodGet, "/api/v1/claim/"+created.ID, nil)
	rec := httptest.NewRecorder()
	reopened.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var got faucet.ClaimResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode claim: %v", err)
	}
	if got.ID != created.ID {
		t.Fatalf("got id = %q, want %q", got.ID, created.ID)
	}
}

func TestPublicClaimSurvivesAppCloseReopen(t *testing.T) {
	cfg := testConfig(t)
	application := newTestApp(t, cfg)
	created := createClaim(t, application, "0x52908400098527886E0F7030069857D2E4169EE7")
	if err := application.Close(context.Background()); err != nil {
		t.Fatalf("close first app: %v", err)
	}

	reopened := newTestApp(t, cfg)
	defer reopened.Close(context.Background()) //nolint:errcheck

	req := httptest.NewRequest(http.MethodGet, "/api/v1/claim/"+created.ID, nil)
	rec := httptest.NewRecorder()
	reopened.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminTokenIsPassedToHandler(t *testing.T) {
	cfg := testConfig(t)
	cfg.AdminToken = "test-admin-token"
	application := newTestApp(t, cfg)
	defer application.Close(context.Background()) //nolint:errcheck

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/dashboard", nil)
	req.Header.Set("Authorization", "Bearer "+cfg.AdminToken)
	rec := httptest.NewRecorder()
	application.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
}

func TestDryRunStartsWithoutRPCOrPrivateKey(t *testing.T) {
	cfg := testConfig(t)
	cfg.DryRun = true
	cfg.RPCURL = "http://127.0.0.1:1"
	cfg.PrivateKeyHex = ""
	cfg.WorkerEnabled = true
	cfg.WorkerPollSeconds = 1

	application := newTestApp(t, cfg)
	if err := application.Close(context.Background()); err != nil {
		t.Fatalf("close app: %v", err)
	}
}

func newTestApp(t *testing.T, cfg config.Config) *App {
	t.Helper()
	application, err := New(cfg)
	if err != nil {
		t.Fatalf("new app: %v", err)
	}
	return application
}

func testConfig(t *testing.T) config.Config {
	t.Helper()
	cfg := config.Defaults()
	cfg.DatabasePath = filepath.Join(t.TempDir(), "faucet.db")
	cfg.WorkerEnabled = false
	cfg.WatcherEnabled = false
	cfg.CooldownSeconds = 0
	return cfg
}

func createClaim(t *testing.T, application *App, address string) faucet.ClaimResponse {
	t.Helper()
	body := []byte(`{"address":"` + address + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/claim", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	application.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202: %s", rec.Code, rec.Body.String())
	}

	var created faucet.ClaimResponse
	if err := json.NewDecoder(rec.Body).Decode(&created); err != nil {
		t.Fatalf("decode claim: %v", err)
	}
	if created.ID == "" {
		t.Fatal("created claim id is empty")
	}
	return created
}

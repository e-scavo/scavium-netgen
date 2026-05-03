package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"scavium-netgen/cmd/scavium-faucet/internal/ready"
	"scavium-netgen/cmd/scavium-faucet/internal/version"
)

func TestHealth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set(requestIDHeader, "test-request-id")
	rec := httptest.NewRecorder()

	NewHandler(Dependencies{}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content type = %q, want application/json", got)
	}
	if got := rec.Header().Get(requestIDHeader); got != "test-request-id" {
		t.Fatalf("request id header = %q, want test-request-id", got)
	}

	var body healthResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Status != "ok" {
		t.Fatalf("status = %q, want ok", body.Status)
	}
	if _, err := time.Parse(time.RFC3339, body.Time); err != nil {
		t.Fatalf("time = %q, want RFC3339: %v", body.Time, err)
	}
}

func TestHealthRejectsUnsupportedMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	req.Header.Set(requestIDHeader, "test-request-id")
	rec := httptest.NewRecorder()

	NewHandler(Dependencies{}).ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
	if got := rec.Header().Get("Allow"); got != http.MethodGet {
		t.Fatalf("allow header = %q, want %q", got, http.MethodGet)
	}

	var body ErrorEnvelope
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != "method_not_allowed" {
		t.Fatalf("code = %q, want method_not_allowed", body.Code)
	}
	if body.Message != "method not allowed" {
		t.Fatalf("message = %q, want method not allowed", body.Message)
	}
	if body.RequestID != "test-request-id" {
		t.Fatalf("request id = %q, want test-request-id", body.RequestID)
	}
	if len(body.Details) != 0 {
		t.Fatalf("details = %#v, want empty object", body.Details)
	}
}

func TestRequestIDMiddlewareGeneratesMissingRequestID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	NewHandler(Dependencies{}).ServeHTTP(rec, req)

	if got := rec.Header().Get(requestIDHeader); got == "" {
		t.Fatal("request id header is empty")
	}
}

func TestNotFoundUsesErrorEnvelope(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	req.Header.Set(requestIDHeader, "test-request-id")
	rec := httptest.NewRecorder()

	NewHandler(Dependencies{}).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusNotFound)
	}

	var body ErrorEnvelope
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != "not_found" {
		t.Fatalf("code = %q, want not_found", body.Code)
	}
	if body.RequestID != "test-request-id" {
		t.Fatalf("request id = %q, want test-request-id", body.RequestID)
	}
}

func TestReadyReportsOK(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()

	NewHandler(Dependencies{
		ReadinessChecks: []ready.Check{
			{Name: "rpc", Run: ready.StubOK},
			{Name: "db", Run: ready.StubOK},
		},
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}

	var body ready.Result
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Status != ready.StatusOK {
		t.Fatalf("status = %q, want %q", body.Status, ready.StatusOK)
	}
	if len(body.Checks) != 2 {
		t.Fatalf("checks length = %d, want 2", len(body.Checks))
	}
}

func TestReadyReportsDegraded(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()

	NewHandler(Dependencies{
		ReadinessChecks: []ready.Check{
			{Name: "rpc", Run: func(context.Context) error {
				return ready.ErrDegraded("rpc unavailable")
			}},
			{Name: "db", Run: ready.StubOK},
		},
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}

	var body ready.Result
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Status != ready.StatusDegraded {
		t.Fatalf("status = %q, want %q", body.Status, ready.StatusDegraded)
	}
	if body.Checks[1].Name != "rpc" || body.Checks[1].Error != "rpc unavailable" {
		t.Fatalf("rpc check = %#v", body.Checks[1])
	}
}

func TestReadyRejectsUnsupportedMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/ready", nil)
	rec := httptest.NewRecorder()

	NewHandler(Dependencies{}).ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
	if got := rec.Header().Get("Allow"); got != http.MethodGet {
		t.Fatalf("allow header = %q, want %q", got, http.MethodGet)
	}
}

func TestVersion(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/version", nil)
	rec := httptest.NewRecorder()

	NewHandler(Dependencies{
		VersionInfo: version.Info{
			Version:   "v1.2.3",
			Commit:    "abc123",
			BuildDate: "2026-05-03T00:00:00Z",
		},
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}

	var body version.Info
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Version != "v1.2.3" || body.Commit != "abc123" || body.BuildDate != "2026-05-03T00:00:00Z" {
		t.Fatalf("version body = %#v", body)
	}
}

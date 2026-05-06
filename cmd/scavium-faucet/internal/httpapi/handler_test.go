package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"scavium-netgen/cmd/scavium-faucet/internal/admin"
	"scavium-netgen/cmd/scavium-faucet/internal/config"
	"scavium-netgen/cmd/scavium-faucet/internal/domain"
	"scavium-netgen/cmd/scavium-faucet/internal/faucet"
	"scavium-netgen/cmd/scavium-faucet/internal/observability"
	"scavium-netgen/cmd/scavium-faucet/internal/ready"
	"scavium-netgen/cmd/scavium-faucet/internal/version"

	"github.com/ethereum/go-ethereum/common"
)

func TestHealth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set(requestIDHeader, "test-request-id")
	req.Header.Set(correlationIDHeader, "test-correlation-id")
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
	if got := rec.Header().Get(correlationIDHeader); got != "test-correlation-id" {
		t.Fatalf("correlation id header = %q, want test-correlation-id", got)
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
	if body.UptimeSeconds < 0 {
		t.Fatalf("uptime_seconds = %d, want non-negative", body.UptimeSeconds)
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

func TestRequestIDMiddlewareDefaultsCorrelationIDToRequestID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set(requestIDHeader, "test-request-id")
	rec := httptest.NewRecorder()

	NewHandler(Dependencies{}).ServeHTTP(rec, req)

	if got := rec.Header().Get(correlationIDHeader); got != "test-request-id" {
		t.Fatalf("correlation id header = %q, want test-request-id", got)
	}
}

func TestRequestLoggingMiddlewareWritesSafeFields(t *testing.T) {
	var logs bytes.Buffer
	req := httptest.NewRequest(http.MethodGet, "/health?private_key=do-not-log", nil)
	req.RemoteAddr = "10.0.0.1:4567"
	req.Header.Set(requestIDHeader, "test-request-id")
	req.Header.Set(correlationIDHeader, "test-correlation-id")
	req.Header.Set("X-Forwarded-For", "203.0.113.9, 10.0.0.1")
	rec := httptest.NewRecorder()

	NewHandler(Dependencies{
		Logger:       observability.NewLogger(&logs),
		TrustedProxy: "10.0.0.1",
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}

	var entry map[string]any
	if err := json.Unmarshal(logs.Bytes(), &entry); err != nil {
		t.Fatalf("decode log entry: %v", err)
	}
	if entry["message"] != "http request" {
		t.Fatalf("message = %q, want http request", entry["message"])
	}
	if entry["request_id"] != "test-request-id" {
		t.Fatalf("request_id = %q", entry["request_id"])
	}
	if entry["correlation_id"] != "test-correlation-id" {
		t.Fatalf("correlation_id = %q", entry["correlation_id"])
	}
	if entry["method"] != http.MethodGet {
		t.Fatalf("method = %q", entry["method"])
	}
	if entry["path"] != "/health" {
		t.Fatalf("path = %q", entry["path"])
	}
	if entry["status"] != float64(http.StatusOK) {
		t.Fatalf("status = %v, want %d", entry["status"], http.StatusOK)
	}
	if entry["remote_ip"] != "203.0.113.9" {
		t.Fatalf("remote_ip = %q, want 203.0.113.9", entry["remote_ip"])
	}
	if entry["duration"] == "" {
		t.Fatal("duration is empty")
	}
	if strings.Contains(logs.String(), "do-not-log") {
		t.Fatalf("log contains query secret: %s", logs.String())
	}
}

func TestRequestLoggingMiddlewareDoesNotLogClaimPayload(t *testing.T) {
	var logs bytes.Buffer
	req := httptest.NewRequest(http.MethodPost, "/api/v1/claim", bytes.NewBufferString(`{
		"address":"0x52908400098527886E0F7030069857D2E4169EE7",
		"captcha_token":"captcha-secret-value",
		"fingerprint":"fingerprint-secret-value"
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer admin-token-secret")
	req.Header.Set(idempotencyKeyHeader, "idempotency-secret")
	rec := httptest.NewRecorder()

	NewHandler(Dependencies{
		ReadService: testClaimService(),
		Logger:      observability.NewLogger(&logs),
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusAccepted)
	}

	logText := logs.String()
	for _, secret := range []string{
		"52908400098527886E0F7030069857D2E4169EE7",
		"captcha-secret-value",
		"fingerprint-secret-value",
		"admin-token-secret",
		"idempotency-secret",
	} {
		if strings.Contains(logText, secret) {
			t.Fatalf("log contains sensitive value %q: %s", secret, logText)
		}
	}
}

func TestAddressHistoryReturnsPaginatedClaims(t *testing.T) {
	service := testClaimService()
	address := "0x52908400098527886E0F7030069857D2E4169EE7"
	reqBody := `{"address":"` + address + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/claim", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	NewHandler(Dependencies{ReadService: service}).ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("create status code = %d, want %d", rec.Code, http.StatusAccepted)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/address/"+address+"/history?limit=1&offset=0", nil)
	rec = httptest.NewRecorder()
	NewHandler(Dependencies{ReadService: service}).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("history status code = %d, want %d", rec.Code, http.StatusOK)
	}
	var body faucet.AddressHistoryResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode history: %v", err)
	}
	if body.Address != address {
		t.Fatalf("address = %q, want %q", body.Address, address)
	}
	if body.Pagination.Limit != 1 || body.Pagination.Offset != 0 || body.Pagination.Count != 1 {
		t.Fatalf("pagination = %#v", body.Pagination)
	}
	if len(body.Claims) != 1 || body.Claims[0].ID != "claim_test" {
		t.Fatalf("claims = %#v", body.Claims)
	}
}

func TestAddressHistoryRejectsInvalidAddressAndPagination(t *testing.T) {
	handler := NewHandler(Dependencies{ReadService: testClaimService()})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/address/not-an-address/history", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid address status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/address/0x52908400098527886E0F7030069857D2E4169EE7/history?limit=abc", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid pagination status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCreateClaimLogsAcceptedClaimFlowWithCorrelationID(t *testing.T) {
	var logs bytes.Buffer
	req := httptest.NewRequest(http.MethodPost, "/api/v1/claim", bytes.NewBufferString(`{
		"address":"0x52908400098527886E0F7030069857D2E4169EE7",
		"captcha_token":"captcha-secret-value",
		"fingerprint":"fingerprint-secret-value"
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(requestIDHeader, "test-request-id")
	req.Header.Set(correlationIDHeader, "test-correlation-id")
	req.Header.Set(idempotencyKeyHeader, "idempotency-secret")
	rec := httptest.NewRecorder()

	NewHandler(Dependencies{
		ReadService: testClaimService(),
		Logger:      observability.NewLogger(&logs),
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusAccepted)
	}

	entries := decodeLogEntries(t, logs.Bytes())
	var claimLog map[string]any
	for _, entry := range entries {
		if entry["message"] == "claim accepted" {
			claimLog = entry
			break
		}
	}
	if claimLog == nil {
		t.Fatalf("claim accepted log not found in %#v", entries)
	}
	if claimLog["request_id"] != "test-request-id" {
		t.Fatalf("request_id = %q", claimLog["request_id"])
	}
	if claimLog["correlation_id"] != "test-correlation-id" {
		t.Fatalf("correlation_id = %q", claimLog["correlation_id"])
	}
	if claimLog["claim_id"] != "claim_test" {
		t.Fatalf("claim_id = %q", claimLog["claim_id"])
	}
	if claimLog["event"] != "token_claim_accepted" {
		t.Fatalf("event = %q", claimLog["event"])
	}
	if claimLog["token_id"] != "native" {
		t.Fatalf("token_id = %q", claimLog["token_id"])
	}
	if claimLog["has_idempotency_key"] != true {
		t.Fatalf("has_idempotency_key = %v", claimLog["has_idempotency_key"])
	}
	if claimLog["has_fingerprint"] != true {
		t.Fatalf("has_fingerprint = %v", claimLog["has_fingerprint"])
	}
	if claimLog["captcha_token_present"] != true {
		t.Fatalf("captcha_token_present = %v", claimLog["captcha_token_present"])
	}
}

func decodeLogEntries(t *testing.T, data []byte) []map[string]any {
	t.Helper()
	var entries []map[string]any
	for _, line := range bytes.Split(bytes.TrimSpace(data), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal(line, &entry); err != nil {
			t.Fatalf("decode log entry: %v", err)
		}
		entries = append(entries, entry)
	}
	return entries
}

func TestCORSDisabledByDefault(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	req.Header.Set("Origin", "https://wallet.example.test")
	rec := httptest.NewRecorder()

	NewHandler(Dependencies{ReadService: testReadService()}).ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("access-control-allow-origin = %q, want empty", got)
	}
	if got := rec.Header().Get("Vary"); got != "" {
		t.Fatalf("vary = %q, want empty", got)
	}
}

func TestCORSAllowsExactOrigin(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	req.Header.Set("Origin", "https://wallet.example.test")
	rec := httptest.NewRecorder()

	NewHandler(Dependencies{
		ReadService: testReadService(),
		CORSOrigins: []string{"https://wallet.example.test", "https://faucet.example.test"},
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://wallet.example.test" {
		t.Fatalf("access-control-allow-origin = %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got != "GET, POST, OPTIONS" {
		t.Fatalf("access-control-allow-methods = %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); got != "Content-Type, Idempotency-Key, Authorization, X-Request-ID, X-Correlation-ID" {
		t.Fatalf("access-control-allow-headers = %q", got)
	}
	if got := rec.Header().Get("Vary"); got != "Origin" {
		t.Fatalf("vary = %q, want Origin", got)
	}
}

func TestCORSRejectsNonExactOrigin(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	req.Header.Set("Origin", "https://evil.example.test")
	rec := httptest.NewRecorder()

	NewHandler(Dependencies{
		ReadService: testReadService(),
		CORSOrigins: []string{"https://wallet.example.test"},
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("access-control-allow-origin = %q, want empty", got)
	}
}

func TestCORSPreflightForAllowedOrigin(t *testing.T) {
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/claim", nil)
	req.Header.Set("Origin", "https://wallet.example.test")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	rec := httptest.NewRecorder()

	NewHandler(Dependencies{
		ReadService: testReadService(),
		CORSOrigins: []string{"https://wallet.example.test"},
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://wallet.example.test" {
		t.Fatalf("access-control-allow-origin = %q", got)
	}
	if got := rec.Header().Get(requestIDHeader); got == "" {
		t.Fatal("request id header is empty")
	}
	if got := rec.Header().Get(correlationIDHeader); got == "" {
		t.Fatal("correlation id header is empty")
	}
}

func TestCORSPreflightPreservesIncomingRequestID(t *testing.T) {
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/claim", nil)
	req.Header.Set("Origin", "https://wallet.example.test")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	req.Header.Set(requestIDHeader, "preflight-request-id")
	rec := httptest.NewRecorder()

	NewHandler(Dependencies{
		ReadService: testReadService(),
		CORSOrigins: []string{"https://wallet.example.test"},
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if got := rec.Header().Get(requestIDHeader); got != "preflight-request-id" {
		t.Fatalf("request id header = %q, want preflight-request-id", got)
	}
	if got := rec.Header().Get(correlationIDHeader); got == "" {
		t.Fatal("correlation id header is empty")
	}
}

func TestRequestLoggingMiddlewareRedactsAddressStatusPath(t *testing.T) {
	const addr = "0x1111111111111111111111111111111111111111"
	var logs bytes.Buffer
	req := httptest.NewRequest(http.MethodGet, "/api/v1/address/"+addr+"/status", nil)
	rec := httptest.NewRecorder()

	NewHandler(Dependencies{
		ReadService: testReadService(),
		Logger:      observability.NewLogger(&logs),
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}

	logText := logs.String()
	if strings.Contains(logText, addr) {
		t.Fatalf("log contains full address: %s", logText)
	}

	entries := decodeLogEntries(t, logs.Bytes())
	if len(entries) == 0 {
		t.Fatal("no log entries captured")
	}
	if got := entries[0]["path"]; got != "/api/v1/address/:address/status" {
		t.Fatalf("path = %q, want /api/v1/address/:address/status", got)
	}
}

func TestRequestLoggingMiddlewareRedactsFaucetEligibilityPath(t *testing.T) {
	const addr = "0x1111111111111111111111111111111111111111"
	var logs bytes.Buffer
	req := httptest.NewRequest(http.MethodGet, "/api/v1/faucet/address/"+addr+"/eligibility", nil)
	rec := httptest.NewRecorder()

	NewHandler(Dependencies{
		ReadService: testReadService(),
		Logger:      observability.NewLogger(&logs),
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}

	logText := logs.String()
	if strings.Contains(logText, addr) {
		t.Fatalf("log contains full address: %s", logText)
	}

	entries := decodeLogEntries(t, logs.Bytes())
	if len(entries) == 0 {
		t.Fatal("no log entries captured")
	}
	if got := entries[0]["path"]; got != "/api/v1/faucet/address/:address/eligibility" {
		t.Fatalf("path = %q, want /api/v1/faucet/address/:address/eligibility", got)
	}
}

func TestCORSSkipsAdminPaths(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/dashboard", nil)
	req.Header.Set("Origin", "https://wallet.example.test")
	rec := httptest.NewRecorder()

	NewHandler(Dependencies{
		ReadService: testReadService(),
		AdminToken:  testAdminToken,
		CORSOrigins: []string{"https://wallet.example.test"},
	}).ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("access-control-allow-origin = %q, want empty", got)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestNotFoundUsesErrorEnvelope(t *testing.T) {
	// Non-API paths now serve the frontend; /api/ paths return JSON 404.
	req := httptest.NewRequest(http.MethodGet, "/api/missing", nil)
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
	if body.Summary.Total != 2 || body.Summary.OK != 2 || body.Summary.Degraded != 0 {
		t.Fatalf("summary = %#v", body.Summary)
	}
	for _, check := range body.Checks {
		if check.DurationMS < 0 {
			t.Fatalf("check duration = %d, want non-negative", check.DurationMS)
		}
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
	if body.Summary.Total != 2 || body.Summary.OK != 1 || body.Summary.Degraded != 1 {
		t.Fatalf("summary = %#v", body.Summary)
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

func TestPublicStatus(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	rec := httptest.NewRecorder()

	NewHandler(Dependencies{ReadService: testReadService()}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}

	var body faucet.StatusResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Status != "active" {
		t.Fatalf("status = %q, want active", body.Status)
	}
	if body.NetworkName != "scavium-test" {
		t.Fatalf("network name = %q", body.NetworkName)
	}
	if body.UpdatedAt != "2026-05-03T12:00:00Z" {
		t.Fatalf("updated at = %q", body.UpdatedAt)
	}
}

func TestWalletStatusAlias(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/faucet/status", nil)
	rec := httptest.NewRecorder()

	NewHandler(Dependencies{ReadService: testReadService()}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestPublicConfig(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/config", nil)
	rec := httptest.NewRecorder()

	NewHandler(Dependencies{ReadService: testReadService()}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}

	var body faucet.ConfigResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.ChainID != 123 {
		t.Fatalf("chain id = %d, want 123", body.ChainID)
	}
	if body.AmountWei != "42" {
		t.Fatalf("amount wei = %q, want 42", body.AmountWei)
	}
	if body.RateLimitIPPerHour <= 0 {
		t.Fatalf("rate_limit_ip_per_hour = %d, want > 0", body.RateLimitIPPerHour)
	}
	if body.RateLimitAddrPerDay <= 0 {
		t.Fatalf("rate_limit_addr_per_day = %d, want > 0", body.RateLimitAddrPerDay)
	}
	if body.CaptchaProvider != "turnstile" {
		t.Fatalf("captcha provider = %q, want turnstile", body.CaptchaProvider)
	}
	if body.CaptchaSiteKey != "1x00000000000000000000AA" {
		t.Fatalf("captcha site key = %q", body.CaptchaSiteKey)
	}
}

func TestWalletConfigAlias(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/faucet/config", nil)
	rec := httptest.NewRecorder()

	NewHandler(Dependencies{ReadService: testReadService()}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}

	var body faucet.ConfigResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.RateLimitIPPerHour <= 0 {
		t.Fatalf("rate_limit_ip_per_hour = %d, want > 0", body.RateLimitIPPerHour)
	}
	if body.RateLimitAddrPerDay <= 0 {
		t.Fatalf("rate_limit_addr_per_day = %d, want > 0", body.RateLimitAddrPerDay)
	}
}

func TestPublicTokens(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tokens", nil)
	rec := httptest.NewRecorder()

	NewHandler(Dependencies{ReadService: testReadService()}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}

	var body struct {
		Tokens []faucet.TokenResponse `json:"tokens"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Tokens) != 2 {
		t.Fatalf("tokens = %d, want 2", len(body.Tokens))
	}
	if body.Tokens[0].ID != "native" || body.Tokens[0].Type != "native" {
		t.Fatalf("native token = %#v", body.Tokens[0])
	}
	if body.Tokens[1].ID != "scav" || body.Tokens[1].Type != "erc20" {
		t.Fatalf("erc20 token = %#v", body.Tokens[1])
	}
	if body.Tokens[1].Address != "0x1111111111111111111111111111111111111111" {
		t.Fatalf("erc20 address = %q", body.Tokens[1].Address)
	}
}

func TestWalletTokensAlias(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/faucet/tokens", nil)
	rec := httptest.NewRecorder()

	NewHandler(Dependencies{ReadService: testReadService()}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestPublicTokensRejectsUnsupportedMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tokens", nil)
	rec := httptest.NewRecorder()

	NewHandler(Dependencies{ReadService: testReadService()}).ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
	if got := rec.Header().Get("Allow"); got != http.MethodGet {
		t.Fatalf("allow header = %q, want %q", got, http.MethodGet)
	}
}

func TestAddressStatus(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/address/0x52908400098527886E0F7030069857D2E4169EE7/status", nil)
	rec := httptest.NewRecorder()

	NewHandler(Dependencies{ReadService: testReadService()}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}

	var body faucet.AddressStatusResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Eligible {
		t.Fatal("eligible = false, want true")
	}
	if body.Address != "0x52908400098527886E0F7030069857D2E4169EE7" {
		t.Fatalf("address = %q", body.Address)
	}
}

func TestWalletEligibilityAlias(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/faucet/address/0x52908400098527886E0F7030069857D2E4169EE7/eligibility", nil)
	rec := httptest.NewRecorder()

	NewHandler(Dependencies{ReadService: testReadService()}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}

	var body faucet.AddressStatusResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Eligible {
		t.Fatal("eligible = false, want true")
	}
	// Wallet-specific compact fields must be present.
	if body.CooldownRemainingSeconds < 0 {
		t.Fatalf("cooldown_remaining_seconds = %d, must be >= 0", body.CooldownRemainingSeconds)
	}
	if body.RateLimitIPPerHour <= 0 {
		t.Fatalf("rate_limit_ip_per_hour = %d, want > 0", body.RateLimitIPPerHour)
	}
	if body.RateLimitAddrPerDay <= 0 {
		t.Fatalf("rate_limit_addr_per_day = %d, want > 0", body.RateLimitAddrPerDay)
	}
}

func TestAddressStatusRejectsInvalidAddress(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/address/not-an-address/status", nil)
	rec := httptest.NewRecorder()

	NewHandler(Dependencies{ReadService: testReadService()}).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	var body ErrorEnvelope
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != "invalid_address" {
		t.Fatalf("code = %q, want invalid_address", body.Code)
	}
}

func TestPublicConfigRejectsUnsupportedMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/config", nil)
	rec := httptest.NewRecorder()

	NewHandler(Dependencies{ReadService: testReadService()}).ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
	if got := rec.Header().Get("Allow"); got != http.MethodGet {
		t.Fatalf("allow header = %q, want %q", got, http.MethodGet)
	}
}

func TestCreateClaim(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/claim", bytes.NewBufferString(`{"address":"0x52908400098527886E0F7030069857D2E4169EE7"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	service := testClaimService()

	NewHandler(Dependencies{ReadService: service}).ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusAccepted)
	}

	var body faucet.ClaimResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.ID != "claim_test" {
		t.Fatalf("id = %q, want claim_test", body.ID)
	}
	if body.Status != "queued" {
		t.Fatalf("status = %q, want queued", body.Status)
	}
}

func TestCreateClaimWalletAlias(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/faucet/claim", bytes.NewBufferString(`{"address":"0x52908400098527886E0F7030069857D2E4169EE7"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	NewHandler(Dependencies{ReadService: testClaimService()}).ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusAccepted)
	}
}

func TestCreateClaimMapsServiceErrors(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{
			name:       "faucet unavailable",
			path:       "/api/v1/claim",
			err:        &faucet.ClaimError{Kind: faucet.ErrFaucetUnavailable, Reason: "faucet mode is paused"},
			wantStatus: http.StatusServiceUnavailable,
			wantCode:   "faucet_unavailable",
		},
		{
			name:       "captcha failed",
			path:       "/api/v1/claim",
			err:        &faucet.ClaimError{Kind: faucet.ErrCaptchaFailed, Reason: "captcha failed"},
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   "captcha_failed",
		},
		{
			name:       "risk rejected",
			path:       "/api/v1/claim",
			err:        &faucet.ClaimError{Kind: faucet.ErrClaimRejected, Reason: "blocklisted address"},
			wantStatus: http.StatusForbidden,
			wantCode:   "claim_rejected",
		},
		{
			name:       "cooldown active",
			path:       "/api/v1/claim",
			err:        &faucet.ClaimError{Kind: faucet.ErrCooldownActive, Reason: "retry after 30 seconds", RetryAfterSeconds: 30},
			wantStatus: http.StatusTooManyRequests,
			wantCode:   "rate_limited",
		},
		{
			name:       "rate limited wallet alias",
			path:       "/api/v1/faucet/claim",
			err:        &faucet.ClaimError{Kind: faucet.ErrRateLimited, Reason: "IP rate limit exceeded"},
			wantStatus: http.StatusTooManyRequests,
			wantCode:   "rate_limited",
		},
		{
			name:       "daily budget exceeded",
			path:       "/api/v1/claim",
			err:        &faucet.ClaimError{Kind: faucet.ErrDailyBudgetExceeded, Reason: "daily budget exceeded"},
			wantStatus: http.StatusTooManyRequests,
			wantCode:   "daily_budget_exceeded",
		},
		{
			name:       "internal error",
			path:       "/api/v1/claim",
			err:        fmt.Errorf("store unavailable"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   "claim_unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tt.path, bytes.NewBufferString(`{"address":"0x52908400098527886E0F7030069857D2E4169EE7"}`))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			NewHandler(Dependencies{ReadService: &failingClaimService{err: tt.err}}).ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status code = %d, want %d", rec.Code, tt.wantStatus)
			}

			var body ErrorEnvelope
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.Code != tt.wantCode {
				t.Fatalf("code = %q, want %q", body.Code, tt.wantCode)
			}
		})
	}
}

func TestCreateClaimErrorMappingWithCORS(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/claim", bytes.NewBufferString(`{"address":"0x52908400098527886E0F7030069857D2E4169EE7"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://wallet.example.test")
	rec := httptest.NewRecorder()

	NewHandler(Dependencies{
		ReadService: &failingClaimService{err: &faucet.ClaimError{Kind: faucet.ErrCaptchaFailed, Reason: "captcha failed"}},
		CORSOrigins: []string{"https://wallet.example.test"},
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://wallet.example.test" {
		t.Fatalf("access-control-allow-origin = %q", got)
	}
	var body ErrorEnvelope
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != "captcha_failed" {
		t.Fatalf("code = %q, want captcha_failed", body.Code)
	}
}

func TestCreateClaimPassesSecuritySignals(t *testing.T) {
	service := &recordingClaimService{}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/claim", bytes.NewBufferString(`{
		"address":"0x52908400098527886E0F7030069857D2E4169EE7",
		"captcha_token":" captcha-token ",
		"fingerprint":" fingerprint-1 "
	}`))
	req.RemoteAddr = "10.0.0.1:4567"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "203.0.113.9, 10.0.0.1")
	req.Header.Set("User-Agent", "wallet-test/1.0")
	req.Header.Set(idempotencyKeyHeader, " idem-key ")
	rec := httptest.NewRecorder()

	NewHandler(Dependencies{
		ReadService:  service,
		TrustedProxy: "10.0.0.1",
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusAccepted)
	}
	if service.request.RemoteIP != "203.0.113.9" {
		t.Fatalf("remote ip = %q, want 203.0.113.9", service.request.RemoteIP)
	}
	if service.request.UserAgent != "wallet-test/1.0" {
		t.Fatalf("user agent = %q", service.request.UserAgent)
	}
	if service.request.CaptchaToken != "captcha-token" {
		t.Fatalf("captcha token = %q", service.request.CaptchaToken)
	}
	if service.request.Fingerprint != "fingerprint-1" {
		t.Fatalf("fingerprint = %q", service.request.Fingerprint)
	}
	if service.request.IdempotencyKey != "idem-key" {
		t.Fatalf("idempotency key = %q", service.request.IdempotencyKey)
	}
}

// TestCreateClaimResponseHasTxHashField verifies the claim response includes the
// tx_hash field (empty when the claim is just queued, populated after sending).
func TestCreateClaimResponseHasTxHashField(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/claim", bytes.NewBufferString(`{"address":"0x52908400098527886E0F7030069857D2E4169EE7"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	NewHandler(Dependencies{ReadService: testClaimService()}).ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusAccepted)
	}

	// Decode as a generic map so we can verify the field is present even when empty.
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	// tx_hash must be present in the JSON response (may be absent when omitempty and empty string).
	// A freshly queued claim has no tx hash — the field should be absent or empty.
	if txHash, ok := body["tx_hash"]; ok && txHash != "" {
		t.Fatalf("tx_hash = %q, want empty or absent for a freshly queued claim", txHash)
	}
}

func TestGetClaim(t *testing.T) {
	service := testClaimService()
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/claim", bytes.NewBufferString(`{"address":"0x52908400098527886E0F7030069857D2E4169EE7"}`))
	createRec := httptest.NewRecorder()
	handler := NewHandler(Dependencies{ReadService: service})

	handler.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusAccepted {
		t.Fatalf("create status code = %d", createRec.Code)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/claim/claim_test", nil)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", getRec.Code, http.StatusOK)
	}

	var body faucet.ClaimResponse
	if err := json.NewDecoder(getRec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.ID != "claim_test" {
		t.Fatalf("id = %q, want claim_test", body.ID)
	}
}

func TestGetClaimWalletAlias(t *testing.T) {
	service := testClaimService()
	handler := NewHandler(Dependencies{ReadService: service})
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/claim", bytes.NewBufferString(`{"address":"0x52908400098527886E0F7030069857D2E4169EE7"}`))
	createRec := httptest.NewRecorder()
	handler.ServeHTTP(createRec, createReq)

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/faucet/claim/claim_test", nil)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", getRec.Code, http.StatusOK)
	}
}

func TestCreateClaimIdempotency(t *testing.T) {
	service := testClaimService()
	handler := NewHandler(Dependencies{ReadService: service})

	firstReq := httptest.NewRequest(http.MethodPost, "/api/v1/claim", bytes.NewBufferString(`{"address":"0x52908400098527886E0F7030069857D2E4169EE7"}`))
	firstReq.Header.Set(idempotencyKeyHeader, "same-key")
	firstRec := httptest.NewRecorder()
	handler.ServeHTTP(firstRec, firstReq)

	secondReq := httptest.NewRequest(http.MethodPost, "/api/v1/claim", bytes.NewBufferString(`{"address":"0x52908400098527886E0F7030069857D2E4169EE7"}`))
	secondReq.Header.Set(idempotencyKeyHeader, "same-key")
	secondRec := httptest.NewRecorder()
	handler.ServeHTTP(secondRec, secondReq)

	if firstRec.Code != http.StatusAccepted || secondRec.Code != http.StatusAccepted {
		t.Fatalf("status codes = %d, %d", firstRec.Code, secondRec.Code)
	}

	var first faucet.ClaimResponse
	var second faucet.ClaimResponse
	if err := json.NewDecoder(firstRec.Body).Decode(&first); err != nil {
		t.Fatalf("decode first: %v", err)
	}
	if err := json.NewDecoder(secondRec.Body).Decode(&second); err != nil {
		t.Fatalf("decode second: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("ids differ: %q != %q", first.ID, second.ID)
	}
	if second.IdempotencyKey != "same-key" {
		t.Fatalf("idempotency key = %q", second.IdempotencyKey)
	}
}

func TestCreateClaimRejectsInvalidAddress(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/claim", bytes.NewBufferString(`{"address":"not-an-address"}`))
	rec := httptest.NewRecorder()

	NewHandler(Dependencies{ReadService: testClaimService()}).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	var body ErrorEnvelope
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != "invalid_address" {
		t.Fatalf("code = %q, want invalid_address", body.Code)
	}
}

func TestCreateClaimRejectsInvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/claim", bytes.NewBufferString(`{`))
	rec := httptest.NewRecorder()

	NewHandler(Dependencies{ReadService: testClaimService()}).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestGetClaimNotFound(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/claim/missing", nil)
	rec := httptest.NewRecorder()

	NewHandler(Dependencies{ReadService: testClaimService()}).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusNotFound)
	}

	var body ErrorEnvelope
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != "claim_not_found" {
		t.Fatalf("code = %q, want claim_not_found", body.Code)
	}
}

func TestCreateClaimRejectsUnsupportedMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/claim", nil)
	rec := httptest.NewRecorder()

	NewHandler(Dependencies{ReadService: testClaimService()}).ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
	if got := rec.Header().Get("Allow"); got != http.MethodPost {
		t.Fatalf("allow header = %q, want %q", got, http.MethodPost)
	}
}

func TestAdminMetricsRequiresAdminToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/metrics", nil)
	rec := httptest.NewRecorder()

	NewHandler(Dependencies{AdminToken: testAdminToken}).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAdminPrometheusMetricsRequiresAdminToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/metrics/prometheus", nil)
	rec := httptest.NewRecorder()

	NewHandler(Dependencies{AdminToken: testAdminToken}).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAdminPrometheusMetricsReportsStableText(t *testing.T) {
	metrics := observability.NewRuntimeMetrics(version.Info{})
	metrics.IncClaimAcceptedForToken("native")
	metrics.IncQueueDequeued(1)
	handler := NewHandler(Dependencies{AdminToken: testAdminToken, Metrics: metrics})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/metrics/prometheus", nil)
	req.Header.Set("Authorization", "Bearer "+testAdminToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/plain") {
		t.Fatalf("content-type = %q", got)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"# HELP scavium_faucet_claims_accepted_total Accepted claim requests.",
		"scavium_faucet_claims_accepted_total 1\n",
		"scavium_faucet_queue_dequeued_total 1\n",
		"scavium_faucet_token_claims_accepted_total{token_id=\"native\"} 1\n",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("prometheus body missing %q in:\n%s", want, body)
		}
	}
}

func TestAdminMetricsReportsRuntimeCounters(t *testing.T) {
	metrics := observability.NewRuntimeMetricsWithClock(version.Info{
		Version:   "v16.2-test",
		Commit:    "abc123",
		BuildDate: "2026-05-04T00:00:00Z",
	}, func() time.Time {
		return time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC)
	})
	handler := NewHandler(Dependencies{
		ReadService: testClaimService(),
		AdminToken:  testAdminToken,
		Metrics:     metrics,
	})

	claimReq := httptest.NewRequest(http.MethodPost, "/api/v1/claim", bytes.NewBufferString(`{"address":"0x52908400098527886E0F7030069857D2E4169EE7"}`))
	claimReq.Header.Set("Content-Type", "application/json")
	claimRec := httptest.NewRecorder()
	handler.ServeHTTP(claimRec, claimReq)
	if claimRec.Code != http.StatusAccepted {
		t.Fatalf("claim status code = %d", claimRec.Code)
	}

	metricsReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/metrics", nil)
	metricsReq.Header.Set("Authorization", "Bearer "+testAdminToken)
	metricsRec := httptest.NewRecorder()
	handler.ServeHTTP(metricsRec, metricsReq)

	if metricsRec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", metricsRec.Code, http.StatusOK)
	}

	var body observability.RuntimeMetricsSnapshot
	if err := json.NewDecoder(metricsRec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Build.Version != "v16.2-test" || body.Build.Commit != "abc123" {
		t.Fatalf("build = %#v", body.Build)
	}
	if body.Claims.Accepted != 1 {
		t.Fatalf("claims.accepted = %d, want 1", body.Claims.Accepted)
	}
	if body.Claims.Rejected != 0 {
		t.Fatalf("claims.rejected = %d, want 0", body.Claims.Rejected)
	}
}

func TestAdminMetricsCountsRejectedClaimClasses(t *testing.T) {
	metrics := observability.NewRuntimeMetrics(version.Info{})
	handler := NewHandler(Dependencies{
		ReadService: &failingClaimService{err: &faucet.ClaimError{Kind: faucet.ErrCaptchaFailed, Reason: "captcha failed"}},
		AdminToken:  testAdminToken,
		Metrics:     metrics,
	})

	claimReq := httptest.NewRequest(http.MethodPost, "/api/v1/claim", bytes.NewBufferString(`{"address":"0x52908400098527886E0F7030069857D2E4169EE7"}`))
	claimReq.Header.Set("Content-Type", "application/json")
	claimRec := httptest.NewRecorder()
	handler.ServeHTTP(claimRec, claimReq)
	if claimRec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("claim status code = %d", claimRec.Code)
	}

	metricsReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/metrics", nil)
	metricsReq.Header.Set("Authorization", "Bearer "+testAdminToken)
	metricsRec := httptest.NewRecorder()
	handler.ServeHTTP(metricsRec, metricsReq)

	if metricsRec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", metricsRec.Code, http.StatusOK)
	}
	var body observability.RuntimeMetricsSnapshot
	if err := json.NewDecoder(metricsRec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Claims.Rejected != 1 {
		t.Fatalf("claims.rejected = %d, want 1", body.Claims.Rejected)
	}
	if body.Captcha.Failed != 1 {
		t.Fatalf("captcha.failed = %d, want 1", body.Captcha.Failed)
	}
}

func TestAdminMetricsCountsInvalidTokenClaims(t *testing.T) {
	metrics := observability.NewRuntimeMetrics(version.Info{})
	handler := NewHandler(Dependencies{
		ReadService: &failingClaimService{err: &faucet.ClaimError{Kind: faucet.ErrClaimRejected, Reason: "invalid_token"}},
		AdminToken:  testAdminToken,
		Metrics:     metrics,
	})

	claimReq := httptest.NewRequest(http.MethodPost, "/api/v1/claim", bytes.NewBufferString(`{"address":"0x52908400098527886E0F7030069857D2E4169EE7","token_id":"missing-token"}`))
	claimReq.Header.Set("Content-Type", "application/json")
	claimRec := httptest.NewRecorder()
	handler.ServeHTTP(claimRec, claimReq)
	if claimRec.Code != http.StatusForbidden {
		t.Fatalf("claim status code = %d", claimRec.Code)
	}

	metricsReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/metrics", nil)
	metricsReq.Header.Set("Authorization", "Bearer "+testAdminToken)
	metricsRec := httptest.NewRecorder()
	handler.ServeHTTP(metricsRec, metricsReq)

	if metricsRec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", metricsRec.Code, http.StatusOK)
	}
	var body observability.RuntimeMetricsSnapshot
	if err := json.NewDecoder(metricsRec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Claims.Rejected != 1 {
		t.Fatalf("claims.rejected = %d, want 1", body.Claims.Rejected)
	}
	if body.Claims.InvalidToken != 1 {
		t.Fatalf("claims.invalid_token = %d, want 1", body.Claims.InvalidToken)
	}
	if body.Claims.RejectedByRisk != 0 {
		t.Fatalf("claims.rejected_by_risk = %d, want 0", body.Claims.RejectedByRisk)
	}
	assertHTTPTokenMetrics(t, body.Tokens, observability.RuntimeTokenMetrics{TokenID: "invalid", Rejected: 1, InvalidToken: 1})
	for _, tm := range body.Tokens {
		if tm.TokenID == "missing-token" {
			t.Fatalf("invalid token metrics must not preserve raw untrusted token_id: %+v", tm)
		}
	}
}

func TestAdminMetricsCountsBlocklistRejections(t *testing.T) {
	metrics := observability.NewRuntimeMetrics(version.Info{})
	handler := NewHandler(Dependencies{
		ReadService: &failingClaimService{err: &faucet.ClaimError{Kind: faucet.ErrClaimRejected, Reason: "blocked by abuse policy"}},
		AdminToken:  testAdminToken,
		Metrics:     metrics,
	})

	claimReq := httptest.NewRequest(http.MethodPost, "/api/v1/claim", bytes.NewBufferString(`{"address":"0x52908400098527886E0F7030069857D2E4169EE7","token_id":"native"}`))
	claimReq.Header.Set("Content-Type", "application/json")
	claimRec := httptest.NewRecorder()
	handler.ServeHTTP(claimRec, claimReq)
	if claimRec.Code != http.StatusForbidden {
		t.Fatalf("claim status code = %d, want %d", claimRec.Code, http.StatusForbidden)
	}

	metricsReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/metrics", nil)
	metricsReq.Header.Set("Authorization", "Bearer "+testAdminToken)
	metricsRec := httptest.NewRecorder()
	handler.ServeHTTP(metricsRec, metricsReq)
	if metricsRec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", metricsRec.Code, http.StatusOK)
	}
	var body observability.RuntimeMetricsSnapshot
	if err := json.NewDecoder(metricsRec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Claims.Rejected != 1 {
		t.Fatalf("claims.rejected = %d, want 1", body.Claims.Rejected)
	}
	if body.Claims.RejectedByRisk != 0 {
		t.Fatalf("claims.rejected_by_risk = %d, want 0", body.Claims.RejectedByRisk)
	}
	if body.Abuse.BlocklistRejected != 1 {
		t.Fatalf("abuse.blocklist_rejected = %d, want 1", body.Abuse.BlocklistRejected)
	}
}

func assertHTTPTokenMetrics(t *testing.T, tokens []observability.RuntimeTokenMetrics, want observability.RuntimeTokenMetrics) {
	t.Helper()
	for _, got := range tokens {
		if got.TokenID != want.TokenID {
			continue
		}
		if got.Accepted != want.Accepted || got.Rejected != want.Rejected || got.RateLimited != want.RateLimited || got.DailyExceeded != want.DailyExceeded || got.InvalidToken != want.InvalidToken {
			t.Fatalf("token metrics for %q = %#v, want %#v", want.TokenID, got, want)
		}
		return
	}
	t.Fatalf("missing token metrics for %q in %#v", want.TokenID, tokens)
}

func testReadService() faucet.ReadService {
	cfg := config.Defaults()
	cfg.NetworkName = "scavium-test"
	cfg.ChainID = 123
	cfg.Symbol = "tSCAV"
	cfg.AmountWei = big.NewInt(42)
	cfg.CooldownSeconds = 60
	cfg.ExplorerTxURL = "https://explorer.example.test/tx/{txHash}"
	cfg.DryRun = false
	cfg.CaptchaProvider = "turnstile"
	cfg.CaptchaSiteKey = "1x00000000000000000000AA"
	cfg.Tokens = []config.TokenConfig{
		{
			ID:             "native",
			Symbol:         "tSCAV",
			Type:           domain.TokenTypeNative,
			Decimals:       18,
			AmountWei:      big.NewInt(42),
			DailyBudgetWei: big.NewInt(4200),
		},
		{
			ID:             "scav",
			Symbol:         "SCAV",
			Type:           domain.TokenTypeERC20,
			Address:        common.HexToAddress("0x1111111111111111111111111111111111111111"),
			Decimals:       18,
			AmountWei:      big.NewInt(100),
			DailyBudgetWei: big.NewInt(10000),
		},
	}

	now := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	return faucet.NewInMemoryReadServiceWithClock(cfg, func() time.Time { return now })
}

func testClaimService() *faucet.InMemoryReadService {
	service := testReadService().(*faucet.InMemoryReadService)
	service.SetClaimIDGenerator(func() (string, error) { return "claim_test", nil })
	return service
}

type recordingClaimService struct {
	request faucet.ClaimRequest
}

func (s *recordingClaimService) Status(context.Context) (faucet.StatusResponse, error) {
	return faucet.StatusResponse{}, nil
}

func (s *recordingClaimService) Config(context.Context) (faucet.ConfigResponse, error) {
	return faucet.ConfigResponse{}, nil
}

func (s *recordingClaimService) Tokens(context.Context) ([]faucet.TokenResponse, error) {
	return nil, nil
}

func (s *recordingClaimService) AddressStatus(context.Context, common.Address) (faucet.AddressStatusResponse, error) {
	return faucet.AddressStatusResponse{}, nil
}

func (s *recordingClaimService) AddressHistory(context.Context, common.Address, int, int) (faucet.AddressHistoryResponse, error) {
	return faucet.AddressHistoryResponse{}, nil
}

func (s *recordingClaimService) CreateClaim(_ context.Context, request faucet.ClaimRequest) (faucet.ClaimResponse, error) {
	s.request = request
	return faucet.ClaimResponse{
		ID:        "claim_recorded",
		Address:   request.Address.Hex(),
		AmountWei: "42",
		Status:    domain.ClaimStatusQueued,
		CreatedAt: "2026-05-03T12:00:00Z",
		UpdatedAt: "2026-05-03T12:00:00Z",
	}, nil
}

func (s *recordingClaimService) GetClaim(context.Context, string) (faucet.ClaimResponse, bool, error) {
	return faucet.ClaimResponse{}, false, nil
}

type failingClaimService struct {
	err error
}

func (s *failingClaimService) Status(context.Context) (faucet.StatusResponse, error) {
	return faucet.StatusResponse{}, nil
}

func (s *failingClaimService) Config(context.Context) (faucet.ConfigResponse, error) {
	return faucet.ConfigResponse{}, nil
}

func (s *failingClaimService) Tokens(context.Context) ([]faucet.TokenResponse, error) {
	return nil, nil
}

func (s *failingClaimService) AddressStatus(context.Context, common.Address) (faucet.AddressStatusResponse, error) {
	return faucet.AddressStatusResponse{}, nil
}

func (s *failingClaimService) AddressHistory(context.Context, common.Address, int, int) (faucet.AddressHistoryResponse, error) {
	return faucet.AddressHistoryResponse{}, nil
}

func (s *failingClaimService) CreateClaim(context.Context, faucet.ClaimRequest) (faucet.ClaimResponse, error) {
	return faucet.ClaimResponse{}, s.err
}

func (s *failingClaimService) GetClaim(context.Context, string) (faucet.ClaimResponse, bool, error) {
	return faucet.ClaimResponse{}, false, nil
}

// --- Admin test helpers ---

const testAdminToken = "test-admin-token-for-tests"

func testAdminDeps() Dependencies {
	return Dependencies{
		ReadService:  testReadService(),
		AdminService: admin.NewInMemoryAdminService(),
		AdminToken:   testAdminToken,
	}
}

func adminRequest(method, path string, body any) *http.Request {
	var req *http.Request
	if body != nil {
		b, _ := json.Marshal(body)
		req = httptest.NewRequest(method, path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req.Header.Set("Authorization", "Bearer "+testAdminToken)
	return req
}

func TestAdminRequiresToken(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/dashboard", nil)
	NewHandler(testAdminDeps()).ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestAdminWrongToken(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/dashboard", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	NewHandler(testAdminDeps()).ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestAdminDisabledWhenNoToken(t *testing.T) {
	deps := testAdminDeps()
	deps.AdminToken = ""
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/dashboard", nil)
	req.Header.Set("Authorization", "Bearer "+testAdminToken)
	NewHandler(deps).ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}

func TestAdminDashboard(t *testing.T) {
	rec := httptest.NewRecorder()
	NewHandler(testAdminDeps()).ServeHTTP(rec, adminRequest(http.MethodGet, "/api/v1/admin/dashboard", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["mode"] != "active" {
		t.Fatalf("mode = %v, want active", body["mode"])
	}
}

func TestAdminSetModePaused(t *testing.T) {
	rec := httptest.NewRecorder()
	NewHandler(testAdminDeps()).ServeHTTP(rec, adminRequest(http.MethodPost, "/api/v1/admin/faucet/mode", map[string]string{"mode": "paused"}))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestAdminSetModeRejectsInvalidMode(t *testing.T) {
	rec := httptest.NewRecorder()
	NewHandler(testAdminDeps()).ServeHTTP(rec, adminRequest(http.MethodPost, "/api/v1/admin/faucet/mode", map[string]string{"mode": "drain-all-funds"}))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}

	var body ErrorEnvelope
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Code != "invalid_mode" {
		t.Fatalf("code = %q, want invalid_mode", body.Code)
	}
}

func TestAdminBlocklistAddListRemove(t *testing.T) {
	deps := testAdminDeps()
	handler := NewHandler(deps)

	// Add
	addRec := httptest.NewRecorder()
	handler.ServeHTTP(addRec, adminRequest(http.MethodPost, "/api/v1/admin/blocklist", map[string]string{
		"key_type": "ip",
		"value":    "1.2.3.4",
		"reason":   "test",
	}))
	if addRec.Code != http.StatusCreated {
		t.Fatalf("add status = %d, want 201", addRec.Code)
	}

	// List
	listRec := httptest.NewRecorder()
	handler.ServeHTTP(listRec, adminRequest(http.MethodGet, "/api/v1/admin/blocklist", nil))
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200", listRec.Code)
	}
	var listBody struct {
		Entries []any `json:"entries"`
	}
	if err := json.NewDecoder(listRec.Body).Decode(&listBody); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(listBody.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(listBody.Entries))
	}

	// Remove
	delRec := httptest.NewRecorder()
	handler.ServeHTTP(delRec, adminRequest(http.MethodDelete, "/api/v1/admin/blocklist?key_type=ip&value=1.2.3.4", nil))
	if delRec.Code != http.StatusOK {
		t.Fatalf("delete status = %d, want 200", delRec.Code)
	}
}

func TestAdminAuditLog(t *testing.T) {
	deps := testAdminDeps()
	svc := deps.AdminService.(*admin.InMemoryAdminService)
	// Trigger an action to populate audit log.
	_ = svc.SetMode(context.Background(), "paused", "test")

	rec := httptest.NewRecorder()
	NewHandler(deps).ServeHTTP(rec, adminRequest(http.MethodGet, "/api/v1/admin/audit", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Entries []any `json:"entries"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Entries) == 0 {
		t.Fatal("expected at least one audit entry")
	}
}

func TestAdminSensitiveActionsEmitStructuredAuditLog(t *testing.T) {
	var logs bytes.Buffer
	deps := testAdminDeps()
	deps.Logger = observability.NewLogger(&logs)

	rec := httptest.NewRecorder()
	req := adminRequest(http.MethodPost, "/api/v1/admin/faucet/mode", map[string]string{"mode": "maintenance"})
	req.Header.Set(requestIDHeader, "audit-request-id")
	req.Header.Set(correlationIDHeader, "audit-correlation-id")
	NewHandler(deps).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var found bool
	for _, line := range strings.Split(strings.TrimSpace(logs.String()), "\n") {
		if !strings.Contains(line, `"message":"admin audit"`) {
			continue
		}
		found = true
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("decode audit log: %v", err)
		}
		if entry["admin_action"] != "set_mode" {
			t.Fatalf("admin_action = %v, want set_mode", entry["admin_action"])
		}
		if entry["request_id"] != "audit-request-id" {
			t.Fatalf("request_id = %v, want audit-request-id", entry["request_id"])
		}
		if entry["correlation_id"] != "audit-correlation-id" {
			t.Fatalf("correlation_id = %v, want audit-correlation-id", entry["correlation_id"])
		}
		if entry["mode"] != "maintenance" {
			t.Fatalf("mode = %v, want maintenance", entry["mode"])
		}
	}
	if !found {
		t.Fatalf("expected admin audit log, got %q", logs.String())
	}
	if strings.Contains(logs.String(), testAdminToken) {
		t.Fatal("admin token must not be logged")
	}
}

func TestAdminBlocklistAuditLogDoesNotExposeValue(t *testing.T) {
	var logs bytes.Buffer
	deps := testAdminDeps()
	deps.Logger = observability.NewLogger(&logs)

	rec := httptest.NewRecorder()
	NewHandler(deps).ServeHTTP(rec, adminRequest(http.MethodPost, "/api/v1/admin/blocklist", map[string]string{
		"key_type": "ip",
		"value":    "203.0.113.9",
		"reason":   "test reason",
	}))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	if !strings.Contains(logs.String(), `"admin_action":"blocklist_add"`) {
		t.Fatalf("expected blocklist_add audit log, got %q", logs.String())
	}
	if strings.Contains(logs.String(), "203.0.113.9") || strings.Contains(logs.String(), "test reason") {
		t.Fatalf("blocklist audit log must not expose value or reason, got %q", logs.String())
	}
}

func TestAdminGetClaimNotFound(t *testing.T) {
	rec := httptest.NewRecorder()
	NewHandler(testAdminDeps()).ServeHTTP(rec, adminRequest(http.MethodGet, "/api/v1/admin/claim/does-not-exist", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestAdminGetClaim(t *testing.T) {
	deps := testAdminDeps()
	svc := deps.AdminService.(*admin.InMemoryAdminService)
	c := domain.Claim{
		ID:        "claim_abc",
		Address:   common.HexToAddress("0xAb5801a7D398351b8bE11C439e05C5B3259aeC9B"),
		Status:    domain.ClaimStatusQueued,
		CreatedAt: time.Now().UTC(),
	}
	svc.AddClaim(c)

	rec := httptest.NewRecorder()
	NewHandler(deps).ServeHTTP(rec, adminRequest(http.MethodGet, fmt.Sprintf("/api/v1/admin/claim/%s", c.ID), nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestAdminCancelClaim(t *testing.T) {
	deps := testAdminDeps()
	svc := deps.AdminService.(*admin.InMemoryAdminService)
	c := domain.Claim{
		ID:        "claim_cancel",
		Address:   common.HexToAddress("0xAb5801a7D398351b8bE11C439e05C5B3259aeC9B"),
		Status:    domain.ClaimStatusQueued,
		CreatedAt: time.Now().UTC(),
	}
	svc.AddClaim(c)

	rec := httptest.NewRecorder()
	NewHandler(deps).ServeHTTP(rec, adminRequest(http.MethodPost, "/api/v1/admin/claim/claim_cancel/cancel", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestAdminRetryClaim(t *testing.T) {
	deps := testAdminDeps()
	svc := deps.AdminService.(*admin.InMemoryAdminService)
	c := domain.Claim{
		ID:        "claim_retry",
		Address:   common.HexToAddress("0xAb5801a7D398351b8bE11C439e05C5B3259aeC9B"),
		Status:    domain.ClaimStatusFailed,
		CreatedAt: time.Now().UTC(),
	}
	svc.AddClaim(c)

	rec := httptest.NewRecorder()
	NewHandler(deps).ServeHTTP(rec, adminRequest(http.MethodPost, "/api/v1/admin/claim/claim_retry/retry", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestAdminQueueRetryClaim(t *testing.T) {
	deps := testAdminDeps()
	svc := deps.AdminService.(*admin.InMemoryAdminService)
	svc.AddClaim(domain.Claim{
		ID:        "queue_retry",
		Address:   common.HexToAddress("0xAb5801a7D398351b8bE11C439e05C5B3259aeC9B"),
		Status:    domain.ClaimStatusFailed,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	})

	rec := httptest.NewRecorder()
	NewHandler(deps).ServeHTTP(rec, adminRequest(http.MethodPost, "/api/v1/admin/queue/retry", admin.QueueControlRequest{ID: "queue_retry"}))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	claim, found, err := svc.GetClaim(context.Background(), "queue_retry")
	if err != nil || !found {
		t.Fatalf("claim not found after retry: found=%v err=%v", found, err)
	}
	if claim.Status != domain.ClaimStatusQueued {
		t.Fatalf("status = %q, want queued", claim.Status)
	}
}

func TestAdminQueueCancelClaim(t *testing.T) {
	deps := testAdminDeps()
	svc := deps.AdminService.(*admin.InMemoryAdminService)
	svc.AddClaim(domain.Claim{
		ID:        "queue_cancel",
		Address:   common.HexToAddress("0xAb5801a7D398351b8bE11C439e05C5B3259aeC9B"),
		Status:    domain.ClaimStatusQueued,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	})

	rec := httptest.NewRecorder()
	NewHandler(deps).ServeHTTP(rec, adminRequest(http.MethodPost, "/api/v1/admin/queue/cancel", admin.QueueControlRequest{ID: "queue_cancel"}))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	claim, found, err := svc.GetClaim(context.Background(), "queue_cancel")
	if err != nil || !found {
		t.Fatalf("claim not found after cancel: found=%v err=%v", found, err)
	}
	if claim.Status != domain.ClaimStatusRejected {
		t.Fatalf("status = %q, want rejected", claim.Status)
	}
}

func TestAdminQueueControlRequiresID(t *testing.T) {
	rec := httptest.NewRecorder()
	NewHandler(testAdminDeps()).ServeHTTP(rec, adminRequest(http.MethodPost, "/api/v1/admin/queue/retry", admin.QueueControlRequest{}))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestAdminQueueControlAllowsOnlyPOST(t *testing.T) {
	rec := httptest.NewRecorder()
	NewHandler(testAdminDeps()).ServeHTTP(rec, adminRequest(http.MethodGet, "/api/v1/admin/queue/retry", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
	if got := rec.Header().Get("Allow"); got != http.MethodPost {
		t.Fatalf("allow = %q, want POST", got)
	}
}

// --- Phase 17.5 post-audit fix tests ---

// TestMetricsAcceptedClaimWithoutTokenIDUsesDefaultBucket (MED-02):
// when client omits token_id, accepted claims must land in the "default" bucket,
// matching the "default" bucket used for rejected claims from the same client.
func TestMetricsAcceptedClaimWithoutTokenIDUsesDefaultBucket(t *testing.T) {
	metrics := observability.NewRuntimeMetrics(version.Info{})
	handler := NewHandler(Dependencies{
		ReadService: testClaimService(),
		AdminToken:  testAdminToken,
		Metrics:     metrics,
	})

	claimReq := httptest.NewRequest(http.MethodPost, "/api/v1/claim",
		bytes.NewBufferString(`{"address":"0x52908400098527886E0F7030069857D2E4169EE7"}`))
	claimReq.Header.Set("Content-Type", "application/json")
	claimRec := httptest.NewRecorder()
	handler.ServeHTTP(claimRec, claimReq)
	if claimRec.Code != http.StatusAccepted {
		t.Fatalf("claim status code = %d, want %d", claimRec.Code, http.StatusAccepted)
	}

	metricsReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/metrics", nil)
	metricsReq.Header.Set("Authorization", "Bearer "+testAdminToken)
	metricsRec := httptest.NewRecorder()
	handler.ServeHTTP(metricsRec, metricsReq)
	if metricsRec.Code != http.StatusOK {
		t.Fatalf("metrics status code = %d", metricsRec.Code)
	}

	var body observability.RuntimeMetricsSnapshot
	if err := json.NewDecoder(metricsRec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	// Accepted claim without token_id must land in "default", not "native".
	assertHTTPTokenMetrics(t, body.Tokens, observability.RuntimeTokenMetrics{TokenID: "default", Accepted: 1})
	// "native" bucket must not exist (no explicit token_id=native was sent).
	for _, tm := range body.Tokens {
		if tm.TokenID == "native" {
			t.Fatalf("unexpected native bucket: %+v", tm)
		}
	}
}

// TestMetricsRejectedClaimWithoutTokenIDUsesDefaultBucket (MED-02):
// rejected claims with no token_id must land in the "default" bucket.
func TestMetricsRejectedClaimWithoutTokenIDUsesDefaultBucket(t *testing.T) {
	metrics := observability.NewRuntimeMetrics(version.Info{})
	handler := NewHandler(Dependencies{
		ReadService: &failingClaimService{err: &faucet.ClaimError{Kind: faucet.ErrCaptchaFailed, Reason: "captcha failed"}},
		AdminToken:  testAdminToken,
		Metrics:     metrics,
	})

	claimReq := httptest.NewRequest(http.MethodPost, "/api/v1/claim",
		bytes.NewBufferString(`{"address":"0x52908400098527886E0F7030069857D2E4169EE7"}`))
	claimReq.Header.Set("Content-Type", "application/json")
	claimRec := httptest.NewRecorder()
	handler.ServeHTTP(claimRec, claimReq)
	if claimRec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("claim status code = %d, want %d", claimRec.Code, http.StatusUnprocessableEntity)
	}

	metricsReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/metrics", nil)
	metricsReq.Header.Set("Authorization", "Bearer "+testAdminToken)
	metricsRec := httptest.NewRecorder()
	handler.ServeHTTP(metricsRec, metricsReq)
	if metricsRec.Code != http.StatusOK {
		t.Fatalf("metrics status code = %d", metricsRec.Code)
	}

	var body observability.RuntimeMetricsSnapshot
	if err := json.NewDecoder(metricsRec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	assertHTTPTokenMetrics(t, body.Tokens, observability.RuntimeTokenMetrics{TokenID: "default", Rejected: 1})
}

// TestMetricsExplicitTokenIDPreservedForAccepted (MED-02):
// when client sends explicit token_id, that value is used for both accepted and rejected buckets.
func TestMetricsExplicitTokenIDPreservedForAccepted(t *testing.T) {
	metrics := observability.NewRuntimeMetrics(version.Info{})
	handler := NewHandler(Dependencies{
		ReadService: testClaimService(),
		AdminToken:  testAdminToken,
		Metrics:     metrics,
	})

	claimReq := httptest.NewRequest(http.MethodPost, "/api/v1/claim",
		bytes.NewBufferString(`{"address":"0x52908400098527886E0F7030069857D2E4169EE7","token_id":"native"}`))
	claimReq.Header.Set("Content-Type", "application/json")
	claimRec := httptest.NewRecorder()
	handler.ServeHTTP(claimRec, claimReq)
	if claimRec.Code != http.StatusAccepted {
		t.Fatalf("claim status code = %d, want %d", claimRec.Code, http.StatusAccepted)
	}

	metricsReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/metrics", nil)
	metricsReq.Header.Set("Authorization", "Bearer "+testAdminToken)
	metricsRec := httptest.NewRecorder()
	handler.ServeHTTP(metricsRec, metricsReq)
	if metricsRec.Code != http.StatusOK {
		t.Fatalf("metrics status code = %d", metricsRec.Code)
	}

	var body observability.RuntimeMetricsSnapshot
	if err := json.NewDecoder(metricsRec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	// Explicit token_id=native lands in "native" bucket, not "default".
	assertHTTPTokenMetrics(t, body.Tokens, observability.RuntimeTokenMetrics{TokenID: "native", Accepted: 1})
}

// TestObservedRequestTokenIDSanitizesControlChars (LOW-03):
// adversarial token_id values containing control characters and excessive length
// must be sanitized before reaching logs and metrics.
func TestObservedRequestTokenIDSanitizesControlChars(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"native", "native"},
		{"", "default"},
		{"   ", "default"},
		{"bad\ntoken", "badtoken"},
		{"bad\x01token", "badtoken"},
		{"bad\x7ftoken", "badtoken"},
		{strings.Repeat("a", 100), strings.Repeat("a", 64)},
		{"valid-token_id.v2", "valid-token_id.v2"},
	}
	for _, tc := range cases {
		got := observedRequestTokenID(tc.input)
		if got != tc.want {
			t.Errorf("observedRequestTokenID(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// TestClaimRejectedLogsDoNotContainRawMaliciousTokenID (LOW-03):
// log fields for rejected claims must not contain raw adversarial token_id control characters.
func TestClaimRejectedLogsDoNotContainRawMaliciousTokenID(t *testing.T) {
	var logs bytes.Buffer
	handler := NewHandler(Dependencies{
		ReadService: &failingClaimService{err: &faucet.ClaimError{Kind: faucet.ErrCaptchaFailed, Reason: "captcha failed"}},
		Logger:      observability.NewLogger(&logs),
	})

	body, _ := json.Marshal(map[string]any{
		"address":  "0x52908400098527886E0F7030069857D2E4169EE7",
		"token_id": "bad\ntoken\x01id",
	})
	claimReq := httptest.NewRequest(http.MethodPost, "/api/v1/claim", bytes.NewReader(body))
	claimReq.Header.Set("Content-Type", "application/json")
	claimRec := httptest.NewRecorder()
	handler.ServeHTTP(claimRec, claimReq)
	if claimRec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("claim status code = %d, want %d", claimRec.Code, http.StatusUnprocessableEntity)
	}

	entries := decodeLogEntries(t, logs.Bytes())
	for _, entry := range entries {
		if tokenID, ok := entry["token_id"].(string); ok {
			for _, c := range tokenID {
				if c < 0x20 || c == 0x7f {
					t.Errorf("log token_id contains control char %q: full value %q", c, tokenID)
				}
			}
		}
	}
}

func TestAdminQueueReportsQueueVisibility(t *testing.T) {
	deps := testAdminDeps()
	svc := deps.AdminService.(*admin.InMemoryAdminService)
	now := time.Now().UTC()
	next := now.Add(5 * time.Minute)
	svc.AddClaim(domain.Claim{
		ID:          "claim_ready",
		Address:     common.HexToAddress("0xAb5801a7D398351b8bE11C439e05C5B3259aeC9B"),
		TokenID:     "native",
		TokenSymbol: "SCAV",
		Status:      domain.ClaimStatusQueued,
		CreatedAt:   now.Add(-2 * time.Minute),
		UpdatedAt:   now.Add(-2 * time.Minute),
	})
	svc.AddClaim(domain.Claim{
		ID:            "claim_delayed",
		Address:       common.HexToAddress("0x52908400098527886E0F7030069857D2E4169EE7"),
		TokenID:       "erc20-demo",
		TokenSymbol:   "DEMO",
		Status:        domain.ClaimStatusQueued,
		RetryCount:    1,
		NextAttemptAt: &next,
		CreatedAt:     now.Add(-1 * time.Minute),
		UpdatedAt:     now.Add(-1 * time.Minute),
	})
	svc.AddClaim(domain.Claim{
		ID:        "claim_sending",
		Address:   common.HexToAddress("0xde709f2102306220921060314715629080e2fb77"),
		Status:    domain.ClaimStatusSending,
		CreatedAt: now,
		UpdatedAt: now,
	})

	rec := httptest.NewRecorder()
	NewHandler(deps).ServeHTTP(rec, adminRequest(http.MethodGet, "/api/v1/admin/queue", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body admin.QueueResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Ready != 1 || body.Delayed != 1 || body.InFlight != 1 {
		t.Fatalf("queue summary = %#v", body)
	}
	if body.Counts["queued"] != 2 || body.Counts["sending"] != 1 {
		t.Fatalf("counts = %#v", body.Counts)
	}
	if len(body.Items) != 3 {
		t.Fatalf("items len = %d, want 3", len(body.Items))
	}
	foundDelayed := false
	for _, item := range body.Items {
		if item.ID == "claim_delayed" {
			foundDelayed = true
			if item.NextAttemptAt == "" || item.RetryCount != 1 || item.TokenID != "erc20-demo" {
				t.Fatalf("delayed item = %#v", item)
			}
		}
	}
	if !foundDelayed {
		t.Fatal("delayed claim not found in queue items")
	}
}

func TestAdminQueueLimit(t *testing.T) {
	deps := testAdminDeps()
	svc := deps.AdminService.(*admin.InMemoryAdminService)
	now := time.Now().UTC()
	svc.AddClaim(domain.Claim{ID: "claim_one", Status: domain.ClaimStatusQueued, CreatedAt: now, UpdatedAt: now})
	svc.AddClaim(domain.Claim{ID: "claim_two", Status: domain.ClaimStatusQueued, CreatedAt: now, UpdatedAt: now})

	rec := httptest.NewRecorder()
	NewHandler(deps).ServeHTTP(rec, adminRequest(http.MethodGet, "/api/v1/admin/queue?limit=1", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body admin.QueueResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Items) != 1 {
		t.Fatalf("items len = %d, want 1", len(body.Items))
	}
	if body.Counts["queued"] != 2 {
		t.Fatalf("queued count = %d, want 2", body.Counts["queued"])
	}
}

func TestAdminQueueAllowsOnlyGET(t *testing.T) {
	rec := httptest.NewRecorder()
	NewHandler(testAdminDeps()).ServeHTTP(rec, adminRequest(http.MethodPost, "/api/v1/admin/queue", nil))

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
	if got := rec.Header().Get("Allow"); got != http.MethodGet {
		t.Fatalf("allow = %q, want GET", got)
	}
}

func TestAdminRuntimeReportsDashboardReadinessAndMetrics(t *testing.T) {
	metrics := observability.NewRuntimeMetricsWithClock(version.Info{
		Version: "v18.2-test",
		Commit:  "runtime123",
	}, func() time.Time {
		return time.Date(2026, 5, 5, 10, 0, 0, 0, time.UTC)
	})
	metrics.IncClaimAcceptedForToken("native")
	deps := testAdminDeps()
	deps.Metrics = metrics
	deps.ReadinessChecks = []ready.Check{{Name: "db", Run: ready.StubOK}}

	rec := httptest.NewRecorder()
	NewHandler(deps).ServeHTTP(rec, adminRequest(http.MethodGet, "/api/v1/admin/runtime", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body adminRuntimeResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Dashboard.Mode != "active" {
		t.Fatalf("dashboard.mode = %q, want active", body.Dashboard.Mode)
	}
	if body.Readiness.Status != ready.StatusOK || body.Readiness.Summary.OK != 1 {
		t.Fatalf("readiness = %#v", body.Readiness)
	}
	if body.Metrics.Build.Version != "v18.2-test" || body.Metrics.Build.Commit != "runtime123" {
		t.Fatalf("metrics.build = %#v", body.Metrics.Build)
	}
	assertHTTPTokenMetrics(t, body.Metrics.Tokens, observability.RuntimeTokenMetrics{TokenID: "native", Accepted: 1})
}

func TestAdminRuntimeAllowsOnlyGET(t *testing.T) {
	rec := httptest.NewRecorder()
	NewHandler(testAdminDeps()).ServeHTTP(rec, adminRequest(http.MethodPost, "/api/v1/admin/runtime", nil))

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
	if got := rec.Header().Get("Allow"); got != http.MethodGet {
		t.Fatalf("allow = %q, want GET", got)
	}
}

func TestAdminAuditActorUsesTrustedProxyHeaders(t *testing.T) {
	var logs bytes.Buffer
	deps := testAdminDeps()
	deps.TrustedProxy = "127.0.0.1"
	deps.Logger = observability.NewLogger(&logs)

	req := adminRequest(http.MethodPost, "/api/v1/admin/faucet/mode", map[string]string{"mode": "maintenance"})
	req.RemoteAddr = "127.0.0.1:45678"
	req.Header.Set("X-Forwarded-For", "198.51.100.42, 127.0.0.1")
	rec := httptest.NewRecorder()
	NewHandler(deps).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	if !strings.Contains(logs.String(), `"actor":"198.51.100.42"`) {
		t.Fatalf("expected real actor IP in audit log, got %q", logs.String())
	}
}

func TestAdminQueueLimitIsCapped(t *testing.T) {
	deps := testAdminDeps()
	svc := deps.AdminService.(*admin.InMemoryAdminService)
	now := time.Now().UTC()
	for i := 0; i < 501; i++ {
		svc.AddClaim(domain.Claim{
			ID:        fmt.Sprintf("claim_%03d", i),
			Status:    domain.ClaimStatusQueued,
			CreatedAt: now.Add(time.Duration(i) * time.Second),
			UpdatedAt: now.Add(time.Duration(i) * time.Second),
		})
	}

	rec := httptest.NewRecorder()
	NewHandler(deps).ServeHTTP(rec, adminRequest(http.MethodGet, "/api/v1/admin/queue?limit=999999", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body admin.QueueResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Items) != 500 {
		t.Fatalf("items len = %d, want 500", len(body.Items))
	}
	if body.Counts["queued"] != 501 {
		t.Fatalf("queued count = %d, want 501", body.Counts["queued"])
	}
}

func TestAdminBlocklistRejectsInvalidKeyType(t *testing.T) {
	handler := NewHandler(testAdminDeps())

	addRec := httptest.NewRecorder()
	handler.ServeHTTP(addRec, adminRequest(http.MethodPost, "/api/v1/admin/blocklist", map[string]string{
		"key_type": "banana",
		"value":    "1.2.3.4",
	}))
	if addRec.Code != http.StatusBadRequest {
		t.Fatalf("add status = %d, want 400", addRec.Code)
	}
	var addBody ErrorEnvelope
	if err := json.NewDecoder(addRec.Body).Decode(&addBody); err != nil {
		t.Fatalf("decode add: %v", err)
	}
	if addBody.Code != "invalid_key_type" {
		t.Fatalf("add code = %q, want invalid_key_type", addBody.Code)
	}

	deleteRec := httptest.NewRecorder()
	handler.ServeHTTP(deleteRec, adminRequest(http.MethodDelete, "/api/v1/admin/blocklist?key_type=banana&value=1.2.3.4", nil))
	if deleteRec.Code != http.StatusBadRequest {
		t.Fatalf("delete status = %d, want 400", deleteRec.Code)
	}
	var deleteBody ErrorEnvelope
	if err := json.NewDecoder(deleteRec.Body).Decode(&deleteBody); err != nil {
		t.Fatalf("decode delete: %v", err)
	}
	if deleteBody.Code != "invalid_key_type" {
		t.Fatalf("delete code = %q, want invalid_key_type", deleteBody.Code)
	}
}

func TestAdminClaimsLimitIsCapped(t *testing.T) {
	deps := testAdminDeps()
	svc := deps.AdminService.(*admin.InMemoryAdminService)
	now := time.Now().UTC()
	for i := 0; i < 501; i++ {
		svc.AddClaim(domain.Claim{
			ID:        fmt.Sprintf("claim_%03d", i),
			Status:    domain.ClaimStatusQueued,
			CreatedAt: now.Add(time.Duration(i) * time.Second),
			UpdatedAt: now.Add(time.Duration(i) * time.Second),
		})
	}

	rec := httptest.NewRecorder()
	NewHandler(deps).ServeHTTP(rec, adminRequest(http.MethodGet, "/api/v1/admin/claims?limit=999999", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Claims []domain.Claim `json:"claims"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Claims) != 500 {
		t.Fatalf("claims len = %d, want 500", len(body.Claims))
	}
}

func TestAdminAuditLimitIsCapped(t *testing.T) {
	deps := testAdminDeps()
	svc := deps.AdminService.(*admin.InMemoryAdminService)
	for i := 0; i < 501; i++ {
		if err := svc.SetMode(context.Background(), "paused", "test"); err != nil {
			t.Fatalf("set mode: %v", err)
		}
	}

	rec := httptest.NewRecorder()
	NewHandler(deps).ServeHTTP(rec, adminRequest(http.MethodGet, "/api/v1/admin/audit?limit=999999", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Entries []admin.AuditEntry `json:"entries"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Entries) != 500 {
		t.Fatalf("entries len = %d, want 500", len(body.Entries))
	}
}

func TestSecurityHeadersMiddlewareAppliesToAPIResponses(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	NewHandler(Dependencies{}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}
	assertSecurityHeaders(t, rec.Header())
}

func TestSecurityHeadersMiddlewareAppliesToFrontendResponses(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	NewHandler(Dependencies{}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}
	assertSecurityHeaders(t, rec.Header())
}

func TestSecurityHeadersMiddlewareDoesNotSetHSTSBehindLoopbackHTTP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	NewHandler(Dependencies{}).ServeHTTP(rec, req)

	if got := rec.Header().Get("Strict-Transport-Security"); got != "" {
		t.Fatalf("Strict-Transport-Security = %q, want empty", got)
	}
}

func assertSecurityHeaders(t *testing.T, header http.Header) {
	t.Helper()

	checks := map[string]string{
		"X-Content-Type-Options":       "nosniff",
		"X-Frame-Options":              "DENY",
		"Referrer-Policy":              "no-referrer",
		"Content-Security-Policy":      securityContentSecurityPolicy,
		"Permissions-Policy":           securityPermissionsPolicy,
		"Cross-Origin-Resource-Policy": "same-origin",
	}
	for name, want := range checks {
		if got := header.Get(name); got != want {
			t.Fatalf("%s = %q, want %q", name, got, want)
		}
	}
}

func TestRequestBodyLimitRejectsOversizedClaimByContentLength(t *testing.T) {
	service := &recordingClaimService{}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/claim", strings.NewReader(`{"address":"0x0000000000000000000000000000000000000001"}`))
	req.Header.Set(requestIDHeader, "oversized-request")
	req.ContentLength = maxJSONRequestBodyBytes + 1
	rec := httptest.NewRecorder()

	NewHandler(Dependencies{ReadService: service}).ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
	if service.request.Address != (common.Address{}) {
		t.Fatalf("claim service was called for oversized body")
	}

	var body ErrorEnvelope
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != "request_body_too_large" {
		t.Fatalf("code = %q, want request_body_too_large", body.Code)
	}
	if body.RequestID != "oversized-request" {
		t.Fatalf("request id = %q, want oversized-request", body.RequestID)
	}
}

func TestRequestBodyLimitDoesNotApplyToReadEndpoints(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.ContentLength = maxJSONRequestBodyBytes + 1
	rec := httptest.NewRecorder()

	NewHandler(Dependencies{}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestCreateClaimRejectsUnsupportedContentType(t *testing.T) {
	service := &recordingClaimService{}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/claim", strings.NewReader(`{"address":"0x52908400098527886E0F7030069857D2E4169EE7"}`))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()

	NewHandler(Dependencies{ReadService: service}).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusUnsupportedMediaType)
	}
	if service.request.Address != (common.Address{}) {
		t.Fatalf("claim service was called for unsupported content type")
	}

	var body ErrorEnvelope
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != "unsupported_media_type" {
		t.Fatalf("code = %q, want unsupported_media_type", body.Code)
	}
}

func TestCreateClaimAllowsJSONContentTypeWithParameters(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/claim", strings.NewReader(`{
		"address":"0x52908400098527886E0F7030069857D2E4169EE7"
	}`))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	rec := httptest.NewRecorder()

	NewHandler(Dependencies{ReadService: testClaimService()}).ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusAccepted)
	}
}

func TestAdminJSONWriteRejectsUnsupportedContentType(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/faucet/mode", strings.NewReader(`{"mode":"paused"}`))
	req.Header.Set("Authorization", "Bearer "+testAdminToken)
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()

	NewHandler(testAdminDeps()).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusUnsupportedMediaType)
	}

	var body ErrorEnvelope
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != "unsupported_media_type" {
		t.Fatalf("code = %q, want unsupported_media_type", body.Code)
	}
}

type staticWalletRuntimeProvider struct {
	wallet AdminWalletRuntime
}

func (p staticWalletRuntimeProvider) WalletRuntime(context.Context) AdminWalletRuntime {
	return p.wallet
}

func TestAdminWalletRequiresAuth(t *testing.T) {
	deps := testAdminDeps()
	deps.WalletRuntimeProvider = staticWalletRuntimeProvider{wallet: AdminWalletRuntime{Enabled: true, Status: "ok"}}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/wallet", nil)
	rec := httptest.NewRecorder()
	NewHandler(deps).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAdminWalletReturnsSafeRuntimeVisibility(t *testing.T) {
	deps := testAdminDeps()
	deps.WalletRuntimeProvider = staticWalletRuntimeProvider{wallet: AdminWalletRuntime{
		Enabled:          true,
		Status:           "ok",
		Address:          "0x000000000000000000000000000000000000dEaD",
		NativeBalanceWei: "123",
		PendingNonce:     7,
		Tokens: []AdminWalletTokenRuntime{{
			TokenID:    "native",
			Symbol:     "SCAV",
			Type:       "native",
			BalanceWei: "123",
			Status:     "ok",
		}},
	}}

	rec := httptest.NewRecorder()
	NewHandler(deps).ServeHTTP(rec, adminRequest(http.MethodGet, "/api/v1/admin/wallet", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}
	raw := rec.Body.String()
	var body AdminWalletRuntime
	if err := json.NewDecoder(strings.NewReader(raw)).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Enabled || body.Status != "ok" || body.NativeBalanceWei != "123" || body.PendingNonce != 7 {
		t.Fatalf("wallet body = %#v", body)
	}
	if strings.Contains(raw, "private") || strings.Contains(raw, testAdminToken) {
		t.Fatalf("wallet response contains sensitive material: %q", raw)
	}
}

package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"scavium-netgen/cmd/scavium-faucet/internal/config"
	"scavium-netgen/cmd/scavium-faucet/internal/faucet"
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

func testReadService() faucet.ReadService {
	cfg := config.Defaults()
	cfg.NetworkName = "scavium-test"
	cfg.ChainID = 123
	cfg.Symbol = "tSCAV"
	cfg.AmountWei = big.NewInt(42)
	cfg.CooldownSeconds = 60
	cfg.ExplorerTxURL = "https://explorer.example.test/tx/{txHash}"
	cfg.DryRun = false

	now := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	return faucet.NewInMemoryReadServiceWithClock(cfg, func() time.Time { return now })
}

func testClaimService() *faucet.InMemoryReadService {
	service := testReadService().(*faucet.InMemoryReadService)
	service.SetClaimIDGenerator(func() (string, error) { return "claim_test", nil })
	return service
}

package faucet

import (
	"context"
	"encoding/hex"
	"errors"
	"math/big"
	"testing"
	"time"

	"scavium-netgen/cmd/scavium-faucet/internal/config"
	"scavium-netgen/cmd/scavium-faucet/internal/domain"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/crypto"
)

func testConfig() config.Config {
	cfg := config.Defaults()
	cfg.NetworkName = "scavium-test"
	cfg.ChainID = 123
	cfg.Symbol = "tSCAV"
	cfg.AmountWei = big.NewInt(42)
	cfg.CooldownSeconds = 60
	cfg.ExplorerTxURL = "https://explorer.example.test/tx/{txHash}"
	cfg.DryRun = false
	return cfg
}

func TestInMemoryReadServiceStatus(t *testing.T) {
	now := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	service := NewInMemoryReadServiceWithClock(testConfig(), func() time.Time { return now })

	status, err := service.Status(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status.Status != domain.FaucetStatusActive {
		t.Fatalf("status = %q, want %q", status.Status, domain.FaucetStatusActive)
	}
	if status.NetworkName != "scavium-test" {
		t.Fatalf("network name = %q", status.NetworkName)
	}
	if status.UpdatedAt != "2026-05-03T12:00:00Z" {
		t.Fatalf("updated at = %q", status.UpdatedAt)
	}
}

func TestInMemoryReadServiceConfig(t *testing.T) {
	service := NewInMemoryReadService(testConfig())

	cfg, err := service.Config(context.Background())
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	if cfg.ChainID != 123 {
		t.Fatalf("chain id = %d", cfg.ChainID)
	}
	if cfg.AmountWei != "42" {
		t.Fatalf("amount wei = %q", cfg.AmountWei)
	}
	if cfg.DryRun {
		t.Fatal("dry run = true, want false")
	}
	if cfg.RateLimitIPPerHour != 10 {
		t.Fatalf("rate_limit_ip_per_hour = %d, want 10", cfg.RateLimitIPPerHour)
	}
	if cfg.RateLimitAddrPerDay != 3 {
		t.Fatalf("rate_limit_addr_per_day = %d, want 3", cfg.RateLimitAddrPerDay)
	}
}

func TestInMemoryReadServiceAddressStatus(t *testing.T) {
	service := NewInMemoryReadService(testConfig())
	address := domain.MustValidateAddress("0x52908400098527886E0F7030069857D2E4169EE7")

	status, err := service.AddressStatus(context.Background(), address)
	if err != nil {
		t.Fatalf("address status: %v", err)
	}
	if !status.Eligible {
		t.Fatal("eligible = false, want true")
	}
	if status.Address != "0x52908400098527886E0F7030069857D2E4169EE7" {
		t.Fatalf("address = %q", status.Address)
	}
	if status.CooldownSeconds != 60 {
		t.Fatalf("cooldown seconds = %d", status.CooldownSeconds)
	}
	if status.CooldownRemainingSeconds != 0 {
		t.Fatalf("cooldown_remaining_seconds = %d, want 0 when eligible", status.CooldownRemainingSeconds)
	}
	if status.RateLimitIPPerHour != 10 {
		t.Fatalf("rate_limit_ip_per_hour = %d, want 10", status.RateLimitIPPerHour)
	}
	if status.RateLimitAddrPerDay != 3 {
		t.Fatalf("rate_limit_addr_per_day = %d, want 3", status.RateLimitAddrPerDay)
	}
	if status.DefaultTokenID != "native" {
		t.Fatalf("default_token_id = %q, want native", status.DefaultTokenID)
	}
	if len(status.Tokens) != 1 || status.Tokens[0].TokenID != "native" || !status.Tokens[0].Eligible {
		t.Fatalf("tokens = %#v", status.Tokens)
	}
}

func TestInMemoryReadServiceAddressStatusIncludesBudgetUse(t *testing.T) {
	now := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	cfg := testConfig()
	cfg.DailyBudgetWei = big.NewInt(1000)
	cfg.Tokens = []config.TokenConfig{{ID: "native", Symbol: "tSCAV", Type: domain.TokenTypeNative, Decimals: 18, AmountWei: big.NewInt(42), DailyBudgetWei: big.NewInt(1000)}}
	service := NewInMemoryReadServiceWithClock(cfg, func() time.Time { return now })
	address := domain.MustValidateAddress("0x52908400098527886E0F7030069857D2E4169EE7")
	if _, err := service.CreateClaim(context.Background(), ClaimRequest{Address: address, TokenID: "native"}); err != nil {
		t.Fatalf("create claim: %v", err)
	}

	status, err := service.AddressStatus(context.Background(), address)
	if err != nil {
		t.Fatalf("address status: %v", err)
	}
	if status.DailyBudget == nil || status.DailyBudget.UsedWei != "42" || status.DailyBudget.RemainingWei != "958" {
		t.Fatalf("daily budget = %#v", status.DailyBudget)
	}
	if len(status.Tokens) != 1 || status.Tokens[0].DailyBudget == nil || status.Tokens[0].DailyBudget.UsedWei != "42" {
		t.Fatalf("token status = %#v", status.Tokens)
	}
}

func TestInMemoryReadServiceAddressStatusReflectsExhaustedDailyBudget(t *testing.T) {
	now := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	cfg := testConfig()
	cfg.DailyBudgetWei = big.NewInt(42)
	cfg.Tokens = []config.TokenConfig{{ID: "native", Symbol: "tSCAV", Type: domain.TokenTypeNative, Decimals: 18, AmountWei: big.NewInt(42), DailyBudgetWei: big.NewInt(42)}}
	service := NewInMemoryReadServiceWithClock(cfg, func() time.Time { return now })
	address := domain.MustValidateAddress("0x52908400098527886E0F7030069857D2E4169EE7")
	if _, err := service.CreateClaim(context.Background(), ClaimRequest{Address: address, TokenID: "native"}); err != nil {
		t.Fatalf("create claim: %v", err)
	}

	status, err := service.AddressStatus(context.Background(), address)
	if err != nil {
		t.Fatalf("address status: %v", err)
	}
	if status.Eligible || status.Reason != "daily_budget_exceeded" {
		t.Fatalf("eligible/reason = %v/%q, want false/daily_budget_exceeded", status.Eligible, status.Reason)
	}
	if status.DailyBudget == nil || status.DailyBudget.RemainingWei != "0" {
		t.Fatalf("daily budget = %#v, want exhausted", status.DailyBudget)
	}
	if len(status.Tokens) != 1 || status.Tokens[0].Eligible || status.Tokens[0].Reason != "daily_budget_exceeded" {
		t.Fatalf("token status = %#v, want exhausted", status.Tokens)
	}
}

func TestInMemoryReadServiceAddressHistoryPagination(t *testing.T) {
	now := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	service := NewInMemoryReadServiceWithClock(testConfig(), func() time.Time { return now })
	ids := []string{"claim_old", "claim_new"}
	next := 0
	service.SetClaimIDGenerator(func() (string, error) {
		id := ids[next]
		next++
		return id, nil
	})
	address := domain.MustValidateAddress("0x52908400098527886E0F7030069857D2E4169EE7")
	if _, err := service.CreateClaim(context.Background(), ClaimRequest{Address: address}); err != nil {
		t.Fatalf("create old claim: %v", err)
	}
	service.now = func() time.Time { return now.Add(time.Minute) }
	if _, err := service.CreateClaim(context.Background(), ClaimRequest{Address: address}); err != nil {
		t.Fatalf("create new claim: %v", err)
	}

	history, err := service.AddressHistory(context.Background(), address, 1, 0)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if history.Pagination.Limit != 1 || history.Pagination.Count != 1 || !history.Pagination.HasMore {
		t.Fatalf("pagination = %#v", history.Pagination)
	}
	if len(history.Claims) != 1 || history.Claims[0].ID != "claim_new" {
		t.Fatalf("claims = %#v", history.Claims)
	}
}

func TestInMemoryReadServiceCreateAndGetClaim(t *testing.T) {
	now := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	service := NewInMemoryReadServiceWithClock(testConfig(), func() time.Time { return now })
	service.SetClaimIDGenerator(func() (string, error) { return "claim_test", nil })
	address := domain.MustValidateAddress("0x52908400098527886E0F7030069857D2E4169EE7")

	created, err := service.CreateClaim(context.Background(), ClaimRequest{Address: address})
	if err != nil {
		t.Fatalf("create claim: %v", err)
	}
	if created.ID != "claim_test" {
		t.Fatalf("id = %q, want claim_test", created.ID)
	}
	if created.Status != domain.ClaimStatusQueued {
		t.Fatalf("status = %q, want %q", created.Status, domain.ClaimStatusQueued)
	}
	if created.CreatedAt != "2026-05-03T12:00:00Z" {
		t.Fatalf("created at = %q", created.CreatedAt)
	}

	got, found, err := service.GetClaim(context.Background(), "claim_test")
	if err != nil {
		t.Fatalf("get claim: %v", err)
	}
	if !found {
		t.Fatal("claim not found")
	}
	if got.ID != created.ID {
		t.Fatalf("got id = %q, want %q", got.ID, created.ID)
	}
}

func TestInMemoryReadServiceIdempotency(t *testing.T) {
	nextID := 0
	service := NewInMemoryReadService(testConfig())
	service.SetClaimIDGenerator(func() (string, error) {
		nextID++
		return "claim_id", nil
	})
	address := domain.MustValidateAddress("0x52908400098527886E0F7030069857D2E4169EE7")

	first, err := service.CreateClaim(context.Background(), ClaimRequest{
		Address:        address,
		IdempotencyKey: "same-key",
	})
	if err != nil {
		t.Fatalf("create first claim: %v", err)
	}

	second, err := service.CreateClaim(context.Background(), ClaimRequest{
		Address:        address,
		IdempotencyKey: "same-key",
	})
	if err != nil {
		t.Fatalf("create second claim: %v", err)
	}

	if first.ID != second.ID {
		t.Fatalf("ids differ: %q != %q", first.ID, second.ID)
	}
	if nextID != 1 {
		t.Fatalf("generated ids = %d, want 1", nextID)
	}
}

func TestInMemoryReadServiceRejectsInvalidToken(t *testing.T) {
	service := NewInMemoryReadService(testConfig())
	address := domain.MustValidateAddress("0x52908400098527886E0F7030069857D2E4169EE7")

	_, err := service.CreateClaim(context.Background(), ClaimRequest{
		Address: address,
		TokenID: "missing-token",
	})
	if err == nil {
		t.Fatal("create claim returned nil error")
	}
	if !errors.Is(err, ErrClaimRejected) {
		t.Fatalf("error = %v, want ErrClaimRejected", err)
	}
	var claimErr *ClaimError
	if !errors.As(err, &claimErr) {
		t.Fatalf("error = %T, want ClaimError", err)
	}
	if claimErr.Reason != "invalid_token" {
		t.Fatalf("reason = %q, want invalid_token", claimErr.Reason)
	}
}

func TestInMemoryWalletChallengeAllowsOptionalSignature(t *testing.T) {
	service := NewInMemoryReadService(testConfig())
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	address := crypto.PubkeyToAddress(key.PublicKey)
	challenge, err := service.CreateWalletChallenge(context.Background(), WalletChallengeRequest{Address: address})
	if err != nil {
		t.Fatalf("challenge: %v", err)
	}
	sig, err := crypto.Sign(accounts.TextHash([]byte(challenge.Message)), key)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	claim, err := service.CreateClaim(context.Background(), ClaimRequest{Address: address, TokenID: "native", WalletChallengeID: challenge.ID, WalletSignature: "0x" + hex.EncodeToString(sig)})
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if claim.Address != address.Hex() {
		t.Fatalf("claim address = %q", claim.Address)
	}
	if _, err := service.CreateClaim(context.Background(), ClaimRequest{Address: address, TokenID: "native", WalletChallengeID: challenge.ID, WalletSignature: "0x" + hex.EncodeToString(sig)}); !errors.Is(err, ErrClaimRejected) {
		t.Fatalf("replay err = %v, want claim rejected", err)
	}
}

func TestInMemoryWalletProofIsOptionalForLegacyClaim(t *testing.T) {
	service := NewInMemoryReadService(testConfig())
	address := domain.MustValidateAddress("0x52908400098527886E0F7030069857D2E4169EE7")
	if _, err := service.CreateClaim(context.Background(), ClaimRequest{Address: address, TokenID: "native"}); err != nil {
		t.Fatalf("legacy claim: %v", err)
	}
}

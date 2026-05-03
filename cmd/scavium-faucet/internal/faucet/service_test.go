package faucet

import (
	"context"
	"math/big"
	"testing"
	"time"

	"scavium-netgen/cmd/scavium-faucet/internal/config"
	"scavium-netgen/cmd/scavium-faucet/internal/domain"
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

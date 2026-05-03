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
}

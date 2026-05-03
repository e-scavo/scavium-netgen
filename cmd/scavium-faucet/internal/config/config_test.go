package config

import (
	"math/big"
	"strings"
	"testing"
)

func TestFromEnvUsesDevelopmentDefaults(t *testing.T) {
	cfg, err := FromEnv(func(string) string { return "" })
	if err != nil {
		t.Fatalf("from env: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate defaults: %v", err)
	}

	if cfg.BindAddr != "127.0.0.1:18080" {
		t.Fatalf("bind addr = %q", cfg.BindAddr)
	}
	if cfg.RPCURL != "http://127.0.0.1:18545" {
		t.Fatalf("rpc url = %q", cfg.RPCURL)
	}
	if cfg.ChainID != 31337 {
		t.Fatalf("chain id = %d", cfg.ChainID)
	}
	if !cfg.DryRun {
		t.Fatal("dry run default = false, want true")
	}
}

func TestFromEnvOverridesValues(t *testing.T) {
	values := map[string]string{
		EnvBindAddr:            "127.0.0.1:19090",
		EnvPublicBaseURL:       "https://faucet.example.test",
		EnvRPCURL:              "http://127.0.0.1:28545",
		EnvChainID:             "999",
		EnvNetworkName:         "scavium-test",
		EnvSymbol:              "tSCAV",
		EnvExplorerTxURL:       "https://explorer.example.test/tx/{txHash}",
		EnvAmountWei:           "42",
		EnvCooldownSeconds:     "60",
		EnvDryRun:              "false",
		EnvRateLimitIPPerHour:  "20",
		EnvRateLimitAddrPerDay: "5",
		EnvDailyBudgetWei:      "9999",
		EnvTrustedProxy:        "127.0.0.1",
	}

	cfg, err := FromEnv(func(key string) string { return values[key] })
	if err != nil {
		t.Fatalf("from env: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate config: %v", err)
	}

	if cfg.BindAddr != values[EnvBindAddr] {
		t.Fatalf("bind addr = %q", cfg.BindAddr)
	}
	if cfg.PublicBaseURL != values[EnvPublicBaseURL] {
		t.Fatalf("public base url = %q", cfg.PublicBaseURL)
	}
	if cfg.RPCURL != values[EnvRPCURL] {
		t.Fatalf("rpc url = %q", cfg.RPCURL)
	}
	if cfg.ChainID != 999 {
		t.Fatalf("chain id = %d", cfg.ChainID)
	}
	if cfg.NetworkName != values[EnvNetworkName] {
		t.Fatalf("network name = %q", cfg.NetworkName)
	}
	if cfg.Symbol != values[EnvSymbol] {
		t.Fatalf("symbol = %q", cfg.Symbol)
	}
	if cfg.ExplorerTxURL != values[EnvExplorerTxURL] {
		t.Fatalf("explorer tx url = %q", cfg.ExplorerTxURL)
	}
	if cfg.AmountWei.Cmp(big.NewInt(42)) != 0 {
		t.Fatalf("amount wei = %s", cfg.AmountWei.String())
	}
	if cfg.CooldownSeconds != 60 {
		t.Fatalf("cooldown seconds = %d", cfg.CooldownSeconds)
	}
	if cfg.DryRun {
		t.Fatal("dry run = true, want false")
	}
	if cfg.RateLimitIPPerHour != 20 {
		t.Fatalf("rate limit ip per hour = %d", cfg.RateLimitIPPerHour)
	}
	if cfg.RateLimitAddrPerDay != 5 {
		t.Fatalf("rate limit addr per day = %d", cfg.RateLimitAddrPerDay)
	}
	if cfg.DailyBudgetWei == nil || cfg.DailyBudgetWei.Cmp(big.NewInt(9999)) != 0 {
		t.Fatalf("daily budget wei = %v", cfg.DailyBudgetWei)
	}
	if cfg.TrustedProxy != "127.0.0.1" {
		t.Fatalf("trusted proxy = %q", cfg.TrustedProxy)
	}
}

func TestFromEnvRateLimitDefaults(t *testing.T) {
	cfg, err := FromEnv(func(string) string { return "" })
	if err != nil {
		t.Fatalf("from env: %v", err)
	}
	if cfg.RateLimitIPPerHour != 10 {
		t.Fatalf("default rate limit ip per hour = %d", cfg.RateLimitIPPerHour)
	}
	if cfg.RateLimitAddrPerDay != 3 {
		t.Fatalf("default rate limit addr per day = %d", cfg.RateLimitAddrPerDay)
	}
	if cfg.DailyBudgetWei != nil {
		t.Fatalf("default daily budget wei = %v, want nil", cfg.DailyBudgetWei)
	}
	if cfg.TrustedProxy != "" {
		t.Fatalf("default trusted proxy = %q, want empty", cfg.TrustedProxy)
	}
}

func TestFromEnvRejectsInvalidNumbers(t *testing.T) {
	_, err := FromEnv(func(key string) string {
		if key == EnvChainID {
			return "not-a-number"
		}
		return ""
	})
	if err == nil || !strings.Contains(err.Error(), EnvChainID) {
		t.Fatalf("error = %v, want %s parse error", err, EnvChainID)
	}

	_, err = FromEnv(func(key string) string {
		if key == EnvAmountWei {
			return "nope"
		}
		return ""
	})
	if err == nil || !strings.Contains(err.Error(), EnvAmountWei) {
		t.Fatalf("error = %v, want %s parse error", err, EnvAmountWei)
	}
}

func TestValidateRejectsCriticalEmptyValues(t *testing.T) {
	cfg := Defaults()
	cfg.BindAddr = ""
	cfg.RPCURL = ""
	cfg.ChainID = 0
	cfg.AmountWei = big.NewInt(0)

	err := cfg.Validate()
	if err == nil {
		t.Fatal("validate returned nil")
	}

	for _, want := range []string{
		"bind address is required",
		"RPC URL is required",
		"chain ID must be positive",
		"amount wei must be positive",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want to contain %q", err.Error(), want)
		}
	}
}

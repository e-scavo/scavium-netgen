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
	if cfg.DatabasePath != "cmd/scavium-faucet/data/scavium-faucet.db" {
		t.Fatalf("database path = %q", cfg.DatabasePath)
	}
	if cfg.AbuseSignalRetentionDays != 30 {
		t.Fatalf("abuse signal retention days = %d", cfg.AbuseSignalRetentionDays)
	}
	if !cfg.WorkerEnabled {
		t.Fatal("worker enabled default = false, want true")
	}
	if cfg.WorkerPollSeconds != 5 {
		t.Fatalf("worker poll seconds = %d", cfg.WorkerPollSeconds)
	}
	if cfg.WatcherEnabled {
		t.Fatal("watcher enabled default = true, want false in dry-run")
	}
	if cfg.WatcherPollSeconds != 15 {
		t.Fatalf("watcher poll seconds = %d", cfg.WatcherPollSeconds)
	}
	if cfg.MinConfirmations != 1 {
		t.Fatalf("min confirmations = %d", cfg.MinConfirmations)
	}
}

func TestFromEnvOverridesValues(t *testing.T) {
	values := map[string]string{
		EnvBindAddr:                             "127.0.0.1:19090",
		EnvPublicBaseURL:                        "https://faucet.example.test",
		EnvRPCURL:                               "http://127.0.0.1:28545",
		EnvChainID:                              "999",
		EnvNetworkName:                          "scavium-test",
		EnvSymbol:                               "tSCAV",
		EnvExplorerTxURL:                        "https://explorer.example.test/tx/{txHash}",
		EnvAmountWei:                            "42",
		EnvCooldownSeconds:                      "60",
		EnvDryRun:                               "false",
		EnvDatabasePath:                         "/tmp/scavium-faucet-test.db",
		EnvRateLimitIPPerHour:                   "20",
		EnvRateLimitAddrPerDay:                  "5",
		EnvDailyBudgetWei:                       "9999",
		EnvAbuseEnforcementEnabled:              "false",
		EnvAbuseEnforcementWindowSeconds:        "1800",
		EnvAbuseEnforcementIPThreshold:          "9",
		EnvAbuseEnforcementAddressThreshold:     "8",
		EnvAbuseEnforcementFingerprintThreshold: "7",
		EnvAbuseSignalRetentionDays:             "14",
		EnvTrustedProxy:                         "127.0.0.1",
		EnvCORSAllowedOrigins:                   "https://faucet.example.test, https://wallet.example.test ",
		EnvWorkerEnabled:                        "false",
		EnvWorkerPollSeconds:                    "7",
		EnvWatcherEnabled:                       "true",
		EnvWatcherPollSeconds:                   "21",
		EnvMinConfirmations:                     "3",
		EnvPrivateKey:                           "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
		EnvCaptchaProvider:                      "hcaptcha",
		EnvCaptchaSiteKey:                       "10000000-ffff-ffff-ffff-000000000001",
		EnvCaptchaSecret:                        "0x-test-secret",
		EnvCaptchaVerifyURL:                     "https://hcaptcha.example.test/siteverify",
		EnvFaucetMode:                           "paused",
		EnvAdminToken:                           "test-admin-token-xyz",
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
	if cfg.DatabasePath != "/tmp/scavium-faucet-test.db" {
		t.Fatalf("database path = %q", cfg.DatabasePath)
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
	if cfg.AbuseEnforcementEnabled {
		t.Fatal("abuse enforcement enabled = true, want false")
	}
	if cfg.AbuseEnforcementWindowSeconds != 1800 {
		t.Fatalf("abuse enforcement window seconds = %d", cfg.AbuseEnforcementWindowSeconds)
	}
	if cfg.AbuseEnforcementIPThreshold != 9 || cfg.AbuseEnforcementAddressThreshold != 8 || cfg.AbuseEnforcementFingerprintThreshold != 7 {
		t.Fatalf("abuse enforcement thresholds = ip:%d address:%d fingerprint:%d", cfg.AbuseEnforcementIPThreshold, cfg.AbuseEnforcementAddressThreshold, cfg.AbuseEnforcementFingerprintThreshold)
	}
	if cfg.AbuseSignalRetentionDays != 14 {
		t.Fatalf("abuse signal retention days = %d", cfg.AbuseSignalRetentionDays)
	}
	if cfg.TrustedProxy != "127.0.0.1" {
		t.Fatalf("trusted proxy = %q", cfg.TrustedProxy)
	}
	if len(cfg.CORSAllowedOrigins) != 2 ||
		cfg.CORSAllowedOrigins[0] != "https://faucet.example.test" ||
		cfg.CORSAllowedOrigins[1] != "https://wallet.example.test" {
		t.Fatalf("cors allowed origins = %#v", cfg.CORSAllowedOrigins)
	}
	if cfg.PrivateKeyHex != "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80" {
		t.Fatal("private key hex not set")
	}
	if cfg.WorkerEnabled {
		t.Fatal("worker enabled = true, want false")
	}
	if cfg.WorkerPollSeconds != 7 {
		t.Fatalf("worker poll seconds = %d", cfg.WorkerPollSeconds)
	}
	if !cfg.WatcherEnabled {
		t.Fatal("watcher enabled = false, want true")
	}
	if cfg.WatcherPollSeconds != 21 {
		t.Fatalf("watcher poll seconds = %d", cfg.WatcherPollSeconds)
	}
	if cfg.MinConfirmations != 3 {
		t.Fatalf("min confirmations = %d", cfg.MinConfirmations)
	}
	if cfg.CaptchaProvider != "hcaptcha" {
		t.Fatalf("captcha provider = %q, want hcaptcha", cfg.CaptchaProvider)
	}
	if cfg.CaptchaSiteKey != "10000000-ffff-ffff-ffff-000000000001" {
		t.Fatalf("captcha site key = %q", cfg.CaptchaSiteKey)
	}
	if cfg.CaptchaSecret != "0x-test-secret" {
		t.Fatal("captcha secret not set")
	}
	if cfg.CaptchaVerifyURL != "https://hcaptcha.example.test/siteverify" {
		t.Fatalf("captcha verify url = %q", cfg.CaptchaVerifyURL)
	}
	if cfg.FaucetMode != "paused" {
		t.Fatalf("faucet mode = %q, want paused", cfg.FaucetMode)
	}
	if cfg.AdminToken != "test-admin-token-xyz" {
		t.Fatal("admin token not set")
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
	if len(cfg.CORSAllowedOrigins) != 0 {
		t.Fatalf("default CORS allowed origins = %#v, want empty", cfg.CORSAllowedOrigins)
	}
	if cfg.CaptchaProvider != "disabled" {
		t.Fatalf("default captcha provider = %q, want disabled", cfg.CaptchaProvider)
	}
	if cfg.FaucetMode != "active" {
		t.Fatalf("default faucet mode = %q, want active", cfg.FaucetMode)
	}
}

func TestValidateRejectsWildcardCORSOrigin(t *testing.T) {
	cfg := Defaults()
	cfg.CORSAllowedOrigins = []string{"https://faucet.example.test", "*"}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("validate returned nil")
	}
	if !strings.Contains(err.Error(), "CORS allowed origins must not contain wildcard") {
		t.Fatalf("error = %q, want CORS wildcard validation", err.Error())
	}
}

func TestFromEnvEnablesWatcherByDefaultWhenNotDryRun(t *testing.T) {
	cfg, err := FromEnv(func(key string) string {
		if key == EnvDryRun {
			return "false"
		}
		return ""
	})
	if err != nil {
		t.Fatalf("from env: %v", err)
	}
	if !cfg.WatcherEnabled {
		t.Fatal("watcher enabled = false, want true when not dry-run")
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

func TestValidateRejectsEmptyDatabasePath(t *testing.T) {
	cfg := Defaults()
	cfg.DatabasePath = " "

	err := cfg.Validate()
	if err == nil {
		t.Fatal("validate returned nil")
	}
	if !strings.Contains(err.Error(), "database path is required") {
		t.Fatalf("error = %q, want database path validation", err.Error())
	}
}

func TestValidateRejectsCaptchaProviderWithoutSiteKeyOrSecret(t *testing.T) {
	cfg := Defaults()
	cfg.CaptchaProvider = "turnstile"

	err := cfg.Validate()
	if err == nil {
		t.Fatal("validate returned nil")
	}
	for _, want := range []string{
		`captcha provider "turnstile" requires site key`,
		`captcha provider "turnstile" requires secret`,
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want %q", err.Error(), want)
		}
	}
}

package config

import (
	"errors"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	EnvBindAddr            = "SCAVIUM_FAUCET_BIND_ADDR"
	EnvPublicBaseURL       = "SCAVIUM_FAUCET_PUBLIC_BASE_URL"
	EnvRPCURL              = "SCAVIUM_FAUCET_RPC_URL"
	EnvChainID             = "SCAVIUM_FAUCET_CHAIN_ID"
	EnvNetworkName         = "SCAVIUM_FAUCET_NETWORK_NAME"
	EnvSymbol              = "SCAVIUM_FAUCET_SYMBOL"
	EnvExplorerTxURL       = "SCAVIUM_FAUCET_EXPLORER_TX_URL"
	EnvAmountWei           = "SCAVIUM_FAUCET_AMOUNT_WEI"
	EnvCooldownSeconds     = "SCAVIUM_FAUCET_COOLDOWN_SECONDS"
	EnvDryRun              = "SCAVIUM_FAUCET_DRY_RUN"
	EnvRateLimitIPPerHour  = "SCAVIUM_FAUCET_RATE_LIMIT_IP_PER_HOUR"
	EnvRateLimitAddrPerDay = "SCAVIUM_FAUCET_RATE_LIMIT_ADDR_PER_DAY"
	EnvDailyBudgetWei      = "SCAVIUM_FAUCET_DAILY_BUDGET_WEI"
	EnvTrustedProxy        = "SCAVIUM_FAUCET_TRUSTED_PROXY"
	// EnvPrivateKey holds the hex-encoded private key used to sign transactions.
	// Never log this value.
	EnvPrivateKey = "SCAVIUM_FAUCET_PRIVATE_KEY"

	// Captcha configuration.
	// EnvCaptchaProvider selects the captcha backend: "disabled", "dev",
	// "hcaptcha", "recaptcha", or "turnstile".
	EnvCaptchaProvider  = "SCAVIUM_FAUCET_CAPTCHA_PROVIDER"
	EnvCaptchaSecret    = "SCAVIUM_FAUCET_CAPTCHA_SECRET"
	EnvCaptchaVerifyURL = "SCAVIUM_FAUCET_CAPTCHA_VERIFY_URL"

	// EnvFaucetMode controls the operational mode: "active", "paused", or
	// "maintenance".
	EnvFaucetMode = "SCAVIUM_FAUCET_MODE"

	// EnvAdminToken is the bearer token required to access admin endpoints.
	// If empty, admin endpoints are disabled.  Never log this value.
	EnvAdminToken = "SCAVIUM_FAUCET_ADMIN_TOKEN"
)

type Config struct {
	BindAddr            string
	PublicBaseURL       string
	RPCURL              string
	ChainID             int64
	NetworkName         string
	Symbol              string
	ExplorerTxURL       string
	AmountWei           *big.Int
	CooldownSeconds     int
	DryRun              bool
	RateLimitIPPerHour  int
	RateLimitAddrPerDay int
	DailyBudgetWei      *big.Int
	TrustedProxy        string
	// PrivateKeyHex is the hex-encoded signer key. Not required when DryRun=true.
	// Never log this value.
	PrivateKeyHex string

	// Captcha settings.
	// CaptchaProvider selects the backend.  Defaults to "disabled".
	CaptchaProvider string
	// CaptchaSecret is the server-side secret for the chosen provider.
	// Never log this value.
	CaptchaSecret    string
	CaptchaVerifyURL string

	// FaucetMode is the operational mode of the faucet ("active", "paused", or
	// "maintenance").  Defaults to "active".
	FaucetMode string

	// AdminToken is the bearer token for admin endpoints.
	// Empty means admin API is disabled.  Never log this value.
	AdminToken string
}

func LoadFromEnv() (Config, error) {
	cfg, err := FromEnv(getenv)
	if err != nil {
		return Config{}, err
	}
	return cfg, cfg.Validate()
}

func FromEnv(lookup func(string) string) (Config, error) {
	cfg := Defaults()

	cfg.BindAddr = envOrDefault(lookup, EnvBindAddr, cfg.BindAddr)
	cfg.PublicBaseURL = envOrDefault(lookup, EnvPublicBaseURL, cfg.PublicBaseURL)
	cfg.RPCURL = envOrDefault(lookup, EnvRPCURL, cfg.RPCURL)
	cfg.NetworkName = envOrDefault(lookup, EnvNetworkName, cfg.NetworkName)
	cfg.Symbol = envOrDefault(lookup, EnvSymbol, cfg.Symbol)
	cfg.ExplorerTxURL = envOrDefault(lookup, EnvExplorerTxURL, cfg.ExplorerTxURL)

	if raw := strings.TrimSpace(lookup(EnvChainID)); raw != "" {
		chainID, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return Config{}, fmt.Errorf("%s: %w", EnvChainID, err)
		}
		cfg.ChainID = chainID
	}

	if raw := strings.TrimSpace(lookup(EnvAmountWei)); raw != "" {
		amount, ok := new(big.Int).SetString(raw, 10)
		if !ok {
			return Config{}, fmt.Errorf("%s: invalid integer", EnvAmountWei)
		}
		cfg.AmountWei = amount
	}

	if raw := strings.TrimSpace(lookup(EnvCooldownSeconds)); raw != "" {
		cooldown, err := strconv.Atoi(raw)
		if err != nil {
			return Config{}, fmt.Errorf("%s: %w", EnvCooldownSeconds, err)
		}
		cfg.CooldownSeconds = cooldown
	}

	if raw := strings.TrimSpace(lookup(EnvDryRun)); raw != "" {
		dryRun, err := strconv.ParseBool(raw)
		if err != nil {
			return Config{}, fmt.Errorf("%s: %w", EnvDryRun, err)
		}
		cfg.DryRun = dryRun
	}

	if raw := strings.TrimSpace(lookup(EnvRateLimitIPPerHour)); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil {
			return Config{}, fmt.Errorf("%s: %w", EnvRateLimitIPPerHour, err)
		}
		cfg.RateLimitIPPerHour = v
	}

	if raw := strings.TrimSpace(lookup(EnvRateLimitAddrPerDay)); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil {
			return Config{}, fmt.Errorf("%s: %w", EnvRateLimitAddrPerDay, err)
		}
		cfg.RateLimitAddrPerDay = v
	}

	if raw := strings.TrimSpace(lookup(EnvDailyBudgetWei)); raw != "" {
		v, ok := new(big.Int).SetString(raw, 10)
		if !ok {
			return Config{}, fmt.Errorf("%s: invalid integer", EnvDailyBudgetWei)
		}
		cfg.DailyBudgetWei = v
	}

	cfg.TrustedProxy = strings.TrimSpace(lookup(EnvTrustedProxy))
	cfg.PrivateKeyHex = strings.TrimSpace(lookup(EnvPrivateKey))

	cfg.CaptchaProvider = envOrDefault(lookup, EnvCaptchaProvider, cfg.CaptchaProvider)
	cfg.CaptchaSecret = strings.TrimSpace(lookup(EnvCaptchaSecret))
	cfg.CaptchaVerifyURL = envOrDefault(lookup, EnvCaptchaVerifyURL, cfg.CaptchaVerifyURL)
	cfg.FaucetMode = envOrDefault(lookup, EnvFaucetMode, cfg.FaucetMode)
	cfg.AdminToken = strings.TrimSpace(lookup(EnvAdminToken))

	return cfg, nil
}

func Defaults() Config {
	return Config{
		BindAddr:            "127.0.0.1:18080",
		PublicBaseURL:       "http://127.0.0.1:18080",
		RPCURL:              "http://127.0.0.1:18545",
		ChainID:             31337,
		NetworkName:         "scavium-dev",
		Symbol:              "SCAV",
		ExplorerTxURL:       "",
		AmountWei:           big.NewInt(1_000_000_000_000_000_000),
		CooldownSeconds:     int((24 * time.Hour).Seconds()),
		DryRun:              true,
		RateLimitIPPerHour:  10,
		RateLimitAddrPerDay: 3,
		DailyBudgetWei:      nil,
		TrustedProxy:        "",
		CaptchaProvider:     "disabled",
		CaptchaVerifyURL:    "",
		FaucetMode:          "active",
	}
}

func (c Config) Validate() error {
	var errs []error

	if strings.TrimSpace(c.BindAddr) == "" {
		errs = append(errs, errors.New("bind address is required"))
	}
	if strings.TrimSpace(c.PublicBaseURL) == "" {
		errs = append(errs, errors.New("public base URL is required"))
	}
	if strings.TrimSpace(c.RPCURL) == "" {
		errs = append(errs, errors.New("RPC URL is required"))
	}
	if c.ChainID <= 0 {
		errs = append(errs, errors.New("chain ID must be positive"))
	}
	if strings.TrimSpace(c.NetworkName) == "" {
		errs = append(errs, errors.New("network name is required"))
	}
	if strings.TrimSpace(c.Symbol) == "" {
		errs = append(errs, errors.New("symbol is required"))
	}
	if c.AmountWei == nil || c.AmountWei.Sign() <= 0 {
		errs = append(errs, errors.New("amount wei must be positive"))
	}
	if c.CooldownSeconds < 0 {
		errs = append(errs, errors.New("cooldown seconds must be zero or positive"))
	}

	return errors.Join(errs...)
}

func getenv(key string) string {
	return os.Getenv(key)
}

func envOrDefault(lookup func(string) string, key, fallback string) string {
	if value := strings.TrimSpace(lookup(key)); value != "" {
		return value
	}
	return fallback
}

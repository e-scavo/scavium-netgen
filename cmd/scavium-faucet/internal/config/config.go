// Package config loads and validates faucet runtime configuration.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"
	"time"

	"scavium-netgen/cmd/scavium-faucet/internal/domain"

	"github.com/ethereum/go-ethereum/common"
)

// Environment variable names consumed by the faucet configuration loader.
const (
	EnvBindAddr                             = "SCAVIUM_FAUCET_BIND_ADDR"
	EnvPublicBaseURL                        = "SCAVIUM_FAUCET_PUBLIC_BASE_URL"
	EnvRPCURL                               = "SCAVIUM_FAUCET_RPC_URL"
	EnvRPCSecondaryURLs                     = "SCAVIUM_FAUCET_RPC_SECONDARY_URLS"
	EnvChainID                              = "SCAVIUM_FAUCET_CHAIN_ID"
	EnvNetworkName                          = "SCAVIUM_FAUCET_NETWORK_NAME"
	EnvSymbol                               = "SCAVIUM_FAUCET_SYMBOL"
	EnvExplorerTxURL                        = "SCAVIUM_FAUCET_EXPLORER_TX_URL"
	EnvAmountWei                            = "SCAVIUM_FAUCET_AMOUNT_WEI"
	EnvTokensJSON                           = "SCAVIUM_FAUCET_TOKENS_JSON"
	EnvDefaultTokenID                       = "SCAVIUM_FAUCET_DEFAULT_TOKEN_ID"
	EnvCooldownSeconds                      = "SCAVIUM_FAUCET_COOLDOWN_SECONDS"
	EnvDryRun                               = "SCAVIUM_FAUCET_DRY_RUN"
	EnvDatabasePath                         = "SCAVIUM_FAUCET_DATABASE_PATH"
	EnvRateLimitIPPerHour                   = "SCAVIUM_FAUCET_RATE_LIMIT_IP_PER_HOUR"
	EnvRateLimitAddrPerDay                  = "SCAVIUM_FAUCET_RATE_LIMIT_ADDR_PER_DAY"
	EnvDailyBudgetWei                       = "SCAVIUM_FAUCET_DAILY_BUDGET_WEI"
	EnvAbuseEnforcementEnabled              = "SCAVIUM_FAUCET_ABUSE_ENFORCEMENT_ENABLED"
	EnvAbuseEnforcementWindowSeconds        = "SCAVIUM_FAUCET_ABUSE_ENFORCEMENT_WINDOW_SECONDS"
	EnvAbuseEnforcementIPThreshold          = "SCAVIUM_FAUCET_ABUSE_ENFORCEMENT_IP_THRESHOLD"
	EnvAbuseEnforcementAddressThreshold     = "SCAVIUM_FAUCET_ABUSE_ENFORCEMENT_ADDRESS_THRESHOLD"
	EnvAbuseEnforcementFingerprintThreshold = "SCAVIUM_FAUCET_ABUSE_ENFORCEMENT_FINGERPRINT_THRESHOLD"
	EnvAbuseSignalRetentionDays             = "SCAVIUM_FAUCET_ABUSE_SIGNAL_RETENTION_DAYS"
	EnvTrustedProxy                         = "SCAVIUM_FAUCET_TRUSTED_PROXY"
	EnvCORSAllowedOrigins                   = "SCAVIUM_FAUCET_CORS_ALLOWED_ORIGINS"
	EnvWorkerEnabled                        = "SCAVIUM_FAUCET_WORKER_ENABLED"
	EnvWorkerPollSeconds                    = "SCAVIUM_FAUCET_WORKER_POLL_SECONDS"
	EnvWatcherEnabled                       = "SCAVIUM_FAUCET_WATCHER_ENABLED"
	EnvWatcherPollSeconds                   = "SCAVIUM_FAUCET_WATCHER_POLL_SECONDS"
	EnvMinConfirmations                     = "SCAVIUM_FAUCET_MIN_CONFIRMATIONS"
	// EnvPrivateKey holds the hex-encoded private key used to sign transactions.
	// Never log this value.
	EnvPrivateKey = "SCAVIUM_FAUCET_PRIVATE_KEY"

	// Captcha configuration.
	// EnvCaptchaProvider selects the captcha backend: "disabled", "dev",
	// "hcaptcha", "recaptcha", or "turnstile".
	EnvCaptchaProvider  = "SCAVIUM_FAUCET_CAPTCHA_PROVIDER"
	EnvCaptchaSiteKey   = "SCAVIUM_FAUCET_CAPTCHA_SITE_KEY"
	EnvCaptchaSecret    = "SCAVIUM_FAUCET_CAPTCHA_SECRET"
	EnvCaptchaVerifyURL = "SCAVIUM_FAUCET_CAPTCHA_VERIFY_URL"

	// EnvFaucetMode controls the operational mode: "active", "paused", or
	// "maintenance".
	EnvFaucetMode = "SCAVIUM_FAUCET_MODE"

	// EnvAdminToken is the bearer token required to access admin endpoints.
	// If empty, admin endpoints are disabled.  Never log this value.
	EnvAdminToken = "SCAVIUM_FAUCET_ADMIN_TOKEN"
)

// TokenConfig describes one claimable faucet asset.
type TokenConfig struct {
	ID             string
	Symbol         string
	Type           domain.TokenType
	Address        common.Address
	Decimals       int
	AmountWei      *big.Int
	DailyBudgetWei *big.Int
}

type tokenConfigJSON struct {
	ID             string `json:"id"`
	Symbol         string `json:"symbol"`
	Type           string `json:"type"`
	Address        string `json:"address"`
	Decimals       int    `json:"decimals"`
	AmountWei      string `json:"amount_wei"`
	DailyBudgetWei string `json:"daily_budget_wei"`
}

// Config is the validated runtime configuration required by the faucet service.
type Config struct {
	BindAddr                             string
	PublicBaseURL                        string
	RPCURL                               string
	RPCSecondaryURLs                     []string
	ChainID                              int64
	NetworkName                          string
	Symbol                               string
	Tokens                               []TokenConfig
	DefaultTokenID                       string
	ExplorerTxURL                        string
	AmountWei                            *big.Int
	CooldownSeconds                      int
	DryRun                               bool
	DatabasePath                         string
	RateLimitIPPerHour                   int
	RateLimitAddrPerDay                  int
	DailyBudgetWei                       *big.Int
	AbuseEnforcementEnabled              bool
	AbuseEnforcementWindowSeconds        int
	AbuseEnforcementIPThreshold          int
	AbuseEnforcementAddressThreshold     int
	AbuseEnforcementFingerprintThreshold int
	AbuseSignalRetentionDays             int
	TrustedProxy                         string
	CORSAllowedOrigins                   []string
	WorkerEnabled                        bool
	WorkerPollSeconds                    int
	WatcherEnabled                       bool
	WatcherPollSeconds                   int
	MinConfirmations                     uint64
	// PrivateKeyHex is the hex-encoded signer key. Not required when DryRun=true.
	// Never log this value.
	PrivateKeyHex string

	// Captcha settings.
	// CaptchaProvider selects the backend.  Defaults to "disabled".
	CaptchaProvider string
	// CaptchaSiteKey is the public browser-side site key for the chosen provider.
	CaptchaSiteKey string
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

// LoadFromEnv loads configuration from the process environment and validates it.
func LoadFromEnv() (Config, error) {
	cfg, err := FromEnv(getenv)
	if err != nil {
		return Config{}, err
	}
	return cfg, cfg.Validate()
}

// FromEnv loads configuration values through lookup, applying Defaults first.
func FromEnv(lookup func(string) string) (Config, error) {
	cfg := Defaults()

	cfg.BindAddr = envOrDefault(lookup, EnvBindAddr, cfg.BindAddr)
	cfg.PublicBaseURL = envOrDefault(lookup, EnvPublicBaseURL, cfg.PublicBaseURL)
	cfg.RPCURL = envOrDefault(lookup, EnvRPCURL, cfg.RPCURL)
	cfg.RPCSecondaryURLs = splitCommaList(lookup(EnvRPCSecondaryURLs))
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

	if raw := strings.TrimSpace(lookup(EnvDefaultTokenID)); raw != "" {
		cfg.DefaultTokenID = raw
	}
	if raw := strings.TrimSpace(lookup(EnvTokensJSON)); raw != "" {
		tokens, err := parseTokensJSON(raw)
		if err != nil {
			return Config{}, fmt.Errorf("%s: %w", EnvTokensJSON, err)
		}
		cfg.Tokens = tokens
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

	cfg.DatabasePath = envOrDefault(lookup, EnvDatabasePath, cfg.DatabasePath)

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

	if raw := strings.TrimSpace(lookup(EnvAbuseEnforcementEnabled)); raw != "" {
		v, err := strconv.ParseBool(raw)
		if err != nil {
			return Config{}, fmt.Errorf("%s: %w", EnvAbuseEnforcementEnabled, err)
		}
		cfg.AbuseEnforcementEnabled = v
	}
	if raw := strings.TrimSpace(lookup(EnvAbuseEnforcementWindowSeconds)); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil {
			return Config{}, fmt.Errorf("%s: %w", EnvAbuseEnforcementWindowSeconds, err)
		}
		cfg.AbuseEnforcementWindowSeconds = v
	}
	if raw := strings.TrimSpace(lookup(EnvAbuseEnforcementIPThreshold)); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil {
			return Config{}, fmt.Errorf("%s: %w", EnvAbuseEnforcementIPThreshold, err)
		}
		cfg.AbuseEnforcementIPThreshold = v
	}
	if raw := strings.TrimSpace(lookup(EnvAbuseEnforcementAddressThreshold)); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil {
			return Config{}, fmt.Errorf("%s: %w", EnvAbuseEnforcementAddressThreshold, err)
		}
		cfg.AbuseEnforcementAddressThreshold = v
	}
	if raw := strings.TrimSpace(lookup(EnvAbuseEnforcementFingerprintThreshold)); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil {
			return Config{}, fmt.Errorf("%s: %w", EnvAbuseEnforcementFingerprintThreshold, err)
		}
		cfg.AbuseEnforcementFingerprintThreshold = v
	}
	if raw := strings.TrimSpace(lookup(EnvAbuseSignalRetentionDays)); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil {
			return Config{}, fmt.Errorf("%s: %w", EnvAbuseSignalRetentionDays, err)
		}
		cfg.AbuseSignalRetentionDays = v
	}

	cfg.TrustedProxy = strings.TrimSpace(lookup(EnvTrustedProxy))
	cfg.CORSAllowedOrigins = splitCommaList(lookup(EnvCORSAllowedOrigins))
	cfg.PrivateKeyHex = strings.TrimSpace(lookup(EnvPrivateKey))

	if raw := strings.TrimSpace(lookup(EnvWorkerEnabled)); raw != "" {
		v, err := strconv.ParseBool(raw)
		if err != nil {
			return Config{}, fmt.Errorf("%s: %w", EnvWorkerEnabled, err)
		}
		cfg.WorkerEnabled = v
	}
	if raw := strings.TrimSpace(lookup(EnvWorkerPollSeconds)); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil {
			return Config{}, fmt.Errorf("%s: %w", EnvWorkerPollSeconds, err)
		}
		cfg.WorkerPollSeconds = v
	}
	if raw := strings.TrimSpace(lookup(EnvWatcherEnabled)); raw != "" {
		v, err := strconv.ParseBool(raw)
		if err != nil {
			return Config{}, fmt.Errorf("%s: %w", EnvWatcherEnabled, err)
		}
		cfg.WatcherEnabled = v
	} else if !cfg.DryRun {
		cfg.WatcherEnabled = true
	}
	if raw := strings.TrimSpace(lookup(EnvWatcherPollSeconds)); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil {
			return Config{}, fmt.Errorf("%s: %w", EnvWatcherPollSeconds, err)
		}
		cfg.WatcherPollSeconds = v
	}
	if raw := strings.TrimSpace(lookup(EnvMinConfirmations)); raw != "" {
		v, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return Config{}, fmt.Errorf("%s: %w", EnvMinConfirmations, err)
		}
		cfg.MinConfirmations = v
	}

	cfg.CaptchaProvider = strings.ToLower(envOrDefault(lookup, EnvCaptchaProvider, cfg.CaptchaProvider))
	cfg.CaptchaSiteKey = strings.TrimSpace(lookup(EnvCaptchaSiteKey))
	cfg.CaptchaSecret = strings.TrimSpace(lookup(EnvCaptchaSecret))
	cfg.CaptchaVerifyURL = envOrDefault(lookup, EnvCaptchaVerifyURL, cfg.CaptchaVerifyURL)
	cfg.FaucetMode = envOrDefault(lookup, EnvFaucetMode, cfg.FaucetMode)
	cfg.AdminToken = strings.TrimSpace(lookup(EnvAdminToken))

	return cfg, nil
}

// Defaults returns the development-safe faucet configuration baseline.
func Defaults() Config {
	return Config{
		BindAddr:                             "127.0.0.1:18080",
		PublicBaseURL:                        "http://127.0.0.1:18080",
		RPCURL:                               "http://127.0.0.1:18545",
		ChainID:                              31337,
		NetworkName:                          "scavium-dev",
		Symbol:                               "SCAV",
		DefaultTokenID:                       "native",
		ExplorerTxURL:                        "",
		AmountWei:                            big.NewInt(1_000_000_000_000_000_000),
		CooldownSeconds:                      int((24 * time.Hour).Seconds()),
		DryRun:                               true,
		DatabasePath:                         "cmd/scavium-faucet/data/scavium-faucet.db",
		RateLimitIPPerHour:                   10,
		RateLimitAddrPerDay:                  3,
		DailyBudgetWei:                       nil,
		AbuseEnforcementEnabled:              true,
		AbuseEnforcementWindowSeconds:        3600,
		AbuseEnforcementIPThreshold:          20,
		AbuseEnforcementAddressThreshold:     12,
		AbuseEnforcementFingerprintThreshold: 15,
		AbuseSignalRetentionDays:             30,
		TrustedProxy:                         "",
		CORSAllowedOrigins:                   nil,
		WorkerEnabled:                        true,
		WorkerPollSeconds:                    5,
		WatcherEnabled:                       false,
		WatcherPollSeconds:                   15,
		MinConfirmations:                     1,
		CaptchaProvider:                      "disabled",
		CaptchaVerifyURL:                     "",
		FaucetMode:                           "active",
	}
}

// Validate reports configuration errors that would prevent the faucet from running safely.
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
	for _, rpcURL := range c.RPCSecondaryURLs {
		if strings.TrimSpace(rpcURL) == "" {
			errs = append(errs, errors.New("secondary RPC URLs must not be empty"))
		}
		if strings.TrimSpace(rpcURL) == strings.TrimSpace(c.RPCURL) {
			errs = append(errs, errors.New("secondary RPC URLs must not duplicate primary RPC URL"))
		}
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

	if err := c.validateTokens(); err != nil {
		errs = append(errs, err)
	}
	if c.CooldownSeconds < 0 {
		errs = append(errs, errors.New("cooldown seconds must be zero or positive"))
	}
	if c.AbuseEnforcementWindowSeconds <= 0 {
		errs = append(errs, errors.New("abuse enforcement window seconds must be positive"))
	}
	if c.AbuseEnforcementIPThreshold < 0 {
		errs = append(errs, errors.New("abuse enforcement IP threshold must be zero or positive"))
	}
	if c.AbuseEnforcementAddressThreshold < 0 {
		errs = append(errs, errors.New("abuse enforcement address threshold must be zero or positive"))
	}
	if c.AbuseEnforcementFingerprintThreshold < 0 {
		errs = append(errs, errors.New("abuse enforcement fingerprint threshold must be zero or positive"))
	}
	if c.AbuseSignalRetentionDays < 0 {
		errs = append(errs, errors.New("abuse signal retention days must be zero or positive"))
	}
	if strings.TrimSpace(c.DatabasePath) == "" {
		errs = append(errs, errors.New("database path is required"))
	}
	for _, origin := range c.CORSAllowedOrigins {
		if strings.TrimSpace(origin) == "*" {
			errs = append(errs, errors.New("CORS allowed origins must not contain wildcard"))
		}
	}
	provider := strings.TrimSpace(strings.ToLower(c.CaptchaProvider))
	switch provider {
	case "", "disabled", "dev", "hcaptcha", "recaptcha", "turnstile":
	default:
		errs = append(errs, fmt.Errorf("unsupported captcha provider %q", c.CaptchaProvider))
	}
	if provider == "hcaptcha" || provider == "recaptcha" || provider == "turnstile" {
		if strings.TrimSpace(c.CaptchaSiteKey) == "" {
			errs = append(errs, fmt.Errorf("captcha provider %q requires site key", provider))
		}
		if strings.TrimSpace(c.CaptchaSecret) == "" {
			errs = append(errs, fmt.Errorf("captcha provider %q requires secret", provider))
		}
	}

	if c.WorkerPollSeconds <= 0 {
		errs = append(errs, errors.New("worker poll seconds must be positive"))
	}
	if c.WatcherPollSeconds <= 0 {
		errs = append(errs, errors.New("watcher poll seconds must be positive"))
	}

	return errors.Join(errs...)
}

// NormalizedTokens returns the configured token list, falling back to the legacy
// single native token fields when SCAVIUM_FAUCET_TOKENS_JSON is not set.
func (c Config) NormalizedTokens() []TokenConfig {
	if len(c.Tokens) > 0 {
		out := make([]TokenConfig, 0, len(c.Tokens))
		for _, token := range c.Tokens {
			out = append(out, normalizeToken(token, c))
		}
		return out
	}
	return []TokenConfig{normalizeToken(TokenConfig{
		ID:        "native",
		Symbol:    c.Symbol,
		Type:      domain.TokenTypeNative,
		Decimals:  18,
		AmountWei: c.AmountWei,
	}, c)}
}

// DefaultToken returns the token used by backward-compatible claim requests that
// do not specify token_id.
func (c Config) DefaultToken() TokenConfig {
	tokens := c.NormalizedTokens()
	wanted := strings.TrimSpace(c.DefaultTokenID)
	if wanted == "" {
		wanted = "native"
	}
	for _, token := range tokens {
		if token.ID == wanted {
			return token
		}
	}
	if len(tokens) == 0 {
		return normalizeToken(TokenConfig{}, c)
	}
	return tokens[0]
}

// TokenByID resolves a configured token by id. Empty id resolves to DefaultToken.
func (c Config) TokenByID(id string) (TokenConfig, bool) {
	id = strings.TrimSpace(id)
	if id == "" {
		return c.DefaultToken(), true
	}
	for _, token := range c.NormalizedTokens() {
		if token.ID == id {
			return token, true
		}
	}
	return TokenConfig{}, false
}

func (c Config) validateTokens() error {
	tokens := c.NormalizedTokens()
	if len(tokens) == 0 {
		return errors.New("at least one faucet token is required")
	}
	seen := map[string]struct{}{}
	var errs []error
	for _, token := range tokens {
		if strings.TrimSpace(token.ID) == "" {
			errs = append(errs, errors.New("token id is required"))
		}
		if _, exists := seen[token.ID]; exists {
			errs = append(errs, fmt.Errorf("duplicate token id %q", token.ID))
		}
		seen[token.ID] = struct{}{}
		if strings.TrimSpace(token.Symbol) == "" {
			errs = append(errs, fmt.Errorf("token %q symbol is required", token.ID))
		}
		if !domain.IsValidTokenType(token.Type) {
			errs = append(errs, fmt.Errorf("token %q has unsupported type %q", token.ID, token.Type))
		}
		if token.Type == domain.TokenTypeERC20 && token.Address == (common.Address{}) {
			errs = append(errs, fmt.Errorf("token %q ERC20 address is required", token.ID))
		}
		if token.Decimals < 0 {
			errs = append(errs, fmt.Errorf("token %q decimals must be zero or positive", token.ID))
		}
		if token.AmountWei == nil || token.AmountWei.Sign() <= 0 {
			errs = append(errs, fmt.Errorf("token %q amount wei must be positive", token.ID))
		}
		if token.DailyBudgetWei != nil && token.DailyBudgetWei.Sign() < 0 {
			errs = append(errs, fmt.Errorf("token %q daily budget wei must be zero or positive", token.ID))
		}
	}
	if _, ok := c.TokenByID(c.DefaultTokenID); !ok {
		errs = append(errs, fmt.Errorf("default token %q is not configured", c.DefaultTokenID))
	}
	return errors.Join(errs...)
}

func normalizeToken(token TokenConfig, cfg Config) TokenConfig {
	if strings.TrimSpace(token.ID) == "" {
		token.ID = "native"
	}
	token.ID = strings.TrimSpace(token.ID)
	if strings.TrimSpace(token.Symbol) == "" {
		token.Symbol = cfg.Symbol
	}
	token.Symbol = strings.TrimSpace(token.Symbol)
	if token.Type == "" {
		token.Type = domain.TokenTypeNative
	}
	token.Type = domain.TokenType(strings.ToLower(strings.TrimSpace(string(token.Type))))
	if token.Decimals == 0 {
		token.Decimals = 18
	}
	if token.AmountWei == nil {
		token.AmountWei = copyBigInt(cfg.AmountWei)
	} else {
		token.AmountWei = copyBigInt(token.AmountWei)
	}
	if token.DailyBudgetWei == nil {
		token.DailyBudgetWei = copyBigIntOrNil(cfg.DailyBudgetWei)
	} else {
		token.DailyBudgetWei = copyBigInt(token.DailyBudgetWei)
	}
	return token
}

func parseTokensJSON(raw string) ([]TokenConfig, error) {
	var encoded []tokenConfigJSON
	if err := json.Unmarshal([]byte(raw), &encoded); err != nil {
		return nil, err
	}
	tokens := make([]TokenConfig, 0, len(encoded))
	for _, item := range encoded {
		amount, err := parseRequiredBigInt(item.AmountWei, "amount_wei")
		if err != nil {
			return nil, err
		}
		budget, err := parseOptionalBigInt(item.DailyBudgetWei, "daily_budget_wei")
		if err != nil {
			return nil, err
		}
		address := common.Address{}
		if strings.TrimSpace(item.Address) != "" {
			if !common.IsHexAddress(strings.TrimSpace(item.Address)) {
				return nil, fmt.Errorf("invalid token address %q", item.Address)
			}
			address = common.HexToAddress(strings.TrimSpace(item.Address))
		}
		tokens = append(tokens, TokenConfig{
			ID:             strings.TrimSpace(item.ID),
			Symbol:         strings.TrimSpace(item.Symbol),
			Type:           domain.TokenType(strings.ToLower(strings.TrimSpace(item.Type))),
			Address:        address,
			Decimals:       item.Decimals,
			AmountWei:      amount,
			DailyBudgetWei: budget,
		})
	}
	return tokens, nil
}

func parseRequiredBigInt(raw, field string) (*big.Int, error) {
	value, err := parseOptionalBigInt(raw, field)
	if err != nil {
		return nil, err
	}
	if value == nil {
		return nil, fmt.Errorf("%s is required", field)
	}
	return value, nil
}

func parseOptionalBigInt(raw, field string) (*big.Int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	value, ok := new(big.Int).SetString(raw, 10)
	if !ok {
		return nil, fmt.Errorf("%s: invalid integer", field)
	}
	return value, nil
}

func copyBigInt(v *big.Int) *big.Int {
	if v == nil {
		return big.NewInt(0)
	}
	return new(big.Int).Set(v)
}

func copyBigIntOrNil(v *big.Int) *big.Int {
	if v == nil {
		return nil
	}
	return new(big.Int).Set(v)
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

func splitCommaList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			values = append(values, value)
		}
	}
	return values
}

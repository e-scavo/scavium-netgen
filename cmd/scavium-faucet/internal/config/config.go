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
	EnvBindAddr        = "SCAVIUM_FAUCET_BIND_ADDR"
	EnvPublicBaseURL   = "SCAVIUM_FAUCET_PUBLIC_BASE_URL"
	EnvRPCURL          = "SCAVIUM_FAUCET_RPC_URL"
	EnvChainID         = "SCAVIUM_FAUCET_CHAIN_ID"
	EnvNetworkName     = "SCAVIUM_FAUCET_NETWORK_NAME"
	EnvSymbol          = "SCAVIUM_FAUCET_SYMBOL"
	EnvExplorerTxURL   = "SCAVIUM_FAUCET_EXPLORER_TX_URL"
	EnvAmountWei       = "SCAVIUM_FAUCET_AMOUNT_WEI"
	EnvCooldownSeconds = "SCAVIUM_FAUCET_COOLDOWN_SECONDS"
	EnvDryRun          = "SCAVIUM_FAUCET_DRY_RUN"
)

type Config struct {
	BindAddr        string
	PublicBaseURL   string
	RPCURL          string
	ChainID         int64
	NetworkName     string
	Symbol          string
	ExplorerTxURL   string
	AmountWei       *big.Int
	CooldownSeconds int
	DryRun          bool
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

	return cfg, nil
}

func Defaults() Config {
	return Config{
		BindAddr:        "127.0.0.1:18080",
		PublicBaseURL:   "http://127.0.0.1:18080",
		RPCURL:          "http://127.0.0.1:18545",
		ChainID:         31337,
		NetworkName:     "scavium-dev",
		Symbol:          "SCAV",
		ExplorerTxURL:   "",
		AmountWei:       big.NewInt(1_000_000_000_000_000_000),
		CooldownSeconds: int((24 * time.Hour).Seconds()),
		DryRun:          true,
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

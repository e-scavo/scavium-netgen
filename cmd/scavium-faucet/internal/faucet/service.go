package faucet

import (
	"context"
	"time"

	"scavium-netgen/cmd/scavium-faucet/internal/config"
	"scavium-netgen/cmd/scavium-faucet/internal/domain"

	"github.com/ethereum/go-ethereum/common"
)

type StatusResponse struct {
	Status      domain.FaucetStatus `json:"status"`
	NetworkName string              `json:"network_name"`
	Symbol      string              `json:"symbol"`
	DryRun      bool                `json:"dry_run"`
	UpdatedAt   string              `json:"updated_at"`
}

type ConfigResponse struct {
	NetworkName     string `json:"network_name"`
	ChainID         int64  `json:"chain_id"`
	Symbol          string `json:"symbol"`
	AmountWei       string `json:"amount_wei"`
	CooldownSeconds int    `json:"cooldown_seconds"`
	ExplorerTxURL   string `json:"explorer_tx_url"`
	DryRun          bool   `json:"dry_run"`
}

type AddressStatusResponse struct {
	Address          string `json:"address"`
	Eligible         bool   `json:"eligible"`
	Reason           string `json:"reason"`
	CooldownSeconds  int    `json:"cooldown_seconds"`
	NextEligibleTime string `json:"next_eligible_time,omitempty"`
}

type ReadService interface {
	Status(context.Context) (StatusResponse, error)
	Config(context.Context) (ConfigResponse, error)
	AddressStatus(context.Context, common.Address) (AddressStatusResponse, error)
}

type InMemoryReadService struct {
	cfg config.Config
	now func() time.Time
}

func NewInMemoryReadService(cfg config.Config) *InMemoryReadService {
	return &InMemoryReadService{
		cfg: cfg,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func NewInMemoryReadServiceWithClock(cfg config.Config, now func() time.Time) *InMemoryReadService {
	service := NewInMemoryReadService(cfg)
	if now != nil {
		service.now = now
	}
	return service
}

func (s *InMemoryReadService) Status(context.Context) (StatusResponse, error) {
	return StatusResponse{
		Status:      domain.FaucetStatusActive,
		NetworkName: s.cfg.NetworkName,
		Symbol:      s.cfg.Symbol,
		DryRun:      s.cfg.DryRun,
		UpdatedAt:   s.now().Format(time.RFC3339),
	}, nil
}

func (s *InMemoryReadService) Config(context.Context) (ConfigResponse, error) {
	amountWei := ""
	if s.cfg.AmountWei != nil {
		amountWei = s.cfg.AmountWei.String()
	}

	return ConfigResponse{
		NetworkName:     s.cfg.NetworkName,
		ChainID:         s.cfg.ChainID,
		Symbol:          s.cfg.Symbol,
		AmountWei:       amountWei,
		CooldownSeconds: s.cfg.CooldownSeconds,
		ExplorerTxURL:   s.cfg.ExplorerTxURL,
		DryRun:          s.cfg.DryRun,
	}, nil
}

func (s *InMemoryReadService) AddressStatus(_ context.Context, address common.Address) (AddressStatusResponse, error) {
	return AddressStatusResponse{
		Address:         address.Hex(),
		Eligible:        true,
		Reason:          "eligible",
		CooldownSeconds: s.cfg.CooldownSeconds,
	}, nil
}

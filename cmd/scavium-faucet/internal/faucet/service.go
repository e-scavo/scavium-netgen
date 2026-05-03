package faucet

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
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

type ClaimRequest struct {
	Address        common.Address
	IdempotencyKey string
}

type ClaimResponse struct {
	ID             string             `json:"id"`
	Address        string             `json:"address"`
	AmountWei      string             `json:"amount_wei"`
	Status         domain.ClaimStatus `json:"status"`
	Reason         string             `json:"reason,omitempty"`
	IdempotencyKey string             `json:"idempotency_key,omitempty"`
	CreatedAt      string             `json:"created_at"`
	UpdatedAt      string             `json:"updated_at"`
}

type ReadService interface {
	Status(context.Context) (StatusResponse, error)
	Config(context.Context) (ConfigResponse, error)
	AddressStatus(context.Context, common.Address) (AddressStatusResponse, error)
	CreateClaim(context.Context, ClaimRequest) (ClaimResponse, error)
	GetClaim(context.Context, string) (ClaimResponse, bool, error)
}

type InMemoryReadService struct {
	mu              sync.RWMutex
	cfg             config.Config
	now             func() time.Time
	claimsByID      map[string]domain.Claim
	claimIDsByIdem  map[string]string
	generateClaimID func() (string, error)
}

func NewInMemoryReadService(cfg config.Config) *InMemoryReadService {
	return &InMemoryReadService{
		cfg: cfg,
		now: func() time.Time {
			return time.Now().UTC()
		},
		claimsByID:     map[string]domain.Claim{},
		claimIDsByIdem: map[string]string{},
		generateClaimID: func() (string, error) {
			return randomID("claim")
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

func (s *InMemoryReadService) SetClaimIDGenerator(generate func() (string, error)) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if generate != nil {
		s.generateClaimID = generate
	}
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

func (s *InMemoryReadService) CreateClaim(_ context.Context, request ClaimRequest) (ClaimResponse, error) {
	idempotencyKey := request.IdempotencyKey

	s.mu.Lock()
	defer s.mu.Unlock()

	if idempotencyKey != "" {
		if claimID, ok := s.claimIDsByIdem[idempotencyKey]; ok {
			return claimResponse(s.claimsByID[claimID], idempotencyKey), nil
		}
	}

	id, err := s.generateClaimID()
	if err != nil {
		return ClaimResponse{}, fmt.Errorf("generate claim id: %w", err)
	}

	now := s.now()
	claim := domain.Claim{
		ID:        id,
		Address:   request.Address,
		AmountWei: s.cfg.AmountWei,
		Status:    domain.ClaimStatusQueued,
		CreatedAt: now,
		UpdatedAt: now,
	}

	s.claimsByID[id] = claim
	if idempotencyKey != "" {
		s.claimIDsByIdem[idempotencyKey] = id
	}

	return claimResponse(claim, idempotencyKey), nil
}

func (s *InMemoryReadService) GetClaim(_ context.Context, id string) (ClaimResponse, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	claim, ok := s.claimsByID[id]
	if !ok {
		return ClaimResponse{}, false, nil
	}

	return claimResponse(claim, ""), true, nil
}

func claimResponse(claim domain.Claim, idempotencyKey string) ClaimResponse {
	amountWei := ""
	if claim.AmountWei != nil {
		amountWei = claim.AmountWei.String()
	}

	return ClaimResponse{
		ID:             claim.ID,
		Address:        claim.Address.Hex(),
		AmountWei:      amountWei,
		Status:         claim.Status,
		Reason:         claim.Reason,
		IdempotencyKey: idempotencyKey,
		CreatedAt:      claim.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:      claim.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func randomID(prefix string) (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(b[:])), nil
}

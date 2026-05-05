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

// StatusResponse is returned by public faucet status endpoints.
type StatusResponse struct {
	Status      domain.FaucetStatus `json:"status"`
	NetworkName string              `json:"network_name"`
	Symbol      string              `json:"symbol"`
	DryRun      bool                `json:"dry_run"`
	UpdatedAt   string              `json:"updated_at"`
}

// ConfigResponse is returned by public faucet config endpoints.
type ConfigResponse struct {
	NetworkName         string          `json:"network_name"`
	ChainID             int64           `json:"chain_id"`
	Symbol              string          `json:"symbol"`
	AmountWei           string          `json:"amount_wei"`
	Tokens              []TokenResponse `json:"tokens,omitempty"`
	CooldownSeconds     int             `json:"cooldown_seconds"`
	ExplorerTxURL       string          `json:"explorer_tx_url"`
	DryRun              bool            `json:"dry_run"`
	RateLimitIPPerHour  int             `json:"rate_limit_ip_per_hour"`
	RateLimitAddrPerDay int             `json:"rate_limit_addr_per_day"`
	CaptchaProvider     string          `json:"captcha_provider"`
	CaptchaSiteKey      string          `json:"captcha_site_key,omitempty"`
}

// AddressStatusResponse describes whether an address can request funds now.
type AddressStatusResponse struct {
	Address                  string `json:"address"`
	Eligible                 bool   `json:"eligible"`
	Reason                   string `json:"reason"`
	CooldownSeconds          int    `json:"cooldown_seconds"`
	CooldownRemainingSeconds int    `json:"cooldown_remaining_seconds"`
	NextEligibleTime         string `json:"next_eligible_time,omitempty"`
	RateLimitIPPerHour       int    `json:"rate_limit_ip_per_hour"`
	RateLimitAddrPerDay      int    `json:"rate_limit_addr_per_day"`
}

// ClaimRequest is the internal read-side input for claim creation.
type ClaimRequest struct {
	Address        common.Address
	TokenID        string
	IdempotencyKey string
	RemoteIP       string
	UserAgent      string
	CaptchaToken   string
	Fingerprint    string
}

// TokenResponse is returned by public config and claim endpoints for token-aware clients.
type TokenResponse struct {
	ID             string `json:"id"`
	Symbol         string `json:"symbol"`
	Type           string `json:"type"`
	Address        string `json:"address,omitempty"`
	Decimals       int    `json:"decimals"`
	AmountWei      string `json:"amount_wei"`
	DailyBudgetWei string `json:"daily_budget_wei,omitempty"`
}

// ClaimResponse is returned by claim creation and lookup endpoints.
type ClaimResponse struct {
	ID             string             `json:"id"`
	Address        string             `json:"address"`
	TxHash         string             `json:"tx_hash,omitempty"`
	TokenID        string             `json:"token_id,omitempty"`
	TokenSymbol    string             `json:"token_symbol,omitempty"`
	TokenType      string             `json:"token_type,omitempty"`
	TokenAddress   string             `json:"token_address,omitempty"`
	TokenDecimals  int                `json:"token_decimals,omitempty"`
	AmountWei      string             `json:"amount_wei"`
	Status         domain.ClaimStatus `json:"status"`
	Reason         string             `json:"reason,omitempty"`
	IdempotencyKey string             `json:"idempotency_key,omitempty"`
	CreatedAt      string             `json:"created_at"`
	UpdatedAt      string             `json:"updated_at"`
}

// ReadService defines the public read/claim contract consumed by HTTP handlers.
type ReadService interface {
	Status(context.Context) (StatusResponse, error)
	Config(context.Context) (ConfigResponse, error)
	Tokens(context.Context) ([]TokenResponse, error)
	AddressStatus(context.Context, common.Address) (AddressStatusResponse, error)
	CreateClaim(context.Context, ClaimRequest) (ClaimResponse, error)
	GetClaim(context.Context, string) (ClaimResponse, bool, error)
}

// InMemoryReadService is a concurrency-safe in-memory implementation of ReadService.
type InMemoryReadService struct {
	mu              sync.RWMutex
	cfg             config.Config
	now             func() time.Time
	claimsByID      map[string]domain.Claim
	claimIDsByIdem  map[string]string
	generateClaimID func() (string, error)
}

// NewInMemoryReadService creates a default in-memory read service.
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

// NewInMemoryReadServiceWithClock is a test helper that injects the time source.
func NewInMemoryReadServiceWithClock(cfg config.Config, now func() time.Time) *InMemoryReadService {
	service := NewInMemoryReadService(cfg)
	if now != nil {
		service.now = now
	}
	return service
}

// SetClaimIDGenerator overrides claim ID generation (primarily for tests).
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

	tokens := tokenResponses(s.cfg.NormalizedTokens())
	return ConfigResponse{
		NetworkName:         s.cfg.NetworkName,
		ChainID:             s.cfg.ChainID,
		Symbol:              s.cfg.Symbol,
		AmountWei:           amountWei,
		Tokens:              tokens,
		CooldownSeconds:     s.cfg.CooldownSeconds,
		ExplorerTxURL:       s.cfg.ExplorerTxURL,
		DryRun:              s.cfg.DryRun,
		RateLimitIPPerHour:  s.cfg.RateLimitIPPerHour,
		RateLimitAddrPerDay: s.cfg.RateLimitAddrPerDay,
		CaptchaProvider:     s.cfg.CaptchaProvider,
		CaptchaSiteKey:      s.cfg.CaptchaSiteKey,
	}, nil
}

// Tokens returns the public faucet token catalog. It intentionally exposes only
// claim-safe metadata and never includes private keys or operational secrets.
func (s *InMemoryReadService) Tokens(context.Context) ([]TokenResponse, error) {
	return tokenResponses(s.cfg.NormalizedTokens()), nil
}

func (s *InMemoryReadService) AddressStatus(_ context.Context, address common.Address) (AddressStatusResponse, error) {
	return AddressStatusResponse{
		Address:                  address.Hex(),
		Eligible:                 true,
		Reason:                   "eligible",
		CooldownSeconds:          s.cfg.CooldownSeconds,
		CooldownRemainingSeconds: 0,
		RateLimitIPPerHour:       s.cfg.RateLimitIPPerHour,
		RateLimitAddrPerDay:      s.cfg.RateLimitAddrPerDay,
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
	token, err := validateClaimToken(s.cfg, request.TokenID)
	if err != nil {
		return ClaimResponse{}, err
	}
	claim := domain.Claim{
		ID:            id,
		Address:       request.Address,
		TokenID:       token.ID,
		TokenSymbol:   token.Symbol,
		TokenType:     token.Type,
		TokenAddress:  token.Address,
		TokenDecimals: token.Decimals,
		AmountWei:     token.AmountWei,
		Status:        domain.ClaimStatusQueued,
		CreatedAt:     now,
		UpdatedAt:     now,
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

	txHash := ""
	if claim.Transaction != nil && claim.Transaction.Hash != (common.Hash{}) {
		txHash = claim.Transaction.Hash.Hex()
	}

	return ClaimResponse{
		ID:             claim.ID,
		Address:        claim.Address.Hex(),
		TxHash:         txHash,
		TokenID:        claim.TokenID,
		TokenSymbol:    claim.TokenSymbol,
		TokenType:      string(claim.TokenType),
		TokenAddress:   tokenAddressHex(claim.TokenAddress),
		TokenDecimals:  claim.TokenDecimals,
		AmountWei:      amountWei,
		Status:         claim.Status,
		Reason:         claim.Reason,
		IdempotencyKey: idempotencyKey,
		CreatedAt:      claim.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:      claim.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func validateClaimToken(cfg config.Config, tokenID string) (config.TokenConfig, error) {
	token, ok := cfg.TokenByID(tokenID)
	if !ok {
		return config.TokenConfig{}, claimError(ErrClaimRejected, "invalid_token")
	}
	if token.ID == "" || token.Symbol == "" || !domain.IsValidTokenType(token.Type) {
		return config.TokenConfig{}, claimError(ErrClaimRejected, "invalid_token")
	}
	if token.Type == domain.TokenTypeERC20 && token.Address == (common.Address{}) {
		return config.TokenConfig{}, claimError(ErrClaimRejected, "invalid_token")
	}
	if token.Decimals < 0 || token.AmountWei == nil || token.AmountWei.Sign() <= 0 {
		return config.TokenConfig{}, claimError(ErrClaimRejected, "invalid_token")
	}
	return token, nil
}

func tokenResponses(tokens []config.TokenConfig) []TokenResponse {
	out := make([]TokenResponse, 0, len(tokens))
	for _, token := range tokens {
		amountWei := ""
		if token.AmountWei != nil {
			amountWei = token.AmountWei.String()
		}
		dailyBudgetWei := ""
		if token.DailyBudgetWei != nil {
			dailyBudgetWei = token.DailyBudgetWei.String()
		}
		out = append(out, TokenResponse{
			ID:             token.ID,
			Symbol:         token.Symbol,
			Type:           string(token.Type),
			Address:        tokenAddressHex(token.Address),
			Decimals:       token.Decimals,
			AmountWei:      amountWei,
			DailyBudgetWei: dailyBudgetWei,
		})
	}
	return out
}

func tokenAddressHex(address common.Address) string {
	if address == (common.Address{}) {
		return ""
	}
	return address.Hex()
}

func randomID(prefix string) (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(b[:])), nil
}

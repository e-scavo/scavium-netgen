package faucet

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"sort"
	"strings"
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
	Address                  string        `json:"address"`
	Eligible                 bool          `json:"eligible"`
	Reason                   string        `json:"reason"`
	CooldownSeconds          int           `json:"cooldown_seconds"`
	CooldownRemainingSeconds int           `json:"cooldown_remaining_seconds"`
	NextEligibleTime         string        `json:"next_eligible_time,omitempty"`
	RateLimitIPPerHour       int           `json:"rate_limit_ip_per_hour"`
	RateLimitAddrPerDay      int           `json:"rate_limit_addr_per_day"`
	DefaultTokenID           string        `json:"default_token_id,omitempty"`
	Tokens                   []TokenStatus `json:"tokens,omitempty"`
	DailyBudget              *BudgetStatus `json:"daily_budget,omitempty"`
}

// TokenStatus is a public-safe per-token eligibility view for an address.
type TokenStatus struct {
	TokenID                  string        `json:"token_id"`
	Symbol                   string        `json:"symbol"`
	Type                     string        `json:"type"`
	Eligible                 bool          `json:"eligible"`
	Reason                   string        `json:"reason"`
	AmountWei                string        `json:"amount_wei"`
	DailyBudget              *BudgetStatus `json:"daily_budget,omitempty"`
	CooldownRemainingSeconds int           `json:"cooldown_remaining_seconds"`
	NextEligibleTime         string        `json:"next_eligible_time,omitempty"`
}

// BudgetStatus is a bounded public view of configured daily faucet capacity.
type BudgetStatus struct {
	BudgetWei    string `json:"budget_wei"`
	UsedWei      string `json:"used_wei"`
	RemainingWei string `json:"remaining_wei"`
}

// AddressHistoryResponse returns deterministic, paginated public claim history.
type AddressHistoryResponse struct {
	Address    string          `json:"address"`
	Claims     []ClaimResponse `json:"claims"`
	Pagination Pagination      `json:"pagination"`
}

// Pagination describes offset pagination used by bounded public list endpoints.
type Pagination struct {
	Limit   int  `json:"limit"`
	Offset  int  `json:"offset"`
	Count   int  `json:"count"`
	HasMore bool `json:"has_more"`
}

// ClaimRequest is the internal read-side input for claim creation.
type ClaimRequest struct {
	Address           common.Address
	TokenID           string
	IdempotencyKey    string
	RemoteIP          string
	UserAgent         string
	CaptchaToken      string
	Fingerprint       string
	Honeypot          string
	CampaignID        string
	InvitationCode    string
	WalletChallengeID string
	WalletSignature   string
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
	CampaignID     string             `json:"campaign_id,omitempty"`
	CreatedAt      string             `json:"created_at"`
	UpdatedAt      string             `json:"updated_at"`
}

// ReadService defines the public read/claim contract consumed by HTTP handlers.
type ReadService interface {
	Status(context.Context) (StatusResponse, error)
	Config(context.Context) (ConfigResponse, error)
	Tokens(context.Context) ([]TokenResponse, error)
	AddressStatus(context.Context, common.Address) (AddressStatusResponse, error)
	AddressHistory(context.Context, common.Address, int, int) (AddressHistoryResponse, error)
	CreateClaim(context.Context, ClaimRequest) (ClaimResponse, error)
	GetClaim(context.Context, string) (ClaimResponse, bool, error)
	CreateWalletChallenge(context.Context, WalletChallengeRequest) (WalletChallengeResponse, error)
}

// InMemoryReadService is a concurrency-safe in-memory implementation of ReadService.
type InMemoryReadService struct {
	mu               sync.RWMutex
	cfg              config.Config
	now              func() time.Time
	claimsByID       map[string]domain.Claim
	claimIDsByIdem   map[string]string
	generateClaimID  func() (string, error)
	walletChallenges map[string]WalletChallenge
}

// NewInMemoryReadService creates a default in-memory read service.
func NewInMemoryReadService(cfg config.Config) *InMemoryReadService {
	return &InMemoryReadService{
		cfg: cfg,
		now: func() time.Time {
			return time.Now().UTC()
		},
		claimsByID:       map[string]domain.Claim{},
		claimIDsByIdem:   map[string]string{},
		walletChallenges: map[string]WalletChallenge{},
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
	tokens := s.cfg.NormalizedTokens()
	response := AddressStatusResponse{
		Address:                  address.Hex(),
		Eligible:                 true,
		Reason:                   "eligible",
		CooldownSeconds:          s.cfg.CooldownSeconds,
		CooldownRemainingSeconds: 0,
		RateLimitIPPerHour:       s.cfg.RateLimitIPPerHour,
		RateLimitAddrPerDay:      s.cfg.RateLimitAddrPerDay,
		DefaultTokenID:           s.cfg.DefaultTokenID,
		Tokens:                   make([]TokenStatus, 0, len(tokens)),
	}

	s.mu.RLock()
	claims := make([]domain.Claim, 0, len(s.claimsByID))
	for _, claim := range s.claimsByID {
		claims = append(claims, claim)
	}
	s.mu.RUnlock()

	response.DailyBudget = inMemoryBudgetStatus(s.cfg.DailyBudgetWei, claims, "", s.now())
	if budgetExhausted(response.DailyBudget) {
		response.Eligible = false
		response.Reason = "daily_budget_exceeded"
	}
	for _, token := range tokens {
		amountWei := ""
		if token.AmountWei != nil {
			amountWei = token.AmountWei.String()
		}
		budget := inMemoryBudgetStatus(inMemoryDailyBudgetForTokenID(s.cfg, token.ID), claims, token.ID, s.now())
		eligible := true
		reason := "eligible"
		if budgetExhausted(budget) {
			eligible = false
			reason = "daily_budget_exceeded"
		}
		response.Tokens = append(response.Tokens, TokenStatus{
			TokenID: token.ID, Symbol: token.Symbol, Type: string(token.Type), Eligible: eligible, Reason: reason,
			AmountWei: amountWei, DailyBudget: budget,
		})
	}
	return response, nil
}

func (s *InMemoryReadService) AddressHistory(_ context.Context, address common.Address, limit, offset int) (AddressHistoryResponse, error) {
	limit = normalizeHistoryLimit(limit)
	offset = normalizeHistoryOffset(offset)

	s.mu.RLock()
	defer s.mu.RUnlock()

	all := make([]domain.Claim, 0)
	for _, claim := range s.claimsByID {
		if claim.Address == address {
			all = append(all, claim)
		}
	}
	sortClaimsForHistory(all)

	end := offset + limit
	if offset > len(all) {
		offset = len(all)
	}
	if end > len(all) {
		end = len(all)
	}
	selected := all[offset:end]
	claims := make([]ClaimResponse, 0, len(selected))
	for _, claim := range selected {
		claims = append(claims, claimResponse(claim, ""))
	}

	return AddressHistoryResponse{
		Address: address.Hex(),
		Claims:  claims,
		Pagination: Pagination{
			Limit: limit, Offset: offset, Count: len(claims), HasMore: end < len(all),
		},
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
	if err := s.verifyOptionalWalletProof(request); err != nil {
		return ClaimResponse{}, err
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
		CampaignID:    strings.TrimSpace(request.CampaignID),
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	s.claimsByID[id] = claim
	if idempotencyKey != "" {
		s.claimIDsByIdem[idempotencyKey] = id
	}

	return claimResponse(claim, idempotencyKey), nil
}

func (s *InMemoryReadService) CreateWalletChallenge(_ context.Context, request WalletChallengeRequest) (WalletChallengeResponse, error) {
	challenge, err := newWalletChallenge(request.Address, s.now())
	if err != nil {
		return WalletChallengeResponse{}, err
	}
	s.mu.Lock()
	s.walletChallenges[challenge.ID] = challenge
	s.mu.Unlock()
	return walletChallengeResponse(challenge), nil
}

func (s *InMemoryReadService) getWalletChallenge(id string, address common.Address) (WalletChallenge, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.walletChallenges[strings.TrimSpace(id)]
	if !ok || c.Address != address || c.ConsumedAt != nil || !s.now().Before(c.ExpiresAt) {
		return WalletChallenge{}, claimError(ErrClaimRejected, "invalid_wallet_challenge")
	}
	return c, nil
}

func (s *InMemoryReadService) consumeWalletChallenge(id string, address common.Address) (WalletChallenge, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.walletChallenges[strings.TrimSpace(id)]
	if !ok || c.Address != address || c.ConsumedAt != nil || !s.now().Before(c.ExpiresAt) {
		return WalletChallenge{}, claimError(ErrClaimRejected, "invalid_wallet_challenge")
	}
	now := s.now()
	c.ConsumedAt = &now
	s.walletChallenges[c.ID] = c
	return c, nil
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
		CampaignID:     claim.CampaignID,
		IdempotencyKey: idempotencyKey,
		CreatedAt:      claim.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:      claim.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func inMemoryDailyBudgetForTokenID(cfg config.Config, tokenID string) *big.Int {
	if token, ok := cfg.TokenByID(tokenID); ok && token.DailyBudgetWei != nil {
		return copyBigInt(token.DailyBudgetWei)
	}
	return copyBigIntOrNil(cfg.DailyBudgetWei)
}

func inMemoryBudgetStatus(budget *big.Int, claims []domain.Claim, tokenID string, now time.Time) *BudgetStatus {
	if budget == nil || budget.Sign() <= 0 {
		return nil
	}
	dayStart, dayEnd := utcDayWindow(now)
	used := big.NewInt(0)
	for _, claim := range claims {
		if tokenID != "" && claim.TokenID != tokenID {
			continue
		}
		if claim.CreatedAt.Before(dayStart) || !claim.CreatedAt.Before(dayEnd) {
			continue
		}
		if !claimStatusCountsForDailyBudget(claim.Status) || claim.AmountWei == nil {
			continue
		}
		used.Add(used, claim.AmountWei)
	}
	remaining := new(big.Int).Sub(copyBigInt(budget), used)
	if remaining.Sign() < 0 {
		remaining = big.NewInt(0)
	}
	return &BudgetStatus{BudgetWei: budget.String(), UsedWei: used.String(), RemainingWei: remaining.String()}
}

func budgetExhausted(budget *BudgetStatus) bool {
	return budget != nil && budget.RemainingWei == "0"
}

func claimStatusCountsForDailyBudget(status domain.ClaimStatus) bool {
	for _, counted := range dailyBudgetStatuses() {
		if status == counted {
			return true
		}
	}
	return false
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

func normalizeHistoryLimit(limit int) int {
	const defaultLimit = 25
	const maxLimit = 100
	if limit <= 0 {
		return defaultLimit
	}
	if limit > maxLimit {
		return maxLimit
	}
	return limit
}

func normalizeHistoryOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}

func sortClaimsForHistory(claims []domain.Claim) {
	sort.SliceStable(claims, func(i, j int) bool {
		if claims[i].CreatedAt.Equal(claims[j].CreatedAt) {
			return claims[i].ID > claims[j].ID
		}
		return claims[i].CreatedAt.After(claims[j].CreatedAt)
	})
}

func randomID(prefix string) (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(b[:])), nil
}

func (s *InMemoryReadService) verifyOptionalWalletProof(request ClaimRequest) error {
	challengeID := strings.TrimSpace(request.WalletChallengeID)
	sig := strings.TrimSpace(request.WalletSignature)
	if challengeID == "" && sig == "" {
		return nil
	}
	if challengeID == "" || sig == "" {
		return claimError(ErrClaimRejected, "wallet_proof_incomplete")
	}
	c, err := s.getWalletChallenge(challengeID, request.Address)
	if err != nil {
		return err
	}
	if err := verifyWalletSignature(request.Address, c.Message, sig); err != nil {
		return err
	}
	_, err = s.consumeWalletChallenge(challengeID, request.Address)
	return err
}

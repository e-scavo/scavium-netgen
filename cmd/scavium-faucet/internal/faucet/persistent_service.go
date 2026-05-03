package faucet

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"
	"time"

	"scavium-netgen/cmd/scavium-faucet/internal/config"
	"scavium-netgen/cmd/scavium-faucet/internal/domain"

	"github.com/ethereum/go-ethereum/common"
)

var _ ReadService = (*PersistentReadService)(nil)

type idempotentClaimStore interface {
	CreateClaimWithIdempotency(ctx context.Context, claim domain.Claim, idempotencyKey string) (domain.Claim, error)
	GetClaimByIdempotencyKey(ctx context.Context, idempotencyKey string) (domain.Claim, error)
}

// PersistentReadService implements ReadService using durable stores.
type PersistentReadService struct {
	cfg             config.Config
	claims          domain.ClaimStore
	queue           domain.QueueStore
	rateLimiter     domain.RateLimiter
	now             func() time.Time
	generateClaimID func() (string, error)
}

// NewPersistentReadService creates a SQLite-ready faucet read service.
func NewPersistentReadService(cfg config.Config, claims domain.ClaimStore, queue domain.QueueStore, rateLimiter domain.RateLimiter) *PersistentReadService {
	return &PersistentReadService{
		cfg:         cfg,
		claims:      claims,
		queue:       queue,
		rateLimiter: rateLimiter,
		now: func() time.Time {
			return time.Now().UTC()
		},
		generateClaimID: func() (string, error) {
			return randomID("claim")
		},
	}
}

// NewPersistentReadServiceWithClock creates a persistent service with test hooks.
func NewPersistentReadServiceWithClock(cfg config.Config, claims domain.ClaimStore, queue domain.QueueStore, rateLimiter domain.RateLimiter, now func() time.Time) *PersistentReadService {
	service := NewPersistentReadService(cfg, claims, queue, rateLimiter)
	if now != nil {
		service.now = now
	}
	return service
}

// SetClaimIDGenerator overrides claim ID generation (primarily for tests).
func (s *PersistentReadService) SetClaimIDGenerator(generate func() (string, error)) {
	if generate != nil {
		s.generateClaimID = generate
	}
}

func (s *PersistentReadService) Status(context.Context) (StatusResponse, error) {
	return StatusResponse{
		Status:      configuredStatus(s.cfg.FaucetMode),
		NetworkName: s.cfg.NetworkName,
		Symbol:      s.cfg.Symbol,
		DryRun:      s.cfg.DryRun,
		UpdatedAt:   s.now().Format(time.RFC3339),
	}, nil
}

func (s *PersistentReadService) Config(context.Context) (ConfigResponse, error) {
	amountWei := ""
	if s.cfg.AmountWei != nil {
		amountWei = s.cfg.AmountWei.String()
	}

	return ConfigResponse{
		NetworkName:         s.cfg.NetworkName,
		ChainID:             s.cfg.ChainID,
		Symbol:              s.cfg.Symbol,
		AmountWei:           amountWei,
		CooldownSeconds:     s.cfg.CooldownSeconds,
		ExplorerTxURL:       s.cfg.ExplorerTxURL,
		DryRun:              s.cfg.DryRun,
		RateLimitIPPerHour:  s.cfg.RateLimitIPPerHour,
		RateLimitAddrPerDay: s.cfg.RateLimitAddrPerDay,
	}, nil
}

func (s *PersistentReadService) AddressStatus(ctx context.Context, address common.Address) (AddressStatusResponse, error) {
	remaining, nextEligible, err := s.cooldown(ctx, address)
	if err != nil {
		return AddressStatusResponse{}, err
	}

	response := AddressStatusResponse{
		Address:                  address.Hex(),
		Eligible:                 remaining == 0,
		Reason:                   "eligible",
		CooldownSeconds:          s.cfg.CooldownSeconds,
		CooldownRemainingSeconds: remaining,
		RateLimitIPPerHour:       s.cfg.RateLimitIPPerHour,
		RateLimitAddrPerDay:      s.cfg.RateLimitAddrPerDay,
	}
	if remaining > 0 {
		response.Reason = "cooldown_active"
		response.NextEligibleTime = nextEligible.UTC().Format(time.RFC3339)
	}
	return response, nil
}

func (s *PersistentReadService) CreateClaim(ctx context.Context, request ClaimRequest) (ClaimResponse, error) {
	idempotencyKey := strings.TrimSpace(request.IdempotencyKey)
	if idempotencyKey != "" {
		if store, ok := s.claims.(idempotentClaimStore); ok {
			existing, err := store.GetClaimByIdempotencyKey(ctx, idempotencyKey)
			if err == nil {
				return claimResponse(existing, idempotencyKey), nil
			}
			if !errors.Is(err, domain.ErrNotFound) {
				return ClaimResponse{}, err
			}
		}
	}

	if configuredStatus(s.cfg.FaucetMode) != domain.FaucetStatusActive {
		return ClaimResponse{}, fmt.Errorf("faucet mode is %s", configuredStatus(s.cfg.FaucetMode))
	}

	remaining, _, err := s.cooldown(ctx, request.Address)
	if err != nil {
		return ClaimResponse{}, err
	}
	if remaining > 0 {
		return ClaimResponse{}, fmt.Errorf("address cooldown active: retry after %d seconds", remaining)
	}

	if err := s.enforceRateLimits(ctx, request.Address); err != nil {
		return ClaimResponse{}, err
	}

	id, err := s.generateClaimID()
	if err != nil {
		return ClaimResponse{}, fmt.Errorf("generate claim id: %w", err)
	}
	now := s.now()
	claim := domain.Claim{
		ID:        id,
		Address:   request.Address,
		AmountWei: copyBigInt(s.cfg.AmountWei),
		Status:    domain.ClaimStatusReceived,
		CreatedAt: now,
		UpdatedAt: now,
	}

	created, err := s.createClaim(ctx, claim, idempotencyKey)
	if err != nil {
		return ClaimResponse{}, err
	}
	if created.ID != claim.ID {
		return claimResponse(created, idempotencyKey), nil
	}

	if err := s.queue.Enqueue(ctx, created.ID); err != nil {
		return ClaimResponse{}, fmt.Errorf("enqueue claim: %w", err)
	}

	queued, err := s.claims.GetClaim(ctx, created.ID)
	if err != nil {
		return ClaimResponse{}, err
	}
	return claimResponse(queued, idempotencyKey), nil
}

func (s *PersistentReadService) GetClaim(ctx context.Context, id string) (ClaimResponse, bool, error) {
	claim, err := s.claims.GetClaim(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return ClaimResponse{}, false, nil
		}
		return ClaimResponse{}, false, err
	}
	return claimResponse(claim, ""), true, nil
}

func (s *PersistentReadService) createClaim(ctx context.Context, claim domain.Claim, idempotencyKey string) (domain.Claim, error) {
	if idempotencyKey != "" {
		if store, ok := s.claims.(idempotentClaimStore); ok {
			return store.CreateClaimWithIdempotency(ctx, claim, idempotencyKey)
		}
	}
	return s.claims.CreateClaim(ctx, claim)
}

func (s *PersistentReadService) cooldown(ctx context.Context, address common.Address) (int, time.Time, error) {
	if s.cfg.CooldownSeconds <= 0 {
		return 0, time.Time{}, nil
	}

	last, err := s.claims.LastClaimByAddress(ctx, address)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return 0, time.Time{}, nil
		}
		return 0, time.Time{}, err
	}

	nextEligible := last.CreatedAt.Add(time.Duration(s.cfg.CooldownSeconds) * time.Second)
	remaining := nextEligible.Sub(s.now())
	if remaining <= 0 {
		return 0, nextEligible, nil
	}
	return int(math.Ceil(remaining.Seconds())), nextEligible, nil
}

func (s *PersistentReadService) enforceRateLimits(ctx context.Context, address common.Address) error {
	if s.rateLimiter == nil || s.cfg.RateLimitAddrPerDay <= 0 {
		return nil
	}

	decision, err := s.rateLimiter.Allow(ctx, "addr:"+strings.ToLower(address.Hex()), s.cfg.RateLimitAddrPerDay, 24*time.Hour)
	if err != nil {
		return err
	}
	if !decision.Allowed {
		if decision.Reason != "" {
			return errors.New(decision.Reason)
		}
		return errors.New("address rate limit exceeded")
	}
	return nil
}

func configuredStatus(mode string) domain.FaucetStatus {
	switch domain.FaucetStatus(strings.TrimSpace(mode)) {
	case domain.FaucetStatusPaused:
		return domain.FaucetStatusPaused
	case domain.FaucetStatusMaintenance:
		return domain.FaucetStatusMaintenance
	default:
		return domain.FaucetStatusActive
	}
}

func copyBigInt(v *big.Int) *big.Int {
	if v == nil {
		return big.NewInt(0)
	}
	return new(big.Int).Set(v)
}

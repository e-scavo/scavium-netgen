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

type dailyBudgetStore interface {
	DailyClaimAmountWei(ctx context.Context, dayStart, dayEnd time.Time, statuses []domain.ClaimStatus) (*big.Int, error)
}

type budgetedClaimStore interface {
	CreateClaimWithIdempotencyAndDailyBudget(ctx context.Context, claim domain.Claim, idempotencyKey string, dayStart, dayEnd time.Time, budgetWei *big.Int, statuses []domain.ClaimStatus) (domain.Claim, *big.Int, bool, error)
}

type tokenBudgetedClaimStore interface {
	CreateClaimWithIdempotencyAndDailyBudgetForToken(ctx context.Context, claim domain.Claim, idempotencyKey string, tokenID string, dayStart, dayEnd time.Time, budgetWei *big.Int, statuses []domain.ClaimStatus) (domain.Claim, *big.Int, bool, error)
}

// PersistentReadService implements ReadService using durable stores.
type PersistentReadService struct {
	cfg             config.Config
	claims          domain.ClaimStore
	queue           domain.QueueStore
	rateLimiter     domain.RateLimiter
	captchaVerifier domain.CaptchaVerifier
	riskEngine      domain.RiskEngine
	abuseSignals    domain.AbuseSignalRecorder
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

// SetCaptchaVerifier enables captcha verification for claim creation.
func (s *PersistentReadService) SetCaptchaVerifier(verifier domain.CaptchaVerifier) {
	s.captchaVerifier = verifier
}

// SetRiskEngine enables anti-abuse risk evaluation for claim creation.
func (s *PersistentReadService) SetRiskEngine(engine domain.RiskEngine) {
	s.riskEngine = engine
}

// SetAbuseSignalRecorder enables durable claim intake signal recording.
func (s *PersistentReadService) SetAbuseSignalRecorder(recorder domain.AbuseSignalRecorder) {
	s.abuseSignals = recorder
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
func (s *PersistentReadService) Tokens(context.Context) ([]TokenResponse, error) {
	return tokenResponses(s.cfg.NormalizedTokens()), nil
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
		return ClaimResponse{}, claimError(ErrFaucetUnavailable, fmt.Sprintf("faucet mode is %s", configuredStatus(s.cfg.FaucetMode)))
	}

	if err := s.verifyCaptcha(ctx, request); err != nil {
		return ClaimResponse{}, err
	}
	if err := s.evaluateRisk(ctx, request); err != nil {
		return ClaimResponse{}, err
	}

	remaining, _, err := s.cooldown(ctx, request.Address)
	if err != nil {
		return ClaimResponse{}, err
	}
	if remaining > 0 {
		reason := fmt.Sprintf("retry after %d seconds", remaining)
		s.recordAbuseSignal(ctx, request, domain.AbuseSignalCooldownActive, "", reason, 0)
		return ClaimResponse{}, claimRetryError(ErrCooldownActive, reason, remaining)
	}

	if err := s.enforceRateLimits(ctx, request); err != nil {
		reason := err.Error()
		var claimErr *ClaimError
		if errors.As(err, &claimErr) && claimErr.Reason != "" {
			reason = claimErr.Reason
		}
		s.recordAbuseSignal(ctx, request, domain.AbuseSignalRateLimited, "", reason, 0)
		return ClaimResponse{}, err
	}

	id, err := s.generateClaimID()
	if err != nil {
		return ClaimResponse{}, fmt.Errorf("generate claim id: %w", err)
	}
	now := s.now()
	token, ok := s.cfg.TokenByID(request.TokenID)
	if !ok {
		return ClaimResponse{}, claimError(ErrClaimRejected, "unsupported token")
	}
	claim := domain.Claim{
		ID:            id,
		Address:       request.Address,
		TokenID:       token.ID,
		TokenSymbol:   token.Symbol,
		TokenType:     token.Type,
		TokenAddress:  token.Address,
		TokenDecimals: token.Decimals,
		AmountWei:     copyBigInt(token.AmountWei),
		Status:        domain.ClaimStatusReceived,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	created, err := s.createClaim(ctx, claim, idempotencyKey)
	if err != nil {
		if errors.Is(err, ErrDailyBudgetExceeded) {
			var claimErr *ClaimError
			reason := err.Error()
			if errors.As(err, &claimErr) && claimErr.Reason != "" {
				reason = claimErr.Reason
			}
			s.recordAbuseSignal(ctx, request, domain.AbuseSignalDailyBudgetExceeded, "", reason, 0)
		}
		return ClaimResponse{}, err
	}
	if created.ID != claim.ID {
		return claimResponse(created, idempotencyKey), nil
	}

	s.recordAbuseSignal(ctx, request, domain.AbuseSignalClaimAccepted, created.ID, "", 0)

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
	if budget := s.dailyBudgetForClaim(claim); budget != nil && budget.Sign() > 0 {
		dayStart, dayEnd := utcDayWindow(s.now())
		if store, ok := s.claims.(tokenBudgetedClaimStore); ok {
			created, used, exceeded, err := store.CreateClaimWithIdempotencyAndDailyBudgetForToken(ctx, claim, idempotencyKey, claim.TokenID, dayStart, dayEnd, copyBigInt(budget), dailyBudgetStatuses())
			if err != nil {
				return domain.Claim{}, err
			}
			if exceeded {
				return domain.Claim{}, dailyBudgetExceededError(used, copyBigInt(claim.AmountWei), copyBigInt(budget))
			}
			return created, nil
		}
		if store, ok := s.claims.(budgetedClaimStore); ok {
			created, used, exceeded, err := store.CreateClaimWithIdempotencyAndDailyBudget(ctx, claim, idempotencyKey, dayStart, dayEnd, copyBigInt(budget), dailyBudgetStatuses())
			if err != nil {
				return domain.Claim{}, err
			}
			if exceeded {
				return domain.Claim{}, dailyBudgetExceededError(used, copyBigInt(claim.AmountWei), copyBigInt(budget))
			}
			return created, nil
		}
		if err := s.enforceDailyBudget(ctx); err != nil {
			return domain.Claim{}, err
		}
	}

	if idempotencyKey != "" {
		if store, ok := s.claims.(idempotentClaimStore); ok {
			return store.CreateClaimWithIdempotency(ctx, claim, idempotencyKey)
		}
	}
	return s.claims.CreateClaim(ctx, claim)
}

func (s *PersistentReadService) enforceDailyBudget(ctx context.Context) error {
	budget := s.dailyBudgetForTokenID("")
	if budget == nil || budget.Sign() <= 0 {
		return nil
	}
	store, ok := s.claims.(dailyBudgetStore)
	if !ok {
		return fmt.Errorf("daily budget store unavailable")
	}

	dayStart, dayEnd := utcDayWindow(s.now())
	used, err := store.DailyClaimAmountWei(ctx, dayStart, dayEnd, dailyBudgetStatuses())
	if err != nil {
		return err
	}
	claimAmount := copyBigInt(s.cfg.DefaultToken().AmountWei)
	nextTotal := new(big.Int).Add(used, claimAmount)
	if nextTotal.Cmp(budget) > 0 {
		return dailyBudgetExceededError(used, claimAmount, budget)
	}
	return nil
}

func (s *PersistentReadService) dailyBudgetForClaim(claim domain.Claim) *big.Int {
	if claim.TokenID != "" {
		if token, ok := s.cfg.TokenByID(claim.TokenID); ok {
			return copyBigIntOrNil(token.DailyBudgetWei)
		}
	}
	return s.dailyBudgetForTokenID("")
}

func (s *PersistentReadService) dailyBudgetForTokenID(tokenID string) *big.Int {
	token, ok := s.cfg.TokenByID(tokenID)
	if ok && token.DailyBudgetWei != nil {
		return copyBigInt(token.DailyBudgetWei)
	}
	return copyBigIntOrNil(s.cfg.DailyBudgetWei)
}

func dailyBudgetExceededError(used, claimAmount, budget *big.Int) error {
	reason := fmt.Sprintf("daily budget exceeded: used %s + claim %s > budget %s", used.String(), claimAmount.String(), budget.String())
	return claimError(ErrDailyBudgetExceeded, reason)
}

func utcDayWindow(t time.Time) (time.Time, time.Time) {
	now := t.UTC()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return dayStart, dayStart.Add(24 * time.Hour)
}

func dailyBudgetStatuses() []domain.ClaimStatus {
	return []domain.ClaimStatus{
		domain.ClaimStatusReceived,
		domain.ClaimStatusValidated,
		domain.ClaimStatusQueued,
		domain.ClaimStatusSending,
		domain.ClaimStatusSent,
		domain.ClaimStatusConfirmed,
	}
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

func (s *PersistentReadService) verifyCaptcha(ctx context.Context, request ClaimRequest) error {
	if s.captchaVerifier == nil {
		return nil
	}
	decision, err := s.captchaVerifier.Verify(ctx, strings.TrimSpace(request.CaptchaToken), strings.TrimSpace(request.RemoteIP))
	if err != nil {
		return err
	}
	if decision.Passed {
		s.recordAbuseSignal(ctx, request, domain.AbuseSignalCaptchaPassed, "", decision.Reason, 0)
		return nil
	}
	reason := decision.Reason
	s.recordAbuseSignal(ctx, request, domain.AbuseSignalCaptchaFailed, "", reason, 0)
	if reason != "" {
		return claimError(ErrCaptchaFailed, reason)
	}
	return claimError(ErrCaptchaFailed, "")
}

func (s *PersistentReadService) evaluateRisk(ctx context.Context, request ClaimRequest) error {
	if s.riskEngine == nil {
		return nil
	}
	decision, err := s.riskEngine.Evaluate(ctx, domain.RiskInput{
		Address:     request.Address,
		RemoteIP:    strings.TrimSpace(request.RemoteIP),
		Fingerprint: strings.TrimSpace(request.Fingerprint),
		UserAgent:   strings.TrimSpace(request.UserAgent),
		RequestedAt: s.now(),
	})
	if err != nil {
		return err
	}
	if decision.Allowed {
		s.recordAbuseSignal(ctx, request, domain.AbuseSignalRiskAllowed, "", decision.Reason, decision.Score)
		return nil
	}
	reason := decision.Reason
	if reason == "" {
		reason = "claim rejected by risk engine"
	}
	s.recordAbuseSignal(ctx, request, domain.AbuseSignalRiskRejected, "", reason, decision.Score)
	return claimError(ErrClaimRejected, reason)
}

func (s *PersistentReadService) enforceRateLimits(ctx context.Context, request ClaimRequest) error {
	if s.rateLimiter == nil {
		return nil
	}

	if err := s.allowRateLimit(ctx, "ip:"+strings.TrimSpace(request.RemoteIP), s.cfg.RateLimitIPPerHour, time.Hour, "IP rate limit exceeded"); err != nil {
		return err
	}
	if err := s.allowRateLimit(ctx, "addr:"+strings.ToLower(request.Address.Hex()), s.cfg.RateLimitAddrPerDay, 24*time.Hour, "address rate limit exceeded"); err != nil {
		return err
	}
	if err := s.allowRateLimit(ctx, "fp:"+strings.ToLower(strings.TrimSpace(request.Fingerprint)), s.cfg.RateLimitIPPerHour, time.Hour, "fingerprint rate limit exceeded"); err != nil {
		return err
	}
	return nil
}

func (s *PersistentReadService) recordAbuseSignal(ctx context.Context, request ClaimRequest, kind domain.AbuseSignalKind, claimID, reason string, score int) {
	if s.abuseSignals == nil {
		return
	}
	_ = s.abuseSignals.RecordAbuseSignal(ctx, domain.AbuseSignal{
		Kind:        kind,
		Address:     request.Address,
		RemoteIP:    strings.TrimSpace(request.RemoteIP),
		Fingerprint: strings.TrimSpace(request.Fingerprint),
		UserAgent:   strings.TrimSpace(request.UserAgent),
		ClaimID:     strings.TrimSpace(claimID),
		Reason:      strings.TrimSpace(reason),
		Score:       score,
		CreatedAt:   s.now(),
	})
}

func (s *PersistentReadService) allowRateLimit(ctx context.Context, key string, limit int, window time.Duration, fallbackReason string) error {
	if limit <= 0 || key == "" || strings.HasSuffix(key, ":") {
		return nil
	}
	decision, err := s.rateLimiter.Allow(ctx, key, limit, window)
	if err != nil {
		return err
	}
	if decision.Allowed {
		return nil
	}
	retryAfterSeconds := int(math.Ceil(decision.RetryAfter.Seconds()))
	if decision.Reason != "" {
		return claimRetryError(ErrRateLimited, decision.Reason, retryAfterSeconds)
	}
	return claimRetryError(ErrRateLimited, fallbackReason, retryAfterSeconds)
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

func copyBigIntOrNil(v *big.Int) *big.Int {
	if v == nil {
		return nil
	}
	return new(big.Int).Set(v)
}

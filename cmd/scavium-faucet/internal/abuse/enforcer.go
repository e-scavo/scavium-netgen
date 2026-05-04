package abuse

import (
	"context"
	"fmt"
	"strings"
	"time"

	"scavium-netgen/cmd/scavium-faucet/internal/config"
	"scavium-netgen/cmd/scavium-faucet/internal/domain"
)

// ProgressiveEnforcer converts accumulated abuse signals into conservative
// temporary deny decisions. It does not own persistence and can be disabled via
// configuration without changing the public claim API.
type ProgressiveEnforcer struct {
	cfg     config.Config
	counter domain.AbuseSignalCounter
	now     func() time.Time
}

// NewProgressiveEnforcer creates an enforcer backed by the given signal counter.
func NewProgressiveEnforcer(cfg config.Config, counter domain.AbuseSignalCounter) *ProgressiveEnforcer {
	return &ProgressiveEnforcer{
		cfg:     cfg,
		counter: counter,
		now:     time.Now,
	}
}

// WithClock replaces the time source. It is intended for deterministic tests.
func (e *ProgressiveEnforcer) WithClock(now func() time.Time) *ProgressiveEnforcer {
	if now != nil {
		e.now = now
	}
	return e
}

// Evaluate returns a risk decision using only recent negative signals.
func (e *ProgressiveEnforcer) Evaluate(ctx context.Context, input domain.RiskInput) (domain.RiskDecision, error) {
	if e == nil || e.counter == nil || !e.cfg.AbuseEnforcementEnabled {
		return domain.RiskDecision{Allowed: true, Reason: "abuse enforcement disabled"}, nil
	}

	windowSeconds := e.cfg.AbuseEnforcementWindowSeconds
	if windowSeconds <= 0 {
		windowSeconds = int(time.Hour.Seconds())
	}
	window := time.Duration(windowSeconds) * time.Second
	since := e.now().UTC().Add(-window)
	negativeKinds := []domain.AbuseSignalKind{
		domain.AbuseSignalCaptchaFailed,
		domain.AbuseSignalRiskRejected,
		domain.AbuseSignalRateLimited,
		domain.AbuseSignalCooldownActive,
		domain.AbuseSignalDailyBudgetExceeded,
	}

	if decision, err := e.evaluateScope(ctx, "ip", strings.TrimSpace(input.RemoteIP), e.cfg.AbuseEnforcementIPThreshold, domain.AbuseSignalFilter{Kinds: negativeKinds, RemoteIP: input.RemoteIP, Since: since}, window); err != nil || !decision.Allowed {
		return decision, err
	}
	if decision, err := e.evaluateScope(ctx, "address", input.Address.Hex(), e.cfg.AbuseEnforcementAddressThreshold, domain.AbuseSignalFilter{Kinds: negativeKinds, Address: input.Address, Since: since}, window); err != nil || !decision.Allowed {
		return decision, err
	}
	if decision, err := e.evaluateScope(ctx, "fingerprint", strings.TrimSpace(input.Fingerprint), e.cfg.AbuseEnforcementFingerprintThreshold, domain.AbuseSignalFilter{Kinds: negativeKinds, Fingerprint: input.Fingerprint, Since: since}, window); err != nil || !decision.Allowed {
		return decision, err
	}

	return domain.RiskDecision{Allowed: true, Reason: "progressive abuse enforcement passed"}, nil
}

func (e *ProgressiveEnforcer) evaluateScope(ctx context.Context, scope, value string, threshold int, filter domain.AbuseSignalFilter, window time.Duration) (domain.RiskDecision, error) {
	if threshold <= 0 || strings.TrimSpace(value) == "" {
		return domain.RiskDecision{Allowed: true}, nil
	}
	count, err := e.counter.CountRecentAbuseSignals(ctx, filter)
	if err != nil {
		return domain.RiskDecision{}, err
	}
	if count < threshold {
		return domain.RiskDecision{Allowed: true}, nil
	}
	return domain.RiskDecision{
		Allowed: false,
		Score:   count,
		Reason:  fmt.Sprintf("progressive abuse enforcement: %s exceeded %d negative signals in %s", scope, threshold, window),
	}, nil
}

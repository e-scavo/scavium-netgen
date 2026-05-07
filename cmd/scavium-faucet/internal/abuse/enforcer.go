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
	return &ProgressiveEnforcer{cfg: cfg, counter: counter, now: time.Now}
}

// WithClock replaces the time source. It is intended for deterministic tests.
func (e *ProgressiveEnforcer) WithClock(now func() time.Time) *ProgressiveEnforcer {
	if now != nil {
		e.now = now
	}
	return e
}

// Evaluate returns a risk decision using recent negative signals plus bounded
// Phase 27 heuristics. All labels/reasons are static categories and never embed
// raw IP, address, fingerprint, user-agent, or honeypot values.
func (e *ProgressiveEnforcer) Evaluate(ctx context.Context, input domain.RiskInput) (domain.RiskDecision, error) {
	if e == nil || e.counter == nil || !e.cfg.AbuseEnforcementEnabled {
		return domain.RiskDecision{Allowed: true, Reason: "abuse enforcement disabled"}, nil
	}
	if e.cfg.AbuseHoneypotEnabled && strings.TrimSpace(input.Honeypot) != "" {
		return domain.RiskDecision{Allowed: false, Score: riskRejectThreshold(e.cfg), Reason: "honeypot challenge failed", Review: true}, nil
	}

	window := enforcementWindow(e.cfg)
	since := e.now().UTC().Add(-window)
	negativeKinds := []domain.AbuseSignalKind{domain.AbuseSignalCaptchaFailed, domain.AbuseSignalRiskRejected, domain.AbuseSignalRateLimited, domain.AbuseSignalCooldownActive, domain.AbuseSignalDailyBudgetExceeded}

	score := 0
	if v, err := e.scopeScore(ctx, "ip", strings.TrimSpace(input.RemoteIP), e.cfg.AbuseEnforcementIPThreshold, domain.AbuseSignalFilter{Kinds: negativeKinds, RemoteIP: input.RemoteIP, Since: since}, window); err != nil || v > 0 {
		if err != nil {
			return domain.RiskDecision{}, err
		}
		return rejected(v, "progressive abuse enforcement: ip threshold exceeded"), nil
	}
	if v, err := e.scopeScore(ctx, "address", input.Address.Hex(), e.cfg.AbuseEnforcementAddressThreshold, domain.AbuseSignalFilter{Kinds: negativeKinds, Address: input.Address, Since: since}, window); err != nil || v > 0 {
		if err != nil {
			return domain.RiskDecision{}, err
		}
		return rejected(v, "progressive abuse enforcement: address threshold exceeded"), nil
	}
	if v, err := e.scopeScore(ctx, "fingerprint", strings.TrimSpace(input.Fingerprint), e.cfg.AbuseEnforcementFingerprintThreshold, domain.AbuseSignalFilter{Kinds: negativeKinds, Fingerprint: input.Fingerprint, Since: since}, window); err != nil || v > 0 {
		if err != nil {
			return domain.RiskDecision{}, err
		}
		return rejected(v, "progressive abuse enforcement: fingerprint threshold exceeded"), nil
	}

	advanced, reason, err := e.advancedScore(ctx, input)
	if err != nil {
		return domain.RiskDecision{}, err
	}
	score += advanced
	if score >= riskRejectThreshold(e.cfg) {
		if reason == "" {
			reason = "risk score threshold exceeded"
		}
		return rejected(score, reason), nil
	}
	if score > 0 {
		return domain.RiskDecision{Allowed: true, Score: score, Reason: "risk score below rejection threshold", Review: score >= riskRejectThreshold(e.cfg)-1}, nil
	}
	return domain.RiskDecision{Allowed: true, Reason: "progressive abuse enforcement passed"}, nil
}

func (e *ProgressiveEnforcer) scopeScore(ctx context.Context, scope, value string, threshold int, filter domain.AbuseSignalFilter, window time.Duration) (int, error) {
	if threshold <= 0 || strings.TrimSpace(value) == "" {
		return 0, nil
	}
	count, err := e.counter.CountRecentAbuseSignals(ctx, filter)
	if err != nil {
		return 0, err
	}
	if count < threshold {
		return 0, nil
	}
	return count, nil
}

func (e *ProgressiveEnforcer) advancedScore(ctx context.Context, input domain.RiskInput) (int, string, error) {
	score := 0
	reason := ""
	now := e.now().UTC()
	burstWindow := time.Duration(e.cfg.AbuseBurstWindowSeconds) * time.Second
	if burstWindow <= 0 {
		burstWindow = 5 * time.Minute
	}
	activityKinds := phase27ClaimIntakeKinds()
	if e.cfg.AbuseBurstThreshold > 0 && strings.TrimSpace(input.RemoteIP) != "" {
		count, err := e.counter.CountRecentAbuseSignals(ctx, domain.AbuseSignalFilter{Kinds: activityKinds, RemoteIP: input.RemoteIP, Since: now.Add(-burstWindow)})
		if err != nil {
			return 0, "", err
		}
		if count >= e.cfg.AbuseBurstThreshold {
			score += 2
			reason = "burst detection threshold exceeded"
		}
	}
	distinct, ok := e.counter.(domain.AbuseSignalDistinctCounter)
	if !ok {
		return score, reason, nil
	}
	if e.cfg.AbuseRotatingIPThreshold > 0 && strings.TrimSpace(input.Fingerprint) != "" {
		count, err := distinct.CountDistinctRecentAbuseSignalValues(ctx, domain.AbuseSignalFilter{Kinds: activityKinds, Fingerprint: input.Fingerprint, Since: now.Add(-enforcementWindow(e.cfg))}, "remote_ip")
		if err != nil {
			return 0, "", err
		}
		if count >= e.cfg.AbuseRotatingIPThreshold {
			score += 2
			reason = "rotating IP heuristic threshold exceeded"
		}
	}
	if e.cfg.AbuseAddressClusterThreshold > 0 {
		filter := domain.AbuseSignalFilter{Kinds: activityKinds, Since: now.Add(-enforcementWindow(e.cfg))}
		if strings.TrimSpace(input.Fingerprint) != "" {
			filter.Fingerprint = input.Fingerprint
		} else {
			filter.RemoteIP = input.RemoteIP
		}
		count, err := distinct.CountDistinctRecentAbuseSignalValues(ctx, filter, "address")
		if err != nil {
			return 0, "", err
		}
		if count >= e.cfg.AbuseAddressClusterThreshold {
			score += 2
			reason = "address clustering threshold exceeded"
		}
	}
	return score, reason, nil
}

func phase27ClaimIntakeKinds() []domain.AbuseSignalKind {
	return []domain.AbuseSignalKind{
		domain.AbuseSignalCaptchaPassed,
		domain.AbuseSignalCaptchaFailed,
		domain.AbuseSignalRiskAllowed,
		domain.AbuseSignalRiskRejected,
		domain.AbuseSignalManualReview,
		domain.AbuseSignalClaimAccepted,
		domain.AbuseSignalRateLimited,
		domain.AbuseSignalCooldownActive,
		domain.AbuseSignalDailyBudgetExceeded,
		domain.AbuseSignalInvalidToken,
	}
}

func rejected(score int, reason string) domain.RiskDecision {
	return domain.RiskDecision{Allowed: false, Score: score, Reason: reason, Review: true}
}
func riskRejectThreshold(cfg config.Config) int {
	if cfg.AbuseRiskScoreRejectThreshold <= 0 {
		return 5
	}
	return cfg.AbuseRiskScoreRejectThreshold
}
func enforcementWindow(cfg config.Config) time.Duration {
	s := cfg.AbuseEnforcementWindowSeconds
	if s <= 0 {
		s = int(time.Hour.Seconds())
	}
	return time.Duration(s) * time.Second
}

func (e *ProgressiveEnforcer) evaluateScope(ctx context.Context, scope, value string, threshold int, filter domain.AbuseSignalFilter, window time.Duration) (domain.RiskDecision, error) {
	v, err := e.scopeScore(ctx, scope, value, threshold, filter, window)
	if err != nil {
		return domain.RiskDecision{}, err
	}
	if v > 0 {
		return rejected(v, fmt.Sprintf("progressive abuse enforcement: %s threshold exceeded", scope)), nil
	}
	return domain.RiskDecision{Allowed: true, Score: v}, nil
}

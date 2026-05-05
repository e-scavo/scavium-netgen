package observability

import (
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"scavium-netgen/cmd/scavium-faucet/internal/version"
)

// RuntimeMetrics holds in-process counters for lightweight operational diagnostics.
// It intentionally avoids external dependencies and process-global state so tests and
// embedded handlers can keep isolated metric sets.
type RuntimeMetrics struct {
	startedAt time.Time
	version   version.Info

	tokensMu sync.RWMutex
	tokens   map[string]*runtimeTokenMetrics

	claimsAccepted      atomic.Uint64
	claimsRejected      atomic.Uint64
	captchaFailed       atomic.Uint64
	rateLimited         atomic.Uint64
	dailyBudgetExceeded atomic.Uint64
	faucetUnavailable   atomic.Uint64
	claimUnavailable    atomic.Uint64
	claimRejectedByRisk atomic.Uint64
	invalidToken        atomic.Uint64
}

// RuntimeMetricsSnapshot is the JSON-safe representation returned by the metrics endpoint.
type RuntimeMetricsSnapshot struct {
	StartedAt     string                  `json:"started_at"`
	UptimeSeconds int64                   `json:"uptime_seconds"`
	Build         RuntimeMetricsBuild     `json:"build"`
	Claims        RuntimeClaimMetrics     `json:"claims"`
	Captcha       RuntimeCaptchaMetrics   `json:"captcha"`
	RateLimits    RuntimeRateLimitMetrics `json:"rate_limits"`
	Budgets       RuntimeBudgetMetrics    `json:"budgets"`
	Tokens        []RuntimeTokenMetrics   `json:"tokens,omitempty"`
}

// RuntimeMetricsBuild exposes build metadata alongside runtime counters.
type RuntimeMetricsBuild struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
}

// RuntimeClaimMetrics exposes claim-flow counters.
type RuntimeClaimMetrics struct {
	Accepted          uint64 `json:"accepted"`
	Rejected          uint64 `json:"rejected"`
	RejectedByRisk    uint64 `json:"rejected_by_risk"`
	FaucetUnavailable uint64 `json:"faucet_unavailable"`
	ClaimUnavailable  uint64 `json:"claim_unavailable"`
	InvalidToken      uint64 `json:"invalid_token"`
}

// RuntimeCaptchaMetrics exposes captcha-related claim counters.
type RuntimeCaptchaMetrics struct {
	Failed uint64 `json:"failed"`
}

// RuntimeRateLimitMetrics exposes rate-limit claim counters.
type RuntimeRateLimitMetrics struct {
	Limited uint64 `json:"limited"`
}

// RuntimeBudgetMetrics exposes budget-related claim counters.
type RuntimeBudgetMetrics struct {
	DailyExceeded uint64 `json:"daily_exceeded"`
}

// RuntimeTokenMetrics exposes token-scoped claim counters for operational diagnostics.
// The special token id "default" represents claims that omitted token_id at the API boundary.
type RuntimeTokenMetrics struct {
	TokenID       string `json:"token_id"`
	Accepted      uint64 `json:"accepted"`
	Rejected      uint64 `json:"rejected"`
	RateLimited   uint64 `json:"rate_limited"`
	DailyExceeded uint64 `json:"daily_exceeded"`
	InvalidToken  uint64 `json:"invalid_token"`
}

type runtimeTokenMetrics struct {
	accepted      atomic.Uint64
	rejected      atomic.Uint64
	rateLimited   atomic.Uint64
	dailyExceeded atomic.Uint64
	invalidToken  atomic.Uint64
}

// NewRuntimeMetrics creates an isolated runtime metrics registry.
func NewRuntimeMetrics(info version.Info) *RuntimeMetrics {
	return NewRuntimeMetricsWithClock(info, time.Now)
}

// NewRuntimeMetricsWithClock creates an isolated runtime metrics registry with
// an injectable clock for deterministic tests.
func NewRuntimeMetricsWithClock(info version.Info, now func() time.Time) *RuntimeMetrics {
	if now == nil {
		now = time.Now
	}
	return &RuntimeMetrics{
		startedAt: now().UTC(),
		version:   info,
		tokens:    map[string]*runtimeTokenMetrics{},
	}
}

// IncClaimAccepted increments the successful claim counter.
func (m *RuntimeMetrics) IncClaimAccepted() {
	m.IncClaimAcceptedForToken("")
}

// IncClaimAcceptedForToken increments aggregate and token-scoped accepted claim counters.
func (m *RuntimeMetrics) IncClaimAcceptedForToken(tokenID string) {
	if m == nil {
		return
	}
	m.claimsAccepted.Add(1)
	m.tokenMetrics(tokenID).accepted.Add(1)
}

// IncClaimRejected increments the aggregate rejected claim counter and the
// matching classified counter when the code is known.
func (m *RuntimeMetrics) IncClaimRejected(code string) {
	m.IncClaimRejectedForToken("", code)
}

// IncClaimRejectedForToken increments aggregate and token-scoped rejected claim counters.
func (m *RuntimeMetrics) IncClaimRejectedForToken(tokenID, code string) {
	if m == nil {
		return
	}
	m.claimsRejected.Add(1)
	tokenMetrics := m.tokenMetrics(tokenID)
	tokenMetrics.rejected.Add(1)
	switch code {
	case "captcha_failed":
		m.captchaFailed.Add(1)
	case "rate_limited":
		m.rateLimited.Add(1)
		tokenMetrics.rateLimited.Add(1)
	case "daily_budget_exceeded":
		m.dailyBudgetExceeded.Add(1)
		tokenMetrics.dailyExceeded.Add(1)
	case "faucet_unavailable":
		m.faucetUnavailable.Add(1)
	case "invalid_token":
		m.invalidToken.Add(1)
		tokenMetrics.invalidToken.Add(1)
	case "claim_rejected":
		m.claimRejectedByRisk.Add(1)
	default:
		m.claimUnavailable.Add(1)
	}
}

// Snapshot returns a consistent point-in-time view of runtime counters.
func (m *RuntimeMetrics) Snapshot(now time.Time) RuntimeMetricsSnapshot {
	if m == nil {
		m = NewRuntimeMetrics(version.Info{})
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	uptime := now.Sub(m.startedAt)
	if uptime < 0 {
		uptime = 0
	}

	return RuntimeMetricsSnapshot{
		StartedAt:     m.startedAt.Format(time.RFC3339),
		UptimeSeconds: int64(uptime.Seconds()),
		Build: RuntimeMetricsBuild{
			Version:   m.version.Version,
			Commit:    m.version.Commit,
			BuildDate: m.version.BuildDate,
		},
		Claims: RuntimeClaimMetrics{
			Accepted:          m.claimsAccepted.Load(),
			Rejected:          m.claimsRejected.Load(),
			RejectedByRisk:    m.claimRejectedByRisk.Load(),
			FaucetUnavailable: m.faucetUnavailable.Load(),
			ClaimUnavailable:  m.claimUnavailable.Load(),
			InvalidToken:      m.invalidToken.Load(),
		},
		Captcha: RuntimeCaptchaMetrics{
			Failed: m.captchaFailed.Load(),
		},
		RateLimits: RuntimeRateLimitMetrics{
			Limited: m.rateLimited.Load(),
		},
		Budgets: RuntimeBudgetMetrics{
			DailyExceeded: m.dailyBudgetExceeded.Load(),
		},
		Tokens: m.tokenSnapshots(),
	}
}

func (m *RuntimeMetrics) tokenMetrics(tokenID string) *runtimeTokenMetrics {
	key := normalizeMetricsTokenID(tokenID)
	m.tokensMu.RLock()
	existing := m.tokens[key]
	m.tokensMu.RUnlock()
	if existing != nil {
		return existing
	}

	m.tokensMu.Lock()
	defer m.tokensMu.Unlock()
	if existing = m.tokens[key]; existing != nil {
		return existing
	}
	created := &runtimeTokenMetrics{}
	m.tokens[key] = created
	return created
}

func (m *RuntimeMetrics) tokenSnapshots() []RuntimeTokenMetrics {
	m.tokensMu.RLock()
	keys := make([]string, 0, len(m.tokens))
	for key := range m.tokens {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]RuntimeTokenMetrics, 0, len(keys))
	for _, key := range keys {
		counters := m.tokens[key]
		out = append(out, RuntimeTokenMetrics{
			TokenID:       key,
			Accepted:      counters.accepted.Load(),
			Rejected:      counters.rejected.Load(),
			RateLimited:   counters.rateLimited.Load(),
			DailyExceeded: counters.dailyExceeded.Load(),
			InvalidToken:  counters.invalidToken.Load(),
		})
	}
	m.tokensMu.RUnlock()
	return out
}

func normalizeMetricsTokenID(tokenID string) string {
	trimmed := strings.TrimSpace(tokenID)
	if trimmed == "" {
		return "default"
	}
	return trimmed
}

package observability

import (
	"testing"
	"time"

	"scavium-netgen/cmd/scavium-faucet/internal/version"
)

func TestRuntimeMetricsSnapshot(t *testing.T) {
	startedAt := time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC)
	metrics := NewRuntimeMetricsWithClock(version.Info{
		Version:   "v1.2.3",
		Commit:    "abc123",
		BuildDate: "2026-05-04T09:00:00Z",
	}, func() time.Time { return startedAt })

	metrics.IncClaimAcceptedForToken("native")
	metrics.IncClaimAccepted()
	metrics.IncClaimRejected("claim_rejected")
	metrics.IncClaimRejected("captcha_failed")
	metrics.IncClaimRejectedForToken("native", "rate_limited")
	metrics.IncClaimRejectedForToken("erc20", "daily_budget_exceeded")
	metrics.IncClaimRejected("faucet_unavailable")
	metrics.IncClaimRejected("claim_unavailable")
	metrics.IncClaimRejectedForToken("missing-token", "invalid_token")

	snapshot := metrics.Snapshot(startedAt.Add(90 * time.Second))
	if snapshot.StartedAt != "2026-05-04T10:00:00Z" {
		t.Fatalf("started_at = %q", snapshot.StartedAt)
	}
	if snapshot.UptimeSeconds != 90 {
		t.Fatalf("uptime_seconds = %d, want 90", snapshot.UptimeSeconds)
	}
	if snapshot.Build.Version != "v1.2.3" || snapshot.Build.Commit != "abc123" {
		t.Fatalf("build = %#v", snapshot.Build)
	}
	if snapshot.Claims.Accepted != 2 {
		t.Fatalf("claims.accepted = %d", snapshot.Claims.Accepted)
	}
	if snapshot.Claims.Rejected != 7 {
		t.Fatalf("claims.rejected = %d", snapshot.Claims.Rejected)
	}
	if snapshot.Claims.RejectedByRisk != 1 {
		t.Fatalf("claims.rejected_by_risk = %d", snapshot.Claims.RejectedByRisk)
	}
	if snapshot.Claims.FaucetUnavailable != 1 {
		t.Fatalf("claims.faucet_unavailable = %d", snapshot.Claims.FaucetUnavailable)
	}
	if snapshot.Claims.ClaimUnavailable != 1 {
		t.Fatalf("claims.claim_unavailable = %d", snapshot.Claims.ClaimUnavailable)
	}
	if snapshot.Claims.InvalidToken != 1 {
		t.Fatalf("claims.invalid_token = %d", snapshot.Claims.InvalidToken)
	}
	if snapshot.Captcha.Failed != 1 {
		t.Fatalf("captcha.failed = %d", snapshot.Captcha.Failed)
	}
	if snapshot.RateLimits.Limited != 1 {
		t.Fatalf("rate_limits.limited = %d", snapshot.RateLimits.Limited)
	}
	if snapshot.Budgets.DailyExceeded != 1 {
		t.Fatalf("budgets.daily_exceeded = %d", snapshot.Budgets.DailyExceeded)
	}
	if len(snapshot.Tokens) != 4 {
		t.Fatalf("tokens len = %d, want 4: %#v", len(snapshot.Tokens), snapshot.Tokens)
	}
	assertTokenMetrics(t, snapshot.Tokens, RuntimeTokenMetrics{TokenID: "default", Accepted: 1, Rejected: 4})
	assertTokenMetrics(t, snapshot.Tokens, RuntimeTokenMetrics{TokenID: "erc20", Rejected: 1, DailyExceeded: 1})
	assertTokenMetrics(t, snapshot.Tokens, RuntimeTokenMetrics{TokenID: "missing-token", Rejected: 1, InvalidToken: 1})
	assertTokenMetrics(t, snapshot.Tokens, RuntimeTokenMetrics{TokenID: "native", Accepted: 1, Rejected: 1, RateLimited: 1})
}

func assertTokenMetrics(t *testing.T, tokens []RuntimeTokenMetrics, want RuntimeTokenMetrics) {
	t.Helper()
	for _, got := range tokens {
		if got.TokenID != want.TokenID {
			continue
		}
		if got.Accepted != want.Accepted || got.Rejected != want.Rejected || got.RateLimited != want.RateLimited || got.DailyExceeded != want.DailyExceeded || got.InvalidToken != want.InvalidToken {
			t.Fatalf("token metrics for %q = %#v, want %#v", want.TokenID, got, want)
		}
		return
	}
	t.Fatalf("missing token metrics for %q in %#v", want.TokenID, tokens)
}

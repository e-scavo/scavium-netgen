package observability

import (
	"fmt"
	"strings"
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
	metrics.IncClaimRejected("blocklist_rejected")
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
	if snapshot.Claims.Rejected != 8 {
		t.Fatalf("claims.rejected = %d, want 8", snapshot.Claims.Rejected)
	}
	if snapshot.Claims.RejectedByRisk != 1 {
		t.Fatalf("claims.rejected_by_risk = %d", snapshot.Claims.RejectedByRisk)
	}
	if snapshot.Abuse.BlocklistRejected != 1 {
		t.Fatalf("abuse.blocklist_rejected = %d, want 1", snapshot.Abuse.BlocklistRejected)
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
	assertTokenMetrics(t, snapshot.Tokens, RuntimeTokenMetrics{TokenID: "default", Accepted: 1, Rejected: 5})
	assertTokenMetrics(t, snapshot.Tokens, RuntimeTokenMetrics{TokenID: "erc20", Rejected: 1, DailyExceeded: 1})
	assertTokenMetrics(t, snapshot.Tokens, RuntimeTokenMetrics{TokenID: "missing-token", Rejected: 1, InvalidToken: 1})
	assertTokenMetrics(t, snapshot.Tokens, RuntimeTokenMetrics{TokenID: "native", Accepted: 1, Rejected: 1, RateLimited: 1})
}

func TestRuntimeMetricsTokenBucketsAreBounded(t *testing.T) {
	metrics := NewRuntimeMetrics(version.Info{})
	for i := 0; i < maxRuntimeTokenMetricBuckets+10; i++ {
		metrics.IncClaimRejectedForToken(fmt.Sprintf("untrusted-%d", i), "invalid_token")
	}
	snapshot := metrics.Snapshot(time.Now().UTC())
	if len(snapshot.Tokens) != maxRuntimeTokenMetricBuckets+1 {
		t.Fatalf("token buckets len = %d, want %d", len(snapshot.Tokens), maxRuntimeTokenMetricBuckets+1)
	}
	assertTokenMetrics(t, snapshot.Tokens, RuntimeTokenMetrics{TokenID: "other", Rejected: 10, InvalidToken: 10})
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

func TestRuntimeMetricsSnapshotIncludesProcessMetrics(t *testing.T) {
	startedAt := time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC)
	metrics := NewRuntimeMetricsWithClock(version.Info{}, func() time.Time { return startedAt })

	snapshot := metrics.Snapshot(startedAt)

	if snapshot.Process.Goroutines <= 0 {
		t.Fatalf("process.goroutines = %d, want > 0", snapshot.Process.Goroutines)
	}
	if snapshot.Process.CPUs <= 0 {
		t.Fatalf("process.cpus = %d, want > 0", snapshot.Process.CPUs)
	}
	if snapshot.Process.SysBytes == 0 {
		t.Fatalf("process.sys_bytes = %d, want > 0", snapshot.Process.SysBytes)
	}
	if snapshot.Process.Mallocs < snapshot.Process.Frees {
		t.Fatalf("process malloc/free counters invalid: mallocs=%d frees=%d", snapshot.Process.Mallocs, snapshot.Process.Frees)
	}
}

func TestRuntimeMetricsQueueWorkerWatcherCountersAndPrometheus(t *testing.T) {
	startedAt := time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC)
	metrics := NewRuntimeMetricsWithClock(version.Info{}, func() time.Time { return startedAt })

	metrics.IncQueueDequeued(2)
	metrics.IncQueueSendSucceeded()
	metrics.IncQueueSendFailed()
	metrics.IncQueueAckFailed()
	metrics.IncWorkerBatchFailed()
	metrics.IncWatcherPendingListed(3)
	metrics.IncWatcherRPCFailed()
	metrics.IncWatcherConfirmed()
	metrics.IncWatcherReverted()
	metrics.IncWatcherStuckFound(4)
	metrics.IncWatcherStuckFailed()
	metrics.IncClaimAcceptedForToken("native")

	snapshot := metrics.Snapshot(startedAt.Add(time.Minute))
	if snapshot.Queue.Dequeued != 2 || snapshot.Queue.SendSucceeded != 1 || snapshot.Queue.SendFailed != 1 || snapshot.Queue.AckFailed != 1 {
		t.Fatalf("queue metrics = %#v", snapshot.Queue)
	}
	if snapshot.Worker.BatchFailed != 1 {
		t.Fatalf("worker metrics = %#v", snapshot.Worker)
	}
	if snapshot.Watcher.PendingListed != 3 || snapshot.Watcher.RPCFailed != 1 || snapshot.Watcher.Confirmed != 1 || snapshot.Watcher.Reverted != 1 || snapshot.Watcher.StuckFound != 4 || snapshot.Watcher.StuckFailed != 1 {
		t.Fatalf("watcher metrics = %#v", snapshot.Watcher)
	}

	text := metrics.PrometheusText(startedAt.Add(time.Minute))
	for _, want := range []string{
		"scavium_faucet_queue_dequeued_total 2\n",
		"scavium_faucet_abuse_blocklist_rejected_total 0\n",
		"scavium_faucet_worker_batch_failed_total 1\n",
		"scavium_faucet_watcher_confirmed_total 1\n",
		"scavium_faucet_token_claims_accepted_total{token_id=\"native\"} 1\n",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("prometheus text missing %q in:\n%s", want, text)
		}
	}
}

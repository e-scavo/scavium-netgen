package observability

import (
	"bytes"
	"fmt"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"scavium-netgen/cmd/scavium-faucet/internal/version"
)

const maxRuntimeTokenMetricBuckets = 64

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
	blocklistRejected   atomic.Uint64
	invalidToken        atomic.Uint64

	queueDequeued      atomic.Uint64
	queueSendSucceeded atomic.Uint64
	queueSendFailed    atomic.Uint64
	queueAckFailed     atomic.Uint64
	workerBatchFailed  atomic.Uint64

	watcherPendingListed atomic.Uint64
	watcherRPCFailed     atomic.Uint64
	watcherConfirmed     atomic.Uint64
	watcherReverted      atomic.Uint64
	watcherStuckFound    atomic.Uint64
	watcherStuckFailed   atomic.Uint64
}

// RuntimeMetricsSnapshot is the JSON-safe representation returned by the metrics endpoint.
type RuntimeMetricsSnapshot struct {
	StartedAt     string                  `json:"started_at"`
	UptimeSeconds int64                   `json:"uptime_seconds"`
	Build         RuntimeMetricsBuild     `json:"build"`
	Process       RuntimeProcessMetrics   `json:"process"`
	Claims        RuntimeClaimMetrics     `json:"claims"`
	Abuse         RuntimeAbuseMetrics     `json:"abuse"`
	Captcha       RuntimeCaptchaMetrics   `json:"captcha"`
	RateLimits    RuntimeRateLimitMetrics `json:"rate_limits"`
	Budgets       RuntimeBudgetMetrics    `json:"budgets"`
	Tokens        []RuntimeTokenMetrics   `json:"tokens,omitempty"`
	Queue         RuntimeQueueMetrics     `json:"queue"`
	Worker        RuntimeWorkerMetrics    `json:"worker"`
	Watcher       RuntimeWatcherMetrics   `json:"watcher"`
}

// RuntimeMetricsBuild exposes build metadata alongside runtime counters.
type RuntimeMetricsBuild struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
}

// RuntimeProcessMetrics exposes process-level runtime state for admin diagnostics.
type RuntimeProcessMetrics struct {
	Goroutines       int    `json:"goroutines"`
	CPUs             int    `json:"cpus"`
	AllocBytes       uint64 `json:"alloc_bytes"`
	SysBytes         uint64 `json:"sys_bytes"`
	HeapAllocBytes   uint64 `json:"heap_alloc_bytes"`
	HeapInUseBytes   uint64 `json:"heap_in_use_bytes"`
	HeapObjects      uint64 `json:"heap_objects"`
	TotalAllocBytes  uint64 `json:"total_alloc_bytes"`
	Mallocs          uint64 `json:"mallocs"`
	Frees            uint64 `json:"frees"`
	GCCycles         uint32 `json:"gc_cycles"`
	LastGCEpochNanos uint64 `json:"last_gc_epoch_nanos"`
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

// RuntimeAbuseMetrics exposes abuse-specific counters that must remain free of
// raw IP addresses, wallet addresses, fingerprints, and blocklist values.
type RuntimeAbuseMetrics struct {
	BlocklistRejected uint64 `json:"blocklist_rejected"`
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

// RuntimeQueueMetrics exposes safe queue/worker counters.
type RuntimeQueueMetrics struct {
	Dequeued      uint64 `json:"dequeued"`
	SendSucceeded uint64 `json:"send_succeeded"`
	SendFailed    uint64 `json:"send_failed"`
	AckFailed     uint64 `json:"ack_failed"`
}

// RuntimeWorkerMetrics exposes worker poll-cycle health counters.
type RuntimeWorkerMetrics struct {
	BatchFailed uint64 `json:"batch_failed"`
}

// RuntimeWatcherMetrics exposes receipt watcher and reconciliation counters.
type RuntimeWatcherMetrics struct {
	PendingListed uint64 `json:"pending_listed"`
	RPCFailed     uint64 `json:"rpc_failed"`
	Confirmed     uint64 `json:"confirmed"`
	Reverted      uint64 `json:"reverted"`
	StuckFound    uint64 `json:"stuck_found"`
	StuckFailed   uint64 `json:"stuck_failed"`
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
	case "blocklist_rejected":
		m.blocklistRejected.Add(1)
	case "claim_rejected":
		m.claimRejectedByRisk.Add(1)
	default:
		m.claimUnavailable.Add(1)
	}
}

// IncQueueDequeued records claims picked up by the worker.
func (m *RuntimeMetrics) IncQueueDequeued(n int) {
	if m == nil || n <= 0 {
		return
	}
	m.queueDequeued.Add(uint64(n))
}

func (m *RuntimeMetrics) IncQueueSendSucceeded() {
	if m != nil {
		m.queueSendSucceeded.Add(1)
	}
}
func (m *RuntimeMetrics) IncQueueSendFailed() {
	if m != nil {
		m.queueSendFailed.Add(1)
	}
}
func (m *RuntimeMetrics) IncQueueAckFailed() {
	if m != nil {
		m.queueAckFailed.Add(1)
	}
}
func (m *RuntimeMetrics) IncWorkerBatchFailed() {
	if m != nil {
		m.workerBatchFailed.Add(1)
	}
}
func (m *RuntimeMetrics) IncWatcherPendingListed(n int) {
	if m != nil && n > 0 {
		m.watcherPendingListed.Add(uint64(n))
	}
}
func (m *RuntimeMetrics) IncWatcherRPCFailed() {
	if m != nil {
		m.watcherRPCFailed.Add(1)
	}
}
func (m *RuntimeMetrics) IncWatcherConfirmed() {
	if m != nil {
		m.watcherConfirmed.Add(1)
	}
}
func (m *RuntimeMetrics) IncWatcherReverted() {
	if m != nil {
		m.watcherReverted.Add(1)
	}
}
func (m *RuntimeMetrics) IncWatcherStuckFound(n int) {
	if m != nil && n > 0 {
		m.watcherStuckFound.Add(uint64(n))
	}
}
func (m *RuntimeMetrics) IncWatcherStuckFailed() {
	if m != nil {
		m.watcherStuckFailed.Add(1)
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
		Process: processMetrics(),
		Claims: RuntimeClaimMetrics{
			Accepted:          m.claimsAccepted.Load(),
			Rejected:          m.claimsRejected.Load(),
			RejectedByRisk:    m.claimRejectedByRisk.Load(),
			FaucetUnavailable: m.faucetUnavailable.Load(),
			ClaimUnavailable:  m.claimUnavailable.Load(),
			InvalidToken:      m.invalidToken.Load(),
		},
		Abuse: RuntimeAbuseMetrics{
			BlocklistRejected: m.blocklistRejected.Load(),
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
		Tokens:  m.tokenSnapshots(),
		Queue:   RuntimeQueueMetrics{Dequeued: m.queueDequeued.Load(), SendSucceeded: m.queueSendSucceeded.Load(), SendFailed: m.queueSendFailed.Load(), AckFailed: m.queueAckFailed.Load()},
		Worker:  RuntimeWorkerMetrics{BatchFailed: m.workerBatchFailed.Load()},
		Watcher: RuntimeWatcherMetrics{PendingListed: m.watcherPendingListed.Load(), RPCFailed: m.watcherRPCFailed.Load(), Confirmed: m.watcherConfirmed.Load(), Reverted: m.watcherReverted.Load(), StuckFound: m.watcherStuckFound.Load(), StuckFailed: m.watcherStuckFailed.Load()},
	}
}

func processMetrics() RuntimeProcessMetrics {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	return RuntimeProcessMetrics{
		Goroutines:       runtime.NumGoroutine(),
		CPUs:             runtime.NumCPU(),
		AllocBytes:       mem.Alloc,
		SysBytes:         mem.Sys,
		HeapAllocBytes:   mem.HeapAlloc,
		HeapInUseBytes:   mem.HeapInuse,
		HeapObjects:      mem.HeapObjects,
		TotalAllocBytes:  mem.TotalAlloc,
		Mallocs:          mem.Mallocs,
		Frees:            mem.Frees,
		GCCycles:         mem.NumGC,
		LastGCEpochNanos: mem.LastGC,
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
	if len(m.tokens) >= maxRuntimeTokenMetricBuckets {
		key = "other"
		if existing = m.tokens[key]; existing != nil {
			return existing
		}
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

// PrometheusText renders the metrics snapshot in a stable, dependency-free
// Prometheus-compatible text format. It intentionally uses a bounded label set
// and never includes addresses, IPs, fingerprints, idempotency keys, or secrets.
func (m *RuntimeMetrics) PrometheusText(now time.Time) string {
	s := m.Snapshot(now)
	var b bytes.Buffer
	metric := func(name, help, typ string, value any) {
		fmt.Fprintf(&b, "# HELP %s %s\n# TYPE %s %s\n%s %v\n", name, help, name, typ, name, value)
	}
	metric("scavium_faucet_uptime_seconds", "Runtime uptime in seconds.", "gauge", s.UptimeSeconds)
	metric("scavium_faucet_process_goroutines", "Current goroutine count.", "gauge", s.Process.Goroutines)
	metric("scavium_faucet_claims_accepted_total", "Accepted claim requests.", "counter", s.Claims.Accepted)
	metric("scavium_faucet_claims_rejected_total", "Rejected claim requests.", "counter", s.Claims.Rejected)
	metric("scavium_faucet_claims_rejected_by_risk_total", "Risk-engine claim rejections.", "counter", s.Claims.RejectedByRisk)
	metric("scavium_faucet_abuse_blocklist_rejected_total", "Claims rejected by blocklist policy without exposing blocked values.", "counter", s.Abuse.BlocklistRejected)
	metric("scavium_faucet_captcha_failed_total", "Captcha failures.", "counter", s.Captcha.Failed)
	metric("scavium_faucet_rate_limited_total", "Rate limited claim requests.", "counter", s.RateLimits.Limited)
	metric("scavium_faucet_daily_budget_exceeded_total", "Daily budget rejections.", "counter", s.Budgets.DailyExceeded)
	metric("scavium_faucet_invalid_token_total", "Invalid token rejections.", "counter", s.Claims.InvalidToken)
	metric("scavium_faucet_queue_dequeued_total", "Claims dequeued by the worker.", "counter", s.Queue.Dequeued)
	metric("scavium_faucet_queue_send_succeeded_total", "Worker send successes.", "counter", s.Queue.SendSucceeded)
	metric("scavium_faucet_queue_send_failed_total", "Worker send failures.", "counter", s.Queue.SendFailed)
	metric("scavium_faucet_queue_ack_failed_total", "Worker ack persistence failures.", "counter", s.Queue.AckFailed)
	metric("scavium_faucet_worker_batch_failed_total", "Worker dequeue batch failures.", "counter", s.Worker.BatchFailed)
	metric("scavium_faucet_watcher_pending_listed_total", "Pending transactions observed by watcher.", "counter", s.Watcher.PendingListed)
	metric("scavium_faucet_watcher_rpc_failed_total", "Watcher RPC/list failures.", "counter", s.Watcher.RPCFailed)
	metric("scavium_faucet_watcher_confirmed_total", "Transactions confirmed by watcher.", "counter", s.Watcher.Confirmed)
	metric("scavium_faucet_watcher_reverted_total", "Reverted transactions found by watcher.", "counter", s.Watcher.Reverted)
	metric("scavium_faucet_watcher_stuck_found_total", "Stuck sending claims found by watcher.", "counter", s.Watcher.StuckFound)
	metric("scavium_faucet_watcher_stuck_failed_total", "Stuck claim reconciliation failures.", "counter", s.Watcher.StuckFailed)
	for _, t := range s.Tokens {
		label := prometheusLabelValue(t.TokenID)
		fmt.Fprintf(&b, "scavium_faucet_token_claims_accepted_total{token_id=\"%s\"} %d\n", label, t.Accepted)
		fmt.Fprintf(&b, "scavium_faucet_token_claims_rejected_total{token_id=\"%s\"} %d\n", label, t.Rejected)
	}
	return b.String()
}

func prometheusLabelValue(v string) string {
	v = strings.ReplaceAll(v, "\\", "\\\\")
	v = strings.ReplaceAll(v, "\n", "")
	v = strings.ReplaceAll(v, "\"", "\\\"")
	return v
}

package domain

import (
	"context"
	"errors"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// ErrNotFound reports that a requested persisted domain record does not exist.
var ErrNotFound = errors.New("not found")

// ClaimStore persists claims and claim state transitions.
type ClaimStore interface {
	CreateClaim(ctx context.Context, claim Claim) (Claim, error)
	GetClaim(ctx context.Context, id string) (Claim, error)
	UpdateClaimStatus(ctx context.Context, id string, status ClaimStatus, reason string) (Claim, error)
	ListClaimsByAddress(ctx context.Context, address common.Address, limit int) ([]Claim, error)
	LastClaimByAddress(ctx context.Context, address common.Address) (Claim, error)
}

// DailyBudgetStore reports persisted claim amounts for UTC day budget checks.
type DailyBudgetStore interface {
	DailyClaimAmountWei(ctx context.Context, dayStart, dayEnd time.Time, statuses []ClaimStatus) (*big.Int, error)
}

// AbuseSignalRecorder persists claim intake abuse signals for later analysis.
type AbuseSignalRecorder interface {
	RecordAbuseSignal(ctx context.Context, signal AbuseSignal) error
}

// AbuseSignalCounter exposes aggregate signal counts for progressive enforcement.
type AbuseSignalCounter interface {
	CountRecentAbuseSignals(ctx context.Context, filter AbuseSignalFilter) (int, error)
}

// AbuseSignalDistinctCounter exposes bounded distinct counts for conservative
// Phase 27 clustering heuristics. The field argument must be one of the
// implementation-defined safe columns, never free-form user input.
type AbuseSignalDistinctCounter interface {
	CountDistinctRecentAbuseSignalValues(ctx context.Context, filter AbuseSignalFilter, field string) (int, error)
}

// AbuseSignalPruner removes old abuse signals according to the configured retention window.
type AbuseSignalPruner interface {
	PruneAbuseSignals(ctx context.Context, olderThan time.Time) (int64, error)
}

// AbuseSignalReporter exposes operational summaries for internal diagnostics.
type AbuseSignalReporter interface {
	ListAbuseSignalSummaries(ctx context.Context, since time.Time, limit int) ([]AbuseSignalSummary, error)
}

// AbuseSignalFilter scopes recent abuse signal lookups.
type AbuseSignalFilter struct {
	Kinds       []AbuseSignalKind
	Address     common.Address
	RemoteIP    string
	Fingerprint string
	Since       time.Time
}

// RateLimiter evaluates whether a key can proceed within a sliding window.
type RateLimiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (RateLimitDecision, error)
}

// RateLimitDecision is the result returned by RateLimiter.Allow.
type RateLimitDecision struct {
	Allowed    bool
	Remaining  int
	RetryAfter time.Duration
	Reason     string
}

// QueueStore coordinates claim queue lifecycle for asynchronous processing.
type QueueStore interface {
	// Enqueue transitions a claim to the 'queued' state, making it available for processing.
	Enqueue(ctx context.Context, claimID string) error
	// DequeueBatch picks up to n claims in 'queued' state whose next_attempt_at is not in the
	// future, transitions them to 'sending', and returns them.
	DequeueBatch(ctx context.Context, n int) ([]Claim, error)
	// Ack transitions a claim from 'sending' to 'sent' and persists the tx record.
	Ack(ctx context.Context, claimID string, tx Transaction) error
	// Fail increments retry_count. If retry_count reaches maxRetries the claim is dead-lettered
	// ('failed'). Otherwise it is re-queued with an exponential backoff on next_attempt_at.
	Fail(ctx context.Context, claimID string, reason string, maxRetries int) error
}

// Sender submits a claim payment transaction.
type Sender interface {
	Send(ctx context.Context, claim Claim) (Transaction, error)
}

// PendingTx holds the minimum information needed to check a transaction receipt.
type PendingTx struct {
	ClaimID string
	TxHash  common.Hash
}

// WatcherStore provides the persistence operations used by the receipt watcher.
type WatcherStore interface {
	// ListPendingTransactions returns up to limit claims in 'sent' state that have
	// an associated transaction hash awaiting confirmation.
	ListPendingTransactions(ctx context.Context, limit int) ([]PendingTx, error)
	// ConfirmTransaction marks the claim and its transaction record as 'confirmed'
	// and records the block number and gas used from the receipt.
	ConfirmTransaction(ctx context.Context, claimID string, blockNumber, gasUsed uint64) error
	// FailTransaction marks the claim and its transaction record as 'failed'.
	FailTransaction(ctx context.Context, claimID string, reason string) error
	// ListStuckSending returns claims that have been in 'sending' state for longer
	// than stuckAfter.  These represent claims whose worker died mid-flight.
	ListStuckSending(ctx context.Context, stuckAfter time.Duration, limit int) ([]Claim, error)
}

// CaptchaVerifier validates a challenge token from a given client IP.
type CaptchaVerifier interface {
	Verify(ctx context.Context, token string, remoteIP string) (CaptchaDecision, error)
}

// CaptchaDecision captures the verifier outcome.
type CaptchaDecision struct {
	Passed bool
	Reason string
}

// RiskEngine evaluates anti-abuse signals for a claim request.
type RiskEngine interface {
	Evaluate(ctx context.Context, input RiskInput) (RiskDecision, error)
}

// RiskInput is the signal set consumed by RiskEngine.
type RiskInput struct {
	Address     common.Address
	RemoteIP    string
	Fingerprint string
	UserAgent   string
	RequestedAt time.Time
	Honeypot    string
}

// RiskDecision is the risk evaluation result for a request.
type RiskDecision struct {
	Allowed bool
	Score   int
	Reason  string
	Review  bool
}

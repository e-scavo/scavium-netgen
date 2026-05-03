package domain

import (
	"context"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

type ClaimStore interface {
	CreateClaim(ctx context.Context, claim Claim) (Claim, error)
	GetClaim(ctx context.Context, id string) (Claim, error)
	UpdateClaimStatus(ctx context.Context, id string, status ClaimStatus, reason string) (Claim, error)
	ListClaimsByAddress(ctx context.Context, address common.Address, limit int) ([]Claim, error)
	LastClaimByAddress(ctx context.Context, address common.Address) (Claim, error)
}

type RateLimiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (RateLimitDecision, error)
}

type RateLimitDecision struct {
	Allowed    bool
	Remaining  int
	RetryAfter time.Duration
	Reason     string
}

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

type Sender interface {
	Send(ctx context.Context, claim Claim) (Transaction, error)
}

type CaptchaVerifier interface {
	Verify(ctx context.Context, token string, remoteIP string) (CaptchaDecision, error)
}

type CaptchaDecision struct {
	Passed bool
	Reason string
}

type RiskEngine interface {
	Evaluate(ctx context.Context, input RiskInput) (RiskDecision, error)
}

type RiskInput struct {
	Address     common.Address
	RemoteIP    string
	Fingerprint string
	UserAgent   string
	RequestedAt time.Time
}

type RiskDecision struct {
	Allowed bool
	Score   int
	Reason  string
}

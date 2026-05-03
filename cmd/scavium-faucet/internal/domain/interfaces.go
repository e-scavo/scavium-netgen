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

type Queue interface {
	Enqueue(ctx context.Context, claim Claim) error
	Dequeue(ctx context.Context) (Claim, error)
	Ack(ctx context.Context, claimID string) error
	Fail(ctx context.Context, claimID string, err error) error
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

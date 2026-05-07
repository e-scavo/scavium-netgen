package domain

import (
	"context"
	"math/big"
)

// RuntimePolicy contains the intentionally small, non-secret subset of faucet
// policy that operators may change at runtime. Nil daily budgets mean the
// persisted override is absent and the environment/default configuration remains authoritative.
type RuntimePolicy struct {
	CooldownSeconds     int
	RateLimitIPPerHour  int
	RateLimitAddrPerDay int
	DailyBudgetWei      *big.Int
	TokenDailyBudgetWei map[string]*big.Int
}

// RuntimePolicyStore durably stores runtime-editable policy overrides.
type RuntimePolicyStore interface {
	GetRuntimePolicy(ctx context.Context) (RuntimePolicy, error)
	SetRuntimePolicy(ctx context.Context, policy RuntimePolicy) error
	ClearRuntimePolicy(ctx context.Context) error
}

func CopyRuntimePolicy(in RuntimePolicy) RuntimePolicy {
	out := RuntimePolicy{CooldownSeconds: in.CooldownSeconds, RateLimitIPPerHour: in.RateLimitIPPerHour, RateLimitAddrPerDay: in.RateLimitAddrPerDay}
	if in.DailyBudgetWei != nil {
		out.DailyBudgetWei = new(big.Int).Set(in.DailyBudgetWei)
	}
	if len(in.TokenDailyBudgetWei) > 0 {
		out.TokenDailyBudgetWei = make(map[string]*big.Int, len(in.TokenDailyBudgetWei))
		for k, v := range in.TokenDailyBudgetWei {
			if v != nil {
				out.TokenDailyBudgetWei[k] = new(big.Int).Set(v)
			}
		}
	}
	return out
}

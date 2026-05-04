package faucet

import (
	"errors"
	"fmt"
)

var (
	ErrFaucetUnavailable = errors.New("faucet unavailable")
	ErrCaptchaFailed     = errors.New("captcha failed")
	ErrClaimRejected     = errors.New("claim rejected")
	ErrCooldownActive    = errors.New("cooldown active")
	ErrRateLimited       = errors.New("rate limited")
)

type ClaimError struct {
	Kind              error
	Reason            string
	RetryAfterSeconds int
}

func (e *ClaimError) Error() string {
	if e == nil {
		return ""
	}
	if e.Reason != "" {
		return fmt.Sprintf("%v: %s", e.Kind, e.Reason)
	}
	return e.Kind.Error()
}

func (e *ClaimError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Kind
}

func claimError(kind error, reason string) error {
	return &ClaimError{Kind: kind, Reason: reason}
}

func claimRetryError(kind error, reason string, retryAfterSeconds int) error {
	return &ClaimError{
		Kind:              kind,
		Reason:            reason,
		RetryAfterSeconds: retryAfterSeconds,
	}
}

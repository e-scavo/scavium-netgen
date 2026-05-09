package domain

import (
	"errors"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// ErrWalletChallengeInvalid reports an absent, expired, consumed, or mismatched wallet challenge.
var ErrWalletChallengeInvalid = errors.New("wallet challenge invalid")

// WalletChallenge stores the short-lived non-secret challenge used by optional wallet proof flows.
type WalletChallenge struct {
	ID         string
	Address    common.Address
	Nonce      string
	Message    string
	ExpiresAt  time.Time
	ConsumedAt *time.Time
	CreatedAt  time.Time
}

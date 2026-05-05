package domain

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// ClaimStatus represents the lifecycle state of a faucet claim.
type ClaimStatus string

// TokenType identifies how a faucet token is transferred on-chain.
type TokenType string

const (
	// TokenTypeNative represents the chain native coin transferred as tx value.
	TokenTypeNative TokenType = "native"
	// TokenTypeERC20 represents an ERC20 token transferred via transfer(address,uint256).
	TokenTypeERC20 TokenType = "erc20"
)

const (
	// ClaimStatusReceived indicates the request was accepted by the API.
	ClaimStatusReceived ClaimStatus = "received"
	// ClaimStatusValidated indicates input and abuse checks have passed.
	ClaimStatusValidated ClaimStatus = "validated"
	// ClaimStatusQueued indicates the claim is waiting for worker pickup.
	ClaimStatusQueued ClaimStatus = "queued"
	// ClaimStatusSending indicates a worker is currently sending the transaction.
	ClaimStatusSending ClaimStatus = "sending"
	// ClaimStatusSent indicates a transaction was broadcast and awaits receipt.
	ClaimStatusSent ClaimStatus = "sent"
	// ClaimStatusConfirmed indicates the transaction was mined successfully.
	ClaimStatusConfirmed ClaimStatus = "confirmed"
	// ClaimStatusFailed indicates processing reached a terminal failure state.
	ClaimStatusFailed ClaimStatus = "failed"
	// ClaimStatusRejected indicates the claim was intentionally denied/cancelled.
	ClaimStatusRejected ClaimStatus = "rejected"
	// ClaimStatusPaused indicates claim creation was blocked by faucet mode.
	ClaimStatusPaused ClaimStatus = "paused"
)

// Claim is the canonical persisted faucet request record.
type Claim struct {
	ID            string
	Address       common.Address
	TokenID       string
	TokenSymbol   string
	TokenType     TokenType
	TokenAddress  common.Address
	TokenDecimals int
	AmountWei     *big.Int
	Status        ClaimStatus
	Transaction   *Transaction
	Reason        string
	RetryCount    int
	NextAttemptAt *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Transaction stores chain-level data associated with a claim payout.
type Transaction struct {
	Hash          common.Hash
	From          common.Address
	To            common.Address
	TokenID       string
	TokenSymbol   string
	TokenType     TokenType
	TokenAddress  common.Address
	TokenDecimals int
	ValueWei      *big.Int
	Status        ClaimStatus
	BlockNumber   uint64
	GasUsed       uint64
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// AbuseSignalKind classifies production-safe signals captured during claim intake.
type AbuseSignalKind string

const (
	// AbuseSignalCaptchaPassed records a successful captcha verification.
	AbuseSignalCaptchaPassed AbuseSignalKind = "captcha_passed"
	// AbuseSignalCaptchaFailed records a failed captcha verification.
	AbuseSignalCaptchaFailed AbuseSignalKind = "captcha_failed"
	// AbuseSignalRiskAllowed records an allow decision from the risk engine.
	AbuseSignalRiskAllowed AbuseSignalKind = "risk_allowed"
	// AbuseSignalRiskRejected records a reject decision from the risk engine.
	AbuseSignalRiskRejected AbuseSignalKind = "risk_rejected"
	// AbuseSignalCooldownActive records a claim denied by wallet cooldown.
	AbuseSignalCooldownActive AbuseSignalKind = "cooldown_active"
	// AbuseSignalRateLimited records a claim denied by IP/address/fingerprint limits.
	AbuseSignalRateLimited AbuseSignalKind = "rate_limited"
	// AbuseSignalDailyBudgetExceeded records a claim denied by the daily faucet budget.
	AbuseSignalDailyBudgetExceeded AbuseSignalKind = "daily_budget_exceeded"
	// AbuseSignalInvalidToken records a claim denied by token catalog validation.
	AbuseSignalInvalidToken AbuseSignalKind = "invalid_token"
	// AbuseSignalClaimAccepted records a claim accepted into durable storage.
	AbuseSignalClaimAccepted AbuseSignalKind = "claim_accepted"
)

// AbuseSignal is a durable, production-safe anti-abuse observation.
// It stores request metadata required for later analysis without changing
// the public claim contract or hard-blocking behavior.
type AbuseSignal struct {
	ID          int64
	Kind        AbuseSignalKind
	Address     common.Address
	RemoteIP    string
	Fingerprint string
	UserAgent   string
	ClaimID     string
	Reason      string
	Score       int
	CreatedAt   time.Time
}

// AbuseSignalSummary is an internal operational rollup of abuse signals.
type AbuseSignalSummary struct {
	Kind  AbuseSignalKind
	Count int
}

// FaucetToken is a public-safe configured faucet asset descriptor.
type FaucetToken struct {
	ID          string
	Symbol      string
	Type        TokenType
	Address     common.Address
	Decimals    int
	AmountWei   *big.Int
	DailyBudget *big.Int
}

// IsValidTokenType reports whether tokenType is supported by the faucet sender.
func IsValidTokenType(tokenType TokenType) bool {
	switch tokenType {
	case TokenTypeNative, TokenTypeERC20:
		return true
	default:
		return false
	}
}

// FaucetConfig is the read model exposed by faucet status/config endpoints.
type FaucetConfig struct {
	NetworkName     string
	ChainID         int64
	Symbol          string
	AmountWei       *big.Int
	Tokens          []FaucetToken
	CooldownSeconds int
	ExplorerTxURL   string
	DryRun          bool
}

// FaucetStatus represents the operational availability state.
type FaucetStatus string

const (
	// FaucetStatusActive means claim requests are accepted.
	FaucetStatusActive FaucetStatus = "active"
	// FaucetStatusPaused means claims are temporarily disabled by operator action.
	FaucetStatusPaused FaucetStatus = "paused"
	// FaucetStatusMaintenance means claims are disabled for maintenance work.
	FaucetStatusMaintenance FaucetStatus = "maintenance"
	// FaucetStatusNoFunds means claims are disabled because hot wallet is empty.
	FaucetStatusNoFunds FaucetStatus = "no_funds"
)

// IsValidClaimStatus reports whether status is part of the supported claim lifecycle.
func IsValidClaimStatus(status ClaimStatus) bool {
	switch status {
	case ClaimStatusReceived,
		ClaimStatusValidated,
		ClaimStatusQueued,
		ClaimStatusSending,
		ClaimStatusSent,
		ClaimStatusConfirmed,
		ClaimStatusFailed,
		ClaimStatusRejected,
		ClaimStatusPaused:
		return true
	default:
		return false
	}
}

// IsTerminalClaimStatus reports whether no further processing should occur.
func IsTerminalClaimStatus(status ClaimStatus) bool {
	switch status {
	case ClaimStatusConfirmed, ClaimStatusFailed, ClaimStatusRejected:
		return true
	default:
		return false
	}
}

// IsValidFaucetStatus reports whether status is a supported faucet mode.
func IsValidFaucetStatus(status FaucetStatus) bool {
	switch status {
	case FaucetStatusActive,
		FaucetStatusPaused,
		FaucetStatusMaintenance,
		FaucetStatusNoFunds:
		return true
	default:
		return false
	}
}

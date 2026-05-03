package domain

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

type ClaimStatus string

const (
	ClaimStatusReceived  ClaimStatus = "received"
	ClaimStatusValidated ClaimStatus = "validated"
	ClaimStatusQueued    ClaimStatus = "queued"
	ClaimStatusSending   ClaimStatus = "sending"
	ClaimStatusSent      ClaimStatus = "sent"
	ClaimStatusConfirmed ClaimStatus = "confirmed"
	ClaimStatusFailed    ClaimStatus = "failed"
	ClaimStatusRejected  ClaimStatus = "rejected"
	ClaimStatusPaused    ClaimStatus = "paused"
)

type Claim struct {
	ID          string
	Address     common.Address
	AmountWei   *big.Int
	Status      ClaimStatus
	Transaction *Transaction
	Reason      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Transaction struct {
	Hash        common.Hash
	From        common.Address
	To          common.Address
	ValueWei    *big.Int
	Status      ClaimStatus
	BlockNumber uint64
	GasUsed     uint64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type FaucetConfig struct {
	NetworkName     string
	ChainID         int64
	Symbol          string
	AmountWei       *big.Int
	CooldownSeconds int
	ExplorerTxURL   string
	DryRun          bool
}

type FaucetStatus string

const (
	FaucetStatusActive      FaucetStatus = "active"
	FaucetStatusPaused      FaucetStatus = "paused"
	FaucetStatusMaintenance FaucetStatus = "maintenance"
	FaucetStatusNoFunds     FaucetStatus = "no_funds"
)

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

func IsTerminalClaimStatus(status ClaimStatus) bool {
	switch status {
	case ClaimStatusConfirmed, ClaimStatusFailed, ClaimStatusRejected:
		return true
	default:
		return false
	}
}

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

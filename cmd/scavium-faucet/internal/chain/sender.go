package chain

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"scavium-netgen/cmd/scavium-faucet/internal/domain"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

var _ domain.Sender = (*EthSender)(nil)
var _ domain.Sender = (*DryRunSender)(nil)

// gasLimitETHTransfer is the fixed gas cost of a plain ETH value transfer on EVM.
const gasLimitETHTransfer = uint64(21_000)

// GasPolicy holds configurable gas-price constraints for EthSender.
type GasPolicy struct {
	// MinGasPrice is the minimum gas price accepted.  If the node suggests a
	// lower value the floor is used instead.  nil means no floor.
	MinGasPrice *big.Int
}

// EthSender implements domain.Sender by submitting real transactions to an EVM node.
type EthSender struct {
	client    ChainClient
	signer    Signer
	chainID   int64
	nonceMgr  *NonceManager
	gasPolicy GasPolicy
}

// NewEthSender creates a sender that signs and broadcasts transactions.
// A NonceManager is created internally and seeded on first use.
func NewEthSender(client ChainClient, signer Signer, chainID int64, policy GasPolicy) *EthSender {
	return &EthSender{
		client:    client,
		signer:    signer,
		chainID:   chainID,
		nonceMgr:  NewNonceManager(client, signer.Address()),
		gasPolicy: policy,
	}
}

// Send builds, signs and broadcasts a legacy ETH value-transfer transaction for claim.
// It applies the configured gas floor, checks the faucet balance before sending,
// and resets the nonce manager on any send error.
func (s *EthSender) Send(ctx context.Context, claim domain.Claim) (domain.Transaction, error) {
	gasPrice, err := s.client.SuggestGasPrice(ctx)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("suggest gas price: %w", err)
	}
	// Apply minimum gas price floor when configured.
	if s.gasPolicy.MinGasPrice != nil && gasPrice.Cmp(s.gasPolicy.MinGasPrice) < 0 {
		gasPrice = new(big.Int).Set(s.gasPolicy.MinGasPrice)
	}

	// Balance guard: faucet must hold at least amountWei + gas cost.
	balance, err := s.client.BalanceAt(ctx, s.signer.Address(), nil)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("balance check: %w", err)
	}
	gasCost := new(big.Int).Mul(gasPrice, new(big.Int).SetUint64(gasLimitETHTransfer))
	required := new(big.Int).Add(claim.AmountWei, gasCost)
	if balance.Cmp(required) < 0 {
		return domain.Transaction{}, fmt.Errorf(
			"insufficient faucet balance: have %s wei, need %s wei",
			balance.String(), required.String(),
		)
	}

	nonce, err := s.nonceMgr.Next(ctx)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("get nonce: %w", err)
	}

	to := claim.Address
	rawTx := types.NewTx(&types.LegacyTx{
		Nonce:    nonce,
		To:       &to,
		Value:    claim.AmountWei,
		Gas:      gasLimitETHTransfer,
		GasPrice: gasPrice,
	})

	signed, err := s.signer.Sign(rawTx, s.chainID)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("sign tx: %w", err)
	}

	if err := s.client.SendTransaction(ctx, signed); err != nil {
		s.nonceMgr.Reset() // force re-sync on next attempt
		return domain.Transaction{}, fmt.Errorf("send transaction: %w", err)
	}

	now := time.Now().UTC()
	return domain.Transaction{
		Hash:      signed.Hash(),
		From:      s.signer.Address(),
		To:        claim.Address,
		ValueWei:  claim.AmountWei,
		Status:    domain.ClaimStatusSent,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// DryRunSender implements domain.Sender without touching the blockchain.
// It is used when DryRun=true or in tests.
type DryRunSender struct {
	from common.Address
}

// NewDryRunSender creates a no-op sender.  from is included in the returned
// Transaction for observability but no RPC calls are made.
func NewDryRunSender(from common.Address) *DryRunSender {
	return &DryRunSender{from: from}
}

// Send returns a fake Transaction with a zero hash.  No blockchain call is made.
func (s *DryRunSender) Send(_ context.Context, claim domain.Claim) (domain.Transaction, error) {
	now := time.Now().UTC()
	return domain.Transaction{
		Hash:      common.Hash{}, // zero hash — no real tx
		From:      s.from,
		To:        claim.Address,
		ValueWei:  claim.AmountWei,
		Status:    domain.ClaimStatusSent,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

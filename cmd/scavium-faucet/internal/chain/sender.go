package chain

import (
	"context"
	"fmt"
	"time"

	"scavium-netgen/cmd/scavium-faucet/internal/domain"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

var _ domain.Sender = (*EthSender)(nil)
var _ domain.Sender = (*DryRunSender)(nil)

// gasLimitETHTransfer is the fixed gas cost of a plain ETH value transfer on EVM.
const gasLimitETHTransfer = uint64(21_000)

// EthSender implements domain.Sender by submitting real transactions to an EVM node.
type EthSender struct {
	client  ChainClient
	signer  Signer
	chainID int64
}

// NewEthSender creates a sender that signs and broadcasts transactions.
func NewEthSender(client ChainClient, signer Signer, chainID int64) *EthSender {
	return &EthSender{client: client, signer: signer, chainID: chainID}
}

// Send builds, signs and broadcasts a legacy ETH value-transfer transaction for claim.
func (s *EthSender) Send(ctx context.Context, claim domain.Claim) (domain.Transaction, error) {
	nonce, err := s.client.NonceAt(ctx, s.signer.Address(), nil)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("get nonce: %w", err)
	}

	gasPrice, err := s.client.SuggestGasPrice(ctx)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("suggest gas price: %w", err)
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

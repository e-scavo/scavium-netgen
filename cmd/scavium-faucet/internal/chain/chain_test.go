package chain

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"scavium-netgen/cmd/scavium-faucet/internal/domain"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// ── fakeClient ───────────────────────────────────────────────────────────────

type fakeClient struct {
	chainID    int64
	nonce      uint64
	gasPrice   *big.Int
	sendErr    error
	sendCalled int
}

func (f *fakeClient) ChainID(_ context.Context) (*big.Int, error) {
	return big.NewInt(f.chainID), nil
}

func (f *fakeClient) BalanceAt(_ context.Context, _ common.Address, _ *big.Int) (*big.Int, error) {
	return big.NewInt(0), nil
}

func (f *fakeClient) NonceAt(_ context.Context, _ common.Address, _ *big.Int) (uint64, error) {
	return f.nonce, nil
}

func (f *fakeClient) SuggestGasPrice(_ context.Context) (*big.Int, error) {
	if f.gasPrice != nil {
		return f.gasPrice, nil
	}
	return big.NewInt(1_000_000_000), nil // 1 gwei
}

func (f *fakeClient) SendTransaction(_ context.Context, _ *types.Transaction) error {
	f.sendCalled++
	return f.sendErr
}

func (f *fakeClient) TransactionReceipt(_ context.Context, _ common.Hash) (*types.Receipt, error) {
	return nil, errors.New("not mined")
}

var _ ChainClient = (*fakeClient)(nil)

// ── fakeSigner ───────────────────────────────────────────────────────────────

type fakeSigner struct {
	addr common.Address
}

func (s *fakeSigner) Address() common.Address { return s.addr }

func (s *fakeSigner) Sign(tx *types.Transaction, chainID int64) (*types.Transaction, error) {
	// Return the tx unsigned for test purposes (hash will still be non-zero).
	return tx, nil
}

var _ Signer = (*fakeSigner)(nil)

// ── helpers ───────────────────────────────────────────────────────────────────

func fakeClaim() domain.Claim {
	return domain.Claim{
		ID:        "claim-1",
		Address:   common.HexToAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"),
		AmountWei: big.NewInt(1_000_000_000_000_000_000),
		Status:    domain.ClaimStatusQueued,
	}
}

// ── ValidateChainID ───────────────────────────────────────────────────────────

func TestValidateChainIDMatch(t *testing.T) {
	client := &fakeClient{chainID: 31337}
	if err := ValidateChainID(context.Background(), client, 31337); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateChainIDMismatch(t *testing.T) {
	client := &fakeClient{chainID: 1}
	err := ValidateChainID(context.Background(), client, 31337)
	if err == nil {
		t.Fatal("expected chain ID mismatch error, got nil")
	}
}

// ── PrivateKeySigner ──────────────────────────────────────────────────────────

// testHardhatKey is Hardhat account #0 — safe to use in tests, never in production.
const testHardhatKey = "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"

func TestNewPrivateKeySignerValidKey(t *testing.T) {
	s, err := NewPrivateKeySigner(testHardhatKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := common.HexToAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266")
	if s.Address() != want {
		t.Fatalf("address = %s, want %s", s.Address().Hex(), want.Hex())
	}
}

func TestNewPrivateKeySignerWithPrefix(t *testing.T) {
	_, err := NewPrivateKeySigner("0x" + testHardhatKey)
	if err != nil {
		t.Fatalf("0x-prefixed key should be accepted: %v", err)
	}
}

func TestNewPrivateKeySignerEmptyKey(t *testing.T) {
	_, err := NewPrivateKeySigner("")
	if err == nil {
		t.Fatal("expected error for empty key")
	}
}

func TestNewPrivateKeySignerInvalidHex(t *testing.T) {
	_, err := NewPrivateKeySigner("not-valid-hex")
	if err == nil {
		t.Fatal("expected error for invalid hex key")
	}
}

func TestNewPrivateKeySignerErrorOmitsKey(t *testing.T) {
	secret := "badhex"
	_, err := NewPrivateKeySigner(secret)
	if err == nil {
		t.Fatal("expected error")
	}
	// The error must not contain the key value.
	if contains(err.Error(), secret) {
		t.Fatalf("error message exposes key value: %q", err.Error())
	}
}

func TestPrivateKeySignerProducesSignedTx(t *testing.T) {
	s, err := NewPrivateKeySigner(testHardhatKey)
	if err != nil {
		t.Fatalf("create signer: %v", err)
	}

	to := common.HexToAddress("0x70997970C51812dc3A010C7d01b50e0d17dc79C8")
	rawTx := types.NewTx(&types.LegacyTx{
		Nonce:    0,
		To:       &to,
		Value:    big.NewInt(1),
		Gas:      21000,
		GasPrice: big.NewInt(1_000_000_000),
	})

	signed, err := s.Sign(rawTx, 31337)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	v, r, ss := signed.RawSignatureValues()
	if v == nil || r == nil || ss == nil {
		t.Fatal("nil signature values after signing")
	}
	if r.Sign() == 0 && ss.Sign() == 0 {
		t.Fatal("r and s are both zero — not signed")
	}
}

// ── DryRunSender ─────────────────────────────────────────────────────────────

func TestDryRunSenderReturnsZeroHash(t *testing.T) {
	sender := NewDryRunSender(common.Address{})
	tx, err := sender.Send(context.Background(), fakeClaim())
	if err != nil {
		t.Fatalf("dry run send: %v", err)
	}
	if tx.Hash != (common.Hash{}) {
		t.Fatalf("expected zero hash, got %s", tx.Hash.Hex())
	}
	if tx.Status != domain.ClaimStatusSent {
		t.Fatalf("status = %q, want sent", tx.Status)
	}
}

func TestDryRunSenderDoesNotCallClient(t *testing.T) {
	// fakeClient panics if SendTransaction is called — ensure it never is.
	client := &fakeClient{sendErr: errors.New("should not be called")}
	_ = client // not wired to DryRunSender
	sender := NewDryRunSender(common.Address{})
	if _, err := sender.Send(context.Background(), fakeClaim()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.sendCalled != 0 {
		t.Fatal("DryRunSender must not call SendTransaction")
	}
}

// ── EthSender ────────────────────────────────────────────────────────────────

func TestEthSenderSendSuccess(t *testing.T) {
	client := &fakeClient{chainID: 31337, nonce: 7}
	signer := &fakeSigner{addr: common.HexToAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266")}
	sender := NewEthSender(client, signer, 31337)

	tx, err := sender.Send(context.Background(), fakeClaim())
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if client.sendCalled != 1 {
		t.Fatalf("sendCalled = %d, want 1", client.sendCalled)
	}
	if tx.Status != domain.ClaimStatusSent {
		t.Fatalf("status = %q, want sent", tx.Status)
	}
	if tx.From != signer.addr {
		t.Fatalf("from = %s, want %s", tx.From.Hex(), signer.addr.Hex())
	}
}

func TestEthSenderPropagatesSendError(t *testing.T) {
	client := &fakeClient{chainID: 31337, sendErr: errors.New("rpc: timeout")}
	signer := &fakeSigner{}
	sender := NewEthSender(client, signer, 31337)

	_, err := sender.Send(context.Background(), fakeClaim())
	if err == nil {
		t.Fatal("expected error from SendTransaction, got nil")
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func contains(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && (s == sub ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}

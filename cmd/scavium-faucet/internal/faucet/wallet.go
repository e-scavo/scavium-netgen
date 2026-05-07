package faucet

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

const walletChallengeTTL = 5 * time.Minute

var ErrWalletChallengeInvalid = errors.New("wallet challenge invalid")

type WalletChallenge struct {
	ID         string
	Address    common.Address
	Nonce      string
	Message    string
	ExpiresAt  time.Time
	ConsumedAt *time.Time
	CreatedAt  time.Time
}

type WalletChallengeRequest struct{ Address common.Address }

type WalletChallengeResponse struct {
	ID        string `json:"id"`
	Address   string `json:"address"`
	Nonce     string `json:"nonce"`
	Message   string `json:"message"`
	ExpiresAt string `json:"expires_at"`
	CreatedAt string `json:"created_at"`
}

type walletChallengeStore interface {
	CreateWalletChallenge(context.Context, WalletChallenge) (WalletChallenge, error)
	GetWalletChallenge(context.Context, string, common.Address, time.Time) (WalletChallenge, error)
	ConsumeWalletChallenge(context.Context, string, common.Address, time.Time) (WalletChallenge, error)
}

func newWalletChallenge(address common.Address, now time.Time) (WalletChallenge, error) {
	id, err := randomID("wch")
	if err != nil {
		return WalletChallenge{}, err
	}
	nonce, err := randomHex(16)
	if err != nil {
		return WalletChallenge{}, err
	}
	created := now.UTC()
	msg := fmt.Sprintf("SCAVIUM Faucet wallet challenge\nAddress: %s\nNonce: %s\nIssued At: %s", address.Hex(), nonce, created.Format(time.RFC3339))
	return WalletChallenge{ID: id, Address: address, Nonce: nonce, Message: msg, CreatedAt: created, ExpiresAt: created.Add(walletChallengeTTL)}, nil
}

func walletChallengeResponse(c WalletChallenge) WalletChallengeResponse {
	return WalletChallengeResponse{ID: c.ID, Address: c.Address.Hex(), Nonce: c.Nonce, Message: c.Message, ExpiresAt: c.ExpiresAt.UTC().Format(time.RFC3339), CreatedAt: c.CreatedAt.UTC().Format(time.RFC3339)}
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func verifyWalletSignature(address common.Address, message, signature string) error {
	sig := strings.TrimPrefix(strings.TrimSpace(signature), "0x")
	raw, err := hex.DecodeString(sig)
	if err != nil || len(raw) != 65 {
		return claimError(ErrClaimRejected, "invalid_wallet_signature")
	}
	if raw[64] >= 27 {
		raw[64] -= 27
	}
	if raw[64] > 1 {
		return claimError(ErrClaimRejected, "invalid_wallet_signature")
	}
	pub, err := crypto.SigToPub(accounts.TextHash([]byte(message)), raw)
	if err != nil {
		return claimError(ErrClaimRejected, "invalid_wallet_signature")
	}
	recovered := crypto.PubkeyToAddress(*pub)
	if recovered != address {
		return claimError(ErrClaimRejected, "wallet_signature_mismatch")
	}
	return nil
}

package chain

import (
	"crypto/ecdsa"
	"errors"
	"math/big"
	"strings"

	gocrypto "github.com/ethereum/go-ethereum/crypto"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// Signer abstracts transaction signing, allowing real and fake implementations.
type Signer interface {
	// Address returns the address derived from the signer's public key.
	Address() common.Address
	// Sign returns a signed copy of tx using the EIP-155 scheme with the given chain ID.
	Sign(tx *types.Transaction, chainID int64) (*types.Transaction, error)
}

// PrivateKeySigner signs transactions with an ECDSA private key loaded from a hex string.
// The key value is never exposed through public APIs or error messages.
type PrivateKeySigner struct {
	key     *ecdsa.PrivateKey
	address common.Address
}

// NewPrivateKeySigner loads an ECDSA private key from a hex string (with or without 0x prefix).
// The key value is not included in any returned error.
func NewPrivateKeySigner(hexKey string) (*PrivateKeySigner, error) {
	hexKey = strings.TrimPrefix(hexKey, "0x")
	if hexKey == "" {
		return nil, errors.New("private key is empty")
	}
	key, err := gocrypto.HexToECDSA(hexKey)
	if err != nil {
		return nil, errors.New("invalid private key: bad hex or wrong length")
	}
	addr := gocrypto.PubkeyToAddress(key.PublicKey)
	return &PrivateKeySigner{key: key, address: addr}, nil
}

// Address returns the Ethereum address derived from the private key.
func (s *PrivateKeySigner) Address() common.Address {
	return s.address
}

// Sign signs the transaction using EIP-155 with the given chain ID.
func (s *PrivateKeySigner) Sign(tx *types.Transaction, chainID int64) (*types.Transaction, error) {
	ethSigner := types.NewEIP155Signer(big.NewInt(chainID))
	return types.SignTx(tx, ethSigner, s.key)
}

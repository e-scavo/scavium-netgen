package ethutil

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

func Dial(rpcURL string) (*ethclient.Client, error) {
	return ethclient.Dial(rpcURL)
}

func HexToECDSA(hexKey string) (*ecdsa.PrivateKey, error) {
	hexKey = strings.TrimSpace(strings.TrimPrefix(hexKey, "0x"))
	return crypto.HexToECDSA(hexKey)
}

func AddressFromPrivateKey(pk *ecdsa.PrivateKey) common.Address {
	return crypto.PubkeyToAddress(pk.PublicKey)
}

func WaitReceipt(client *ethclient.Client, txHash common.Hash, timeout time.Duration) (*types.Receipt, error) {
	deadline := time.Now().Add(timeout)
	ctx := context.Background()

	for time.Now().Before(deadline) {
		receipt, err := client.TransactionReceipt(ctx, txHash)
		if err == nil && receipt != nil {
			return receipt, nil
		}
		time.Sleep(2 * time.Second)
	}
	return nil, fmt.Errorf("timeout waiting for receipt: %s", txHash.Hex())
}

func ParseAmountWei(s string) (*big.Int, error) {
	n := new(big.Int)
	_, ok := n.SetString(strings.TrimSpace(s), 10)
	if !ok {
		return nil, fmt.Errorf("invalid wei amount: %s", s)
	}
	return n, nil
}

func MustAddress(hexAddr string) common.Address {
	return common.HexToAddress(strings.TrimSpace(hexAddr))
}

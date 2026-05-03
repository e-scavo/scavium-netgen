// Package chain provides blockchain client abstractions and implementations.
// All interactions with go-ethereum are isolated here behind testable interfaces.
package chain

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// ChainClient abstracts the subset of the Ethereum JSON-RPC API used by this package.
// Implementations must be safe for concurrent use.
type ChainClient interface {
	// ChainID returns the chain ID of the connected network.
	ChainID(ctx context.Context) (*big.Int, error)
	// BalanceAt returns the wei balance of addr at the given block (nil = latest).
	BalanceAt(ctx context.Context, addr common.Address, blockNumber *big.Int) (*big.Int, error)
	// NonceAt returns the pending nonce for addr.
	NonceAt(ctx context.Context, addr common.Address, blockNumber *big.Int) (uint64, error)
	// SuggestGasPrice returns the recommended legacy gas price.
	SuggestGasPrice(ctx context.Context) (*big.Int, error)
	// SendTransaction broadcasts a signed transaction to the network.
	SendTransaction(ctx context.Context, tx *types.Transaction) error
	// TransactionReceipt fetches the receipt for txHash (returns error if not yet mined).
	TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error)
}

// Client wraps ethclient.Client and implements ChainClient.
type Client struct {
	ec *ethclient.Client
}

var _ ChainClient = (*Client)(nil)

// NewClient dials an Ethereum JSON-RPC endpoint and returns a ready-to-use Client.
func NewClient(ctx context.Context, rpcURL string) (*Client, error) {
	ec, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", rpcURL, err)
	}
	return &Client{ec: ec}, nil
}

// Close releases the underlying connection.
func (c *Client) Close() { c.ec.Close() }

func (c *Client) ChainID(ctx context.Context) (*big.Int, error) {
	return c.ec.ChainID(ctx)
}

func (c *Client) BalanceAt(ctx context.Context, addr common.Address, blockNumber *big.Int) (*big.Int, error) {
	return c.ec.BalanceAt(ctx, addr, blockNumber)
}

func (c *Client) NonceAt(ctx context.Context, addr common.Address, blockNumber *big.Int) (uint64, error) {
	return c.ec.NonceAt(ctx, addr, blockNumber)
}

func (c *Client) SuggestGasPrice(ctx context.Context) (*big.Int, error) {
	return c.ec.SuggestGasPrice(ctx)
}

func (c *Client) SendTransaction(ctx context.Context, tx *types.Transaction) error {
	return c.ec.SendTransaction(ctx, tx)
}

func (c *Client) TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error) {
	return c.ec.TransactionReceipt(ctx, txHash)
}

// ValidateChainID checks that the node's chain ID matches the expected value.
// This should be called once at startup before processing any transactions.
func ValidateChainID(ctx context.Context, client ChainClient, expected int64) error {
	got, err := client.ChainID(ctx)
	if err != nil {
		return fmt.Errorf("get chain id from node: %w", err)
	}
	if got.Int64() != expected {
		return fmt.Errorf("chain ID mismatch: node reports %d, configured %d", got.Int64(), expected)
	}
	return nil
}

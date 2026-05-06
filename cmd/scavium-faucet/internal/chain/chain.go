// Package chain provides blockchain client abstractions and implementations.
// All interactions with go-ethereum are isolated here behind testable interfaces.
package chain

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
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
	// BlockNumber returns the number of the most recently mined block.
	BlockNumber(ctx context.Context) (uint64, error)
}

// ClosableChainClient is a chain client with an owned connection lifecycle.
type ClosableChainClient interface {
	ChainClient
	Close()
}

// ContractCaller abstracts read-only eth_call access for optional ERC20 balance visibility.
type ContractCaller interface {
	CallContract(ctx context.Context, msg ethereum.CallMsg, blockNumber *big.Int) ([]byte, error)
}

// Client wraps ethclient.Client and implements ChainClient.
type Client struct {
	ec *ethclient.Client
}

var _ ChainClient = (*Client)(nil)
var _ ContractCaller = (*Client)(nil)

// NewClient dials an Ethereum JSON-RPC endpoint and returns a ready-to-use Client.
func NewClient(ctx context.Context, rpcURL string) (*Client, error) {
	ec, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", rpcURL, err)
	}
	return &Client{ec: ec}, nil
}

// RPCSelection describes the validated RPC endpoint selected at startup.
type RPCSelection struct {
	Client ClosableChainClient
	URL    string
	Index  int
}

type ClientFactory func(context.Context, string) (ClosableChainClient, error)

// NewValidatedClient dials the primary RPC URL and, only if necessary, tries
// configured secondaries in order. Every candidate is chain-ID validated before
// being returned. There is no load balancing and no per-transaction endpoint
// rotation, so transaction semantics remain identical after startup selection.
func NewValidatedClient(ctx context.Context, primaryURL string, secondaryURLs []string, expectedChainID int64) (RPCSelection, error) {
	return newValidatedClient(ctx, primaryURL, secondaryURLs, expectedChainID, func(ctx context.Context, rpcURL string) (ClosableChainClient, error) { return NewClient(ctx, rpcURL) })
}

func newValidatedClient(ctx context.Context, primaryURL string, secondaryURLs []string, expectedChainID int64, factory ClientFactory) (RPCSelection, error) {
	urls := CandidateRPCURLs(primaryURL, secondaryURLs)
	if len(urls) == 0 {
		return RPCSelection{}, errors.New("at least one RPC URL is required")
	}
	var errs []error
	for i, rpcURL := range urls {
		client, err := factory(ctx, rpcURL)
		if err != nil {
			errs = append(errs, fmt.Errorf("candidate %d %q: %w", i, rpcURL, err))
			continue
		}
		if err := ValidateChainID(ctx, client, expectedChainID); err != nil {
			client.Close()
			errs = append(errs, fmt.Errorf("candidate %d %q: %w", i, rpcURL, err))
			continue
		}
		return RPCSelection{Client: client, URL: rpcURL, Index: i}, nil
	}
	return RPCSelection{}, errors.Join(errs...)
}

// CandidateRPCURLs returns the primary URL followed by de-duplicated secondary
// URLs. Empty entries are ignored to keep env parsing forgiving.
func CandidateRPCURLs(primaryURL string, secondaryURLs []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, 1+len(secondaryURLs))
	add := func(raw string) {
		u := strings.TrimSpace(raw)
		if u == "" {
			return
		}
		if _, exists := seen[u]; exists {
			return
		}
		seen[u] = struct{}{}
		out = append(out, u)
	}
	add(primaryURL)
	for _, u := range secondaryURLs {
		add(u)
	}
	return out
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

func (c *Client) BlockNumber(ctx context.Context) (uint64, error) {
	return c.ec.BlockNumber(ctx)
}

func (c *Client) CallContract(ctx context.Context, msg ethereum.CallMsg, blockNumber *big.Int) ([]byte, error) {
	return c.ec.CallContract(ctx, msg, blockNumber)
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

// ERC20BalanceOf performs a safe read-only balanceOf(address) call. It returns
// the raw token unit balance and does not attempt formatting or decimals logic.
func ERC20BalanceOf(ctx context.Context, caller ContractCaller, token common.Address, holder common.Address) (*big.Int, error) {
	if caller == nil {
		return nil, errors.New("contract caller is not configured")
	}
	selector, _ := hex.DecodeString("70a08231")
	data := make([]byte, 4+32)
	copy(data[:4], selector)
	copy(data[4+12:], holder.Bytes())
	out, err := caller.CallContract(ctx, ethereum.CallMsg{To: &token, Data: data}, nil)
	if err != nil {
		return nil, fmt.Errorf("erc20 balanceOf call failed: %w", err)
	}
	if len(out) < 32 {
		return nil, fmt.Errorf("erc20 balanceOf returned %d bytes, want at least 32", len(out))
	}
	return new(big.Int).SetBytes(out[len(out)-32:]), nil
}

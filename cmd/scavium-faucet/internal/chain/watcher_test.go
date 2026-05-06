package chain

import (
	"context"
	"math/big"
	"sync"
	"testing"
	"time"

	"scavium-netgen/cmd/scavium-faucet/internal/domain"
	"scavium-netgen/cmd/scavium-faucet/internal/observability"
	"scavium-netgen/cmd/scavium-faucet/internal/version"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// ── fakeWatcherStore ─────────────────────────────────────────────────────────

type fakeWatcherStore struct {
	mu        sync.Mutex
	pending   []domain.PendingTx
	stuck     []domain.Claim
	confirmed []confirmRecord
	failed    []failedRecord
}

type confirmRecord struct {
	claimID     string
	blockNumber uint64
	gasUsed     uint64
}

type failedRecord struct {
	claimID string
	reason  string
}

func (s *fakeWatcherStore) ListPendingTransactions(_ context.Context, limit int) ([]domain.PendingTx, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit > len(s.pending) {
		limit = len(s.pending)
	}
	out := make([]domain.PendingTx, limit)
	copy(out, s.pending[:limit])
	return out, nil
}

func (s *fakeWatcherStore) ConfirmTransaction(_ context.Context, claimID string, blockNumber, gasUsed uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.confirmed = append(s.confirmed, confirmRecord{claimID, blockNumber, gasUsed})
	return nil
}

func (s *fakeWatcherStore) FailTransaction(_ context.Context, claimID string, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failed = append(s.failed, failedRecord{claimID, reason})
	return nil
}

func (s *fakeWatcherStore) ListStuckSending(_ context.Context, _ time.Duration, limit int) ([]domain.Claim, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit > len(s.stuck) {
		limit = len(s.stuck)
	}
	out := make([]domain.Claim, limit)
	copy(out, s.stuck[:limit])
	return out, nil
}

var _ domain.WatcherStore = (*fakeWatcherStore)(nil)

// ── fakeChainForWatcher ───────────────────────────────────────────────────────

type fakeChainForWatcher struct {
	blockNum uint64
	receipts map[common.Hash]*types.Receipt // nil entry = not mined
}

func (f *fakeChainForWatcher) ChainID(_ context.Context) (*big.Int, error) {
	return big.NewInt(31337), nil
}

func (f *fakeChainForWatcher) BalanceAt(_ context.Context, _ common.Address, _ *big.Int) (*big.Int, error) {
	return big.NewInt(0), nil
}

func (f *fakeChainForWatcher) NonceAt(_ context.Context, _ common.Address, _ *big.Int) (uint64, error) {
	return 0, nil
}

func (f *fakeChainForWatcher) SuggestGasPrice(_ context.Context) (*big.Int, error) {
	return big.NewInt(1_000_000_000), nil
}

func (f *fakeChainForWatcher) SendTransaction(_ context.Context, _ *types.Transaction) error {
	return nil
}

func (f *fakeChainForWatcher) TransactionReceipt(_ context.Context, txHash common.Hash) (*types.Receipt, error) {
	if f.receipts == nil {
		return nil, errNotMined
	}
	r, ok := f.receipts[txHash]
	if !ok || r == nil {
		return nil, errNotMined
	}
	return r, nil
}

func (f *fakeChainForWatcher) BlockNumber(_ context.Context) (uint64, error) {
	return f.blockNum, nil
}

var _ ChainClient = (*fakeChainForWatcher)(nil)

var errNotMined = errorf("not mined")

type stringError string

func (e stringError) Error() string { return string(e) }

func errorf(s string) error { return stringError(s) }

// successReceipt returns a receipt with Status=1 at the given block number.
func successReceipt(blockNum uint64, gasUsed uint64) *types.Receipt {
	return &types.Receipt{
		Status:      types.ReceiptStatusSuccessful,
		BlockNumber: new(big.Int).SetUint64(blockNum),
		GasUsed:     gasUsed,
	}
}

// failedReceipt returns a receipt with Status=0 (reverted) at the given block number.
func revertedReceipt(blockNum uint64) *types.Receipt {
	return &types.Receipt{
		Status:      types.ReceiptStatusFailed,
		BlockNumber: new(big.Int).SetUint64(blockNum),
	}
}

// ── Watcher tests ─────────────────────────────────────────────────────────────

func TestWatcherConfirmsReceiptAfterMinConfirmations(t *testing.T) {
	txHash := common.HexToHash("0xaaaa")
	store := &fakeWatcherStore{
		pending: []domain.PendingTx{{ClaimID: "c1", TxHash: txHash}},
	}
	chain := &fakeChainForWatcher{
		blockNum: 101, // tx at 100, current at 101 → 1 confirmation
		receipts: map[common.Hash]*types.Receipt{txHash: successReceipt(100, 21000)},
	}
	cfg := WatcherConfig{PollInterval: time.Minute, MinConfirmations: 1, BatchSize: 10}
	w := NewWatcher(store, chain, cfg, nil)

	w.watchReceipts(context.Background())

	if len(store.confirmed) != 1 {
		t.Fatalf("confirmed = %d, want 1", len(store.confirmed))
	}
	if store.confirmed[0].claimID != "c1" {
		t.Fatalf("claimID = %q, want c1", store.confirmed[0].claimID)
	}
	if store.confirmed[0].blockNumber != 100 {
		t.Fatalf("blockNumber = %d, want 100", store.confirmed[0].blockNumber)
	}
	if store.confirmed[0].gasUsed != 21000 {
		t.Fatalf("gasUsed = %d, want 21000", store.confirmed[0].gasUsed)
	}
}

func TestWatcherSkipsTxNotYetMined(t *testing.T) {
	txHash := common.HexToHash("0xbbbb")
	store := &fakeWatcherStore{
		pending: []domain.PendingTx{{ClaimID: "c2", TxHash: txHash}},
	}
	chain := &fakeChainForWatcher{
		blockNum: 100,
		receipts: map[common.Hash]*types.Receipt{txHash: nil}, // nil = not mined
	}
	cfg := WatcherConfig{PollInterval: time.Minute, MinConfirmations: 1, BatchSize: 10}
	w := NewWatcher(store, chain, cfg, nil)

	w.watchReceipts(context.Background())

	if len(store.confirmed) != 0 || len(store.failed) != 0 {
		t.Fatalf("expected no confirmations or failures, got confirmed=%d failed=%d",
			len(store.confirmed), len(store.failed))
	}
}

func TestWatcherSkipsTxBelowMinConfirmations(t *testing.T) {
	txHash := common.HexToHash("0xcccc")
	store := &fakeWatcherStore{
		pending: []domain.PendingTx{{ClaimID: "c3", TxHash: txHash}},
	}
	chain := &fakeChainForWatcher{
		blockNum: 100, // tx at 100, current also 100 → 0 confirmations, need 1
		receipts: map[common.Hash]*types.Receipt{txHash: successReceipt(100, 21000)},
	}
	cfg := WatcherConfig{PollInterval: time.Minute, MinConfirmations: 1, BatchSize: 10}
	w := NewWatcher(store, chain, cfg, nil)

	w.watchReceipts(context.Background())

	if len(store.confirmed) != 0 {
		t.Fatalf("expected no confirmations yet, got %d", len(store.confirmed))
	}
}

func TestWatcherFailsRevertedTransaction(t *testing.T) {
	txHash := common.HexToHash("0xdddd")
	store := &fakeWatcherStore{
		pending: []domain.PendingTx{{ClaimID: "c4", TxHash: txHash}},
	}
	chain := &fakeChainForWatcher{
		blockNum: 200,
		receipts: map[common.Hash]*types.Receipt{txHash: revertedReceipt(199)},
	}
	cfg := WatcherConfig{PollInterval: time.Minute, MinConfirmations: 1, BatchSize: 10}
	w := NewWatcher(store, chain, cfg, nil)

	w.watchReceipts(context.Background())

	if len(store.failed) != 1 {
		t.Fatalf("failed = %d, want 1", len(store.failed))
	}
	if store.failed[0].claimID != "c4" {
		t.Fatalf("claimID = %q, want c4", store.failed[0].claimID)
	}
	if len(store.confirmed) != 0 {
		t.Fatal("a reverted tx must not be confirmed")
	}
}

func TestWatcherReconcileStuckClaims(t *testing.T) {
	store := &fakeWatcherStore{
		stuck: []domain.Claim{
			{ID: "stuck1", Status: domain.ClaimStatusSending},
			{ID: "stuck2", Status: domain.ClaimStatusSending},
		},
	}
	chain := &fakeChainForWatcher{blockNum: 1}
	cfg := WatcherConfig{PollInterval: time.Minute, StuckTimeout: time.Minute, BatchSize: 10}
	w := NewWatcher(store, chain, cfg, nil)

	w.reconcileStuck(context.Background())

	if len(store.failed) != 2 {
		t.Fatalf("reconciled = %d, want 2", len(store.failed))
	}
}

func TestWatcherRunStopsOnContextCancel(t *testing.T) {
	store := &fakeWatcherStore{}
	chain := &fakeChainForWatcher{blockNum: 1}
	cfg := WatcherConfig{PollInterval: time.Hour, MinConfirmations: 1, BatchSize: 10}
	w := NewWatcher(store, chain, cfg, nil)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop after context cancel")
	}
}

func TestWatcherEmptyPendingDoesNothing(t *testing.T) {
	store := &fakeWatcherStore{}
	chain := &fakeChainForWatcher{blockNum: 50}
	cfg := WatcherConfig{PollInterval: time.Minute, MinConfirmations: 1, BatchSize: 10}
	w := NewWatcher(store, chain, cfg, nil)

	w.watchReceipts(context.Background())
	w.reconcileStuck(context.Background())

	if len(store.confirmed)+len(store.failed) != 0 {
		t.Fatal("expected no side-effects on empty store")
	}
}

func TestWatcherMetrics(t *testing.T) {
	txHash := common.HexToHash("0xeeee")
	store := &fakeWatcherStore{
		pending: []domain.PendingTx{{ClaimID: "c1", TxHash: txHash}},
		stuck:   []domain.Claim{{ID: "stuck1", Status: domain.ClaimStatusSending}},
	}
	chain := &fakeChainForWatcher{
		blockNum: 101,
		receipts: map[common.Hash]*types.Receipt{txHash: successReceipt(100, 21000)},
	}
	metrics := observability.NewRuntimeMetrics(version.Info{})
	w := NewWatcherWithMetrics(store, chain, WatcherConfig{PollInterval: time.Minute, MinConfirmations: 1, StuckTimeout: time.Minute, BatchSize: 10}, nil, metrics)

	w.watchReceipts(context.Background())
	w.reconcileStuck(context.Background())

	snapshot := metrics.Snapshot(time.Now())
	if snapshot.Watcher.PendingListed != 1 || snapshot.Watcher.Confirmed != 1 || snapshot.Watcher.StuckFound != 1 {
		t.Fatalf("watcher metrics = %#v", snapshot.Watcher)
	}
}

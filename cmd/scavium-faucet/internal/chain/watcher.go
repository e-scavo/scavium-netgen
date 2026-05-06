package chain

import (
	"context"
	"log/slog"
	"time"

	"scavium-netgen/cmd/scavium-faucet/internal/domain"
	"scavium-netgen/cmd/scavium-faucet/internal/observability"
)

// WatcherConfig holds tunable parameters for the Watcher.
type WatcherConfig struct {
	// PollInterval controls how often the watcher looks for pending transactions.
	PollInterval time.Duration
	// MinConfirmations is the number of blocks that must follow the tx block before
	// a transaction is considered confirmed (0 = confirm as soon as receipt appears).
	MinConfirmations uint64
	// StuckTimeout is the duration after which a claim in 'sending' state is
	// considered stuck and eligible for reconciliation back to 'queued'.
	StuckTimeout time.Duration
	// BatchSize is the maximum number of pending transactions processed per cycle.
	BatchSize int
}

// DefaultWatcherConfig returns a WatcherConfig with conservative production defaults.
func DefaultWatcherConfig() WatcherConfig {
	return WatcherConfig{
		PollInterval:     15 * time.Second,
		MinConfirmations: 1,
		StuckTimeout:     2 * time.Minute,
		BatchSize:        50,
	}
}

// Watcher polls for pending and stuck claims and drives them to terminal states.
// Call Run once; it blocks until ctx is cancelled.
type Watcher struct {
	client  domain.WatcherStore
	chain   ChainClient
	cfg     WatcherConfig
	log     *slog.Logger
	metrics *observability.RuntimeMetrics
}

// NewWatcher creates a Watcher.  If log is nil, slog.Default() is used.
func NewWatcher(store domain.WatcherStore, chain ChainClient, cfg WatcherConfig, log *slog.Logger) *Watcher {
	return NewWatcherWithMetrics(store, chain, cfg, log, nil)
}

// NewWatcherWithMetrics creates a Watcher with optional runtime metrics instrumentation.
func NewWatcherWithMetrics(store domain.WatcherStore, chain ChainClient, cfg WatcherConfig, log *slog.Logger, metrics *observability.RuntimeMetrics) *Watcher {
	if log == nil {
		log = slog.Default()
	}
	return &Watcher{client: store, chain: chain, cfg: cfg, log: log, metrics: metrics}
}

// Run blocks until ctx is cancelled.  It calls watchReceipts and reconcileStuck
// on every PollInterval tick and returns nil on clean shutdown.
func (w *Watcher) Run(ctx context.Context) error {
	ticker := time.NewTicker(w.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			w.watchReceipts(ctx)
			w.reconcileStuck(ctx)
		}
	}
}

// watchReceipts fetches receipts for pending transactions and confirms or fails them.
func (w *Watcher) watchReceipts(ctx context.Context) {
	pending, err := w.client.ListPendingTransactions(ctx, w.cfg.BatchSize)
	if err != nil {
		if w.metrics != nil {
			w.metrics.IncWatcherRPCFailed()
		}
		w.log.Error("watcher: list pending transactions", "error", err)
		return
	}
	if w.metrics != nil {
		w.metrics.IncWatcherPendingListed(len(pending))
	}
	if len(pending) == 0 {
		return
	}

	currentBlock, err := w.chain.BlockNumber(ctx)
	if err != nil {
		if w.metrics != nil {
			w.metrics.IncWatcherRPCFailed()
		}
		w.log.Error("watcher: get block number", "error", err)
		return
	}

	for _, pt := range pending {
		receipt, err := w.chain.TransactionReceipt(ctx, pt.TxHash)
		if err != nil {
			// Not yet mined — skip silently.
			continue
		}

		if receipt.Status == 0 {
			if w.metrics != nil {
				w.metrics.IncWatcherReverted()
			}
			// Transaction reverted on-chain.
			if err := w.client.FailTransaction(ctx, pt.ClaimID, "transaction reverted on-chain"); err != nil {
				w.log.Error("watcher: fail reverted tx", "claim_id", pt.ClaimID,
					"tx_hash", pt.TxHash.Hex(), "error", err)
			} else {
				w.log.Info("watcher: transaction reverted", "claim_id", pt.ClaimID,
					"tx_hash", pt.TxHash.Hex())
			}
			continue
		}

		// Receipt present and successful — check confirmations.
		txBlock := receipt.BlockNumber.Uint64()
		if currentBlock < txBlock+w.cfg.MinConfirmations {
			// Not enough confirmations yet.
			continue
		}

		if err := w.client.ConfirmTransaction(ctx, pt.ClaimID, txBlock, receipt.GasUsed); err != nil {
			w.log.Error("watcher: confirm transaction", "claim_id", pt.ClaimID,
				"tx_hash", pt.TxHash.Hex(), "error", err)
		} else {
			if w.metrics != nil {
				w.metrics.IncWatcherConfirmed()
			}
			w.log.Info("watcher: transaction confirmed", "claim_id", pt.ClaimID,
				"tx_hash", pt.TxHash.Hex(), "block", txBlock)
		}
	}
}

// reconcileStuck moves claims stuck in 'sending' back to 'queued' so the
// worker can retry them.
func (w *Watcher) reconcileStuck(ctx context.Context) {
	stuck, err := w.client.ListStuckSending(ctx, w.cfg.StuckTimeout, w.cfg.BatchSize)
	if err != nil {
		if w.metrics != nil {
			w.metrics.IncWatcherRPCFailed()
		}
		w.log.Error("watcher: list stuck sending", "error", err)
		return
	}
	if w.metrics != nil {
		w.metrics.IncWatcherStuckFound(len(stuck))
	}

	for _, claim := range stuck {
		// Re-queue with a sentinel tx hash so the worker picks it up.
		if err := w.client.FailTransaction(ctx, claim.ID, "reconciled: stuck in sending"); err != nil {
			if w.metrics != nil {
				w.metrics.IncWatcherStuckFailed()
			}
			w.log.Error("watcher: reconcile stuck claim", "claim_id", claim.ID, "error", err)
		} else {
			w.log.Warn("watcher: reconciled stuck claim", "claim_id", claim.ID)
		}
	}
}

// Package worker processes queued faucet claims and submits payouts.
package worker

import (
	"context"
	"log/slog"
	"time"

	"scavium-netgen/cmd/scavium-faucet/internal/domain"
)

const (
	defaultBatchSize    = 10
	defaultMaxRetries   = 3
	defaultPollInterval = 5 * time.Second
)

// Config holds tunable parameters for the Worker.
type Config struct {
	// BatchSize is the maximum number of claims processed per poll cycle.
	BatchSize int
	// MaxRetries is the number of send attempts before a claim is dead-lettered.
	MaxRetries int
	// PollInterval controls how often the worker wakes up to look for work.
	PollInterval time.Duration
}

// DefaultConfig returns a Config with conservative production defaults.
func DefaultConfig() Config {
	return Config{
		BatchSize:    defaultBatchSize,
		MaxRetries:   defaultMaxRetries,
		PollInterval: defaultPollInterval,
	}
}

// Worker polls a QueueStore for queued claims, attempts to send them via
// Sender, and acknowledges or retries/dead-letters based on the result.
// It shuts down cleanly when the supplied context is cancelled.
type Worker struct {
	queue  domain.QueueStore
	sender domain.Sender
	cfg    Config
	log    *slog.Logger
}

// New creates a Worker.  If log is nil, slog.Default() is used.
func New(queue domain.QueueStore, sender domain.Sender, cfg Config, log *slog.Logger) *Worker {
	if log == nil {
		log = slog.Default()
	}
	return &Worker{queue: queue, sender: sender, cfg: cfg, log: log}
}

// Run blocks until ctx is cancelled.  It processes one batch per PollInterval
// tick and returns nil on clean shutdown.
func (w *Worker) Run(ctx context.Context) error {
	ticker := time.NewTicker(w.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := w.processBatch(ctx); err != nil {
				w.log.Error("worker: process batch", "error", err)
			}
		}
	}
}

// processBatch dequeues up to BatchSize claims and processes each one.
func (w *Worker) processBatch(ctx context.Context) error {
	claims, err := w.queue.DequeueBatch(ctx, w.cfg.BatchSize)
	if err != nil {
		return err
	}

	for _, claim := range claims {
		tx, sendErr := w.sender.Send(ctx, claim)
		if sendErr != nil {
			w.log.Warn("worker: send failed", "claim_id", claim.ID, "error", sendErr)
			if failErr := w.queue.Fail(ctx, claim.ID, sendErr.Error(), w.cfg.MaxRetries); failErr != nil {
				w.log.Error("worker: fail claim", "claim_id", claim.ID, "error", failErr)
			}
			continue
		}
		if ackErr := w.queue.Ack(ctx, claim.ID, tx); ackErr != nil {
			w.log.Error("worker: ack claim", "claim_id", claim.ID, "error", ackErr)
		}
	}
	return nil
}

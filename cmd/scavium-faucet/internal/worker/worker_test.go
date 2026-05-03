package worker

import (
	"context"
	"errors"
	"math/big"
	"sync"
	"testing"
	"time"

	"scavium-netgen/cmd/scavium-faucet/internal/domain"

	"github.com/ethereum/go-ethereum/common"
)

// ── fakes ────────────────────────────────────────────────────────────────────

type fakeQueue struct {
	mu     sync.Mutex
	claims []domain.Claim
	acked  []string
	failed []failRecord
}

type failRecord struct {
	claimID    string
	reason     string
	maxRetries int
}

func (q *fakeQueue) Enqueue(_ context.Context, _ string) error { return nil }

func (q *fakeQueue) DequeueBatch(_ context.Context, n int) ([]domain.Claim, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.claims) == 0 {
		return nil, nil
	}
	take := n
	if take > len(q.claims) {
		take = len(q.claims)
	}
	batch := make([]domain.Claim, take)
	copy(batch, q.claims[:take])
	q.claims = q.claims[take:]
	return batch, nil
}

func (q *fakeQueue) Ack(_ context.Context, claimID string, _ domain.Transaction) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.acked = append(q.acked, claimID)
	return nil
}

func (q *fakeQueue) Fail(_ context.Context, claimID string, reason string, maxRetries int) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.failed = append(q.failed, failRecord{claimID, reason, maxRetries})
	return nil
}

type fakeSender struct {
	mu    sync.Mutex
	calls int
	err   error // if non-nil, every Send returns this error
}

func (s *fakeSender) Send(_ context.Context, _ domain.Claim) (domain.Transaction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if s.err != nil {
		return domain.Transaction{}, s.err
	}
	return domain.Transaction{}, nil
}

func fakeClaim(id string) domain.Claim {
	return domain.Claim{
		ID:        id,
		Address:   common.HexToAddress("0x1234567890123456789012345678901234567890"),
		AmountWei: big.NewInt(1e18),
		Status:    domain.ClaimStatusQueued,
	}
}

// ── tests ────────────────────────────────────────────────────────────────────

func TestWorkerProcessesBatch(t *testing.T) {
	q := &fakeQueue{
		claims: []domain.Claim{fakeClaim("c1"), fakeClaim("c2"), fakeClaim("c3")},
	}
	s := &fakeSender{}
	w := New(q, s, DefaultConfig(), nil)

	if err := w.processBatch(context.Background()); err != nil {
		t.Fatalf("processBatch: %v", err)
	}

	if s.calls != 3 {
		t.Fatalf("sender calls = %d, want 3", s.calls)
	}
	if len(q.acked) != 3 {
		t.Fatalf("acked = %d, want 3", len(q.acked))
	}
	if len(q.failed) != 0 {
		t.Fatalf("failed = %d, want 0", len(q.failed))
	}
}

func TestWorkerRetryOnSendFailure(t *testing.T) {
	q := &fakeQueue{
		claims: []domain.Claim{fakeClaim("c1"), fakeClaim("c2")},
	}
	s := &fakeSender{err: errors.New("rpc error")}
	w := New(q, s, DefaultConfig(), nil)

	if err := w.processBatch(context.Background()); err != nil {
		t.Fatalf("processBatch: %v", err)
	}

	if s.calls != 2 {
		t.Fatalf("sender calls = %d, want 2", s.calls)
	}
	if len(q.acked) != 0 {
		t.Fatalf("acked = %d, want 0", len(q.acked))
	}
	if len(q.failed) != 2 {
		t.Fatalf("failed = %d, want 2", len(q.failed))
	}
	for _, f := range q.failed {
		if f.maxRetries != defaultMaxRetries {
			t.Fatalf("maxRetries = %d, want %d", f.maxRetries, defaultMaxRetries)
		}
	}
}

func TestWorkerEmptyBatch(t *testing.T) {
	q := &fakeQueue{}
	s := &fakeSender{}
	w := New(q, s, DefaultConfig(), nil)

	if err := w.processBatch(context.Background()); err != nil {
		t.Fatalf("processBatch: %v", err)
	}

	if s.calls != 0 {
		t.Fatalf("sender calls = %d, want 0", s.calls)
	}
}

func TestWorkerBatchSizeRespected(t *testing.T) {
	q := &fakeQueue{
		claims: []domain.Claim{
			fakeClaim("c1"), fakeClaim("c2"), fakeClaim("c3"), fakeClaim("c4"), fakeClaim("c5"),
		},
	}
	s := &fakeSender{}
	cfg := Config{BatchSize: 2, MaxRetries: 3, PollInterval: time.Second}
	w := New(q, s, cfg, nil)

	if err := w.processBatch(context.Background()); err != nil {
		t.Fatalf("first batch: %v", err)
	}
	if s.calls != 2 {
		t.Fatalf("after 1st batch: sender calls = %d, want 2", s.calls)
	}

	if err := w.processBatch(context.Background()); err != nil {
		t.Fatalf("second batch: %v", err)
	}
	if s.calls != 4 {
		t.Fatalf("after 2nd batch: sender calls = %d, want 4", s.calls)
	}
}

func TestWorkerRunExitsOnContextCancel(t *testing.T) {
	q := &fakeQueue{}
	s := &fakeSender{}
	cfg := Config{BatchSize: 1, MaxRetries: 3, PollInterval: 10 * time.Millisecond}
	w := New(q, s, cfg, nil)

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
		t.Fatal("Run did not exit after context cancel")
	}
}

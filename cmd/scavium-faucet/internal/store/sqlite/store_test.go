package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"math/big"
	"path/filepath"
	"testing"
	"time"

	"scavium-netgen/cmd/scavium-faucet/internal/domain"

	"github.com/ethereum/go-ethereum/common"
)

func TestMigrateCreatesRequiredTables(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	for _, table := range []string{"requests", "transactions", "rate_limits", "config", "abuse_signals", "schema_migrations"} {
		if !tableExists(t, store.db, table) {
			t.Fatalf("table %s does not exist", table)
		}
	}
}

func TestMigrateCreatesIndexes(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	for _, index := range []string{"idx_requests_address", "idx_requests_status", "idx_requests_created_at", "idx_abuse_signals_kind", "idx_abuse_signals_remote_ip"} {
		if !indexExists(t, store.db, index) {
			t.Fatalf("index %s does not exist", index)
		}
	}
}

func TestCreateAndGetClaim(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	claim := testClaim("claim_1")
	created, err := store.CreateClaim(context.Background(), claim)
	if err != nil {
		t.Fatalf("create claim: %v", err)
	}
	if created.ID != claim.ID {
		t.Fatalf("created id = %q", created.ID)
	}

	got, err := store.GetClaim(context.Background(), claim.ID)
	if err != nil {
		t.Fatalf("get claim: %v", err)
	}
	if got.ID != claim.ID {
		t.Fatalf("got id = %q", got.ID)
	}
	if got.Address != claim.Address {
		t.Fatalf("got address = %s", got.Address.Hex())
	}
	if got.AmountWei.Cmp(claim.AmountWei) != 0 {
		t.Fatalf("got amount = %s", got.AmountWei.String())
	}
	if got.Status != domain.ClaimStatusQueued {
		t.Fatalf("got status = %q", got.Status)
	}
}

func TestCreateClaimWithIdempotencyReturnsExistingClaim(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	first, err := store.CreateClaimWithIdempotency(context.Background(), testClaim("claim_1"), "idem-key")
	if err != nil {
		t.Fatalf("create first claim: %v", err)
	}

	second, err := store.CreateClaimWithIdempotency(context.Background(), testClaim("claim_2"), "idem-key")
	if err != nil {
		t.Fatalf("create duplicate idempotency key: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("id = %q, want existing id %q", second.ID, first.ID)
	}
}

func TestUpdateClaimStatus(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	claim := testClaim("claim_1")
	_, err := store.CreateClaim(context.Background(), claim)
	if err != nil {
		t.Fatalf("create claim: %v", err)
	}

	updated, err := store.UpdateClaimStatus(context.Background(), claim.ID, domain.ClaimStatusRejected, "invalid request")
	if err != nil {
		t.Fatalf("update claim: %v", err)
	}
	if updated.Status != domain.ClaimStatusRejected {
		t.Fatalf("status = %q", updated.Status)
	}
	if updated.Reason != "invalid request" {
		t.Fatalf("reason = %q", updated.Reason)
	}
}

func TestListAndLastClaimsByAddress(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	first := testClaim("claim_1")
	first.CreatedAt = time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	first.UpdatedAt = first.CreatedAt
	second := testClaim("claim_2")
	second.CreatedAt = time.Date(2026, 5, 3, 11, 0, 0, 0, time.UTC)
	second.UpdatedAt = second.CreatedAt

	if _, err := store.CreateClaim(context.Background(), first); err != nil {
		t.Fatalf("create first claim: %v", err)
	}
	if _, err := store.CreateClaim(context.Background(), second); err != nil {
		t.Fatalf("create second claim: %v", err)
	}

	claims, err := store.ListClaimsByAddress(context.Background(), first.Address, 10)
	if err != nil {
		t.Fatalf("list claims: %v", err)
	}
	if len(claims) != 2 {
		t.Fatalf("claims length = %d", len(claims))
	}
	if claims[0].ID != "claim_2" {
		t.Fatalf("first listed claim = %q", claims[0].ID)
	}

	last, err := store.LastClaimByAddress(context.Background(), first.Address)
	if err != nil {
		t.Fatalf("last claim: %v", err)
	}
	if last.ID != "claim_2" {
		t.Fatalf("last claim = %q", last.ID)
	}
}

func TestDailyClaimAmountWeiUsesUTCWindowAndIncludedStatuses(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	dayStart := time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC)
	dayEnd := dayStart.Add(24 * time.Hour)
	included := []domain.ClaimStatus{
		domain.ClaimStatusReceived,
		domain.ClaimStatusValidated,
		domain.ClaimStatusQueued,
		domain.ClaimStatusSending,
		domain.ClaimStatusSent,
		domain.ClaimStatusConfirmed,
	}

	for i, status := range included {
		claim := testClaim("included")
		claim.ID = "included_" + string(rune('a'+i))
		claim.Status = status
		claim.AmountWei = big.NewInt(10)
		claim.CreatedAt = dayStart.Add(time.Duration(i) * time.Hour)
		claim.UpdatedAt = claim.CreatedAt
		if _, err := store.CreateClaim(context.Background(), claim); err != nil {
			t.Fatalf("create included %s: %v", status, err)
		}
	}

	subsecond := testClaim("included_subsecond")
	subsecond.Status = domain.ClaimStatusQueued
	subsecond.AmountWei = big.NewInt(10)
	subsecond.CreatedAt = dayStart.Add(time.Nanosecond)
	subsecond.UpdatedAt = subsecond.CreatedAt
	if _, err := store.CreateClaim(context.Background(), subsecond); err != nil {
		t.Fatalf("create subsecond included: %v", err)
	}

	excluded := testClaim("excluded_failed")
	excluded.Status = domain.ClaimStatusFailed
	excluded.AmountWei = big.NewInt(1000)
	excluded.CreatedAt = dayStart.Add(time.Hour)
	excluded.UpdatedAt = excluded.CreatedAt
	if _, err := store.CreateClaim(context.Background(), excluded); err != nil {
		t.Fatalf("create excluded failed: %v", err)
	}

	previousDay := testClaim("previous_day")
	previousDay.AmountWei = big.NewInt(1000)
	previousDay.CreatedAt = dayStart.Add(-time.Nanosecond)
	previousDay.UpdatedAt = previousDay.CreatedAt
	if _, err := store.CreateClaim(context.Background(), previousDay); err != nil {
		t.Fatalf("create previous day: %v", err)
	}

	total, err := store.DailyClaimAmountWei(context.Background(), dayStart, dayEnd, included)
	if err != nil {
		t.Fatalf("daily claim amount: %v", err)
	}
	if total.Cmp(big.NewInt(70)) != 0 {
		t.Fatalf("total = %s, want 70", total)
	}
}

func TestCreateClaimWithIdempotencyAndDailyBudgetBlocksExceeded(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	dayStart := time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC)
	dayEnd := dayStart.Add(24 * time.Hour)
	existing := testClaim("existing")
	existing.AmountWei = big.NewInt(60)
	existing.CreatedAt = dayStart.Add(time.Hour)
	existing.UpdatedAt = existing.CreatedAt
	if _, err := store.CreateClaim(context.Background(), existing); err != nil {
		t.Fatalf("create existing: %v", err)
	}

	next := testClaim("next")
	next.AmountWei = big.NewInt(50)
	next.CreatedAt = dayStart.Add(2 * time.Hour)
	next.UpdatedAt = next.CreatedAt
	_, used, exceeded, err := store.CreateClaimWithIdempotencyAndDailyBudget(context.Background(), next, "", dayStart, dayEnd, big.NewInt(100), []domain.ClaimStatus{domain.ClaimStatusQueued})
	if err != nil {
		t.Fatalf("create with budget: %v", err)
	}
	if !exceeded {
		t.Fatal("exceeded = false, want true")
	}
	if used.Cmp(big.NewInt(60)) != 0 {
		t.Fatalf("used = %s, want 60", used)
	}
	if _, err := store.GetClaim(context.Background(), next.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("next claim err = %v, want ErrNotFound", err)
	}
}

func TestGetClaimNotFound(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	_, err := store.GetClaim(context.Background(), "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestAllowUnderLimit(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	d, err := store.Allow(context.Background(), "ip:1.2.3.4", 5, time.Hour)
	if err != nil {
		t.Fatalf("allow: %v", err)
	}
	if !d.Allowed {
		t.Fatal("expected allowed")
	}
	if d.Remaining != 4 {
		t.Fatalf("remaining = %d, want 4", d.Remaining)
	}
	if d.Reason != "" {
		t.Fatalf("reason = %q, want empty", d.Reason)
	}
}

func TestAllowExactlyAtLimit(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	for i := range 5 {
		d, err := store.Allow(context.Background(), "ip:1.2.3.4", 5, time.Hour)
		if err != nil {
			t.Fatalf("allow %d: %v", i, err)
		}
		if !d.Allowed {
			t.Fatalf("call %d: expected allowed", i)
		}
	}
}

func TestAllowOverLimit(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	for i := range 3 {
		_, err := store.Allow(context.Background(), "addr:0xABC", 3, time.Hour)
		if err != nil {
			t.Fatalf("allow %d: %v", i, err)
		}
	}

	d, err := store.Allow(context.Background(), "addr:0xABC", 3, time.Hour)
	if err != nil {
		t.Fatalf("allow over limit: %v", err)
	}
	if d.Allowed {
		t.Fatal("expected not allowed on 4th call with limit=3")
	}
	if d.Remaining != 0 {
		t.Fatalf("remaining = %d, want 0", d.Remaining)
	}
	if d.RetryAfter <= 0 {
		t.Fatalf("retry after = %v, want > 0", d.RetryAfter)
	}
	if d.Reason == "" {
		t.Fatal("reason should not be empty when blocked")
	}
}

func TestAllowDifferentKeys(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	// Exhaust limit for key A.
	for i := range 2 {
		_, err := store.Allow(context.Background(), "ip:10.0.0.1", 2, time.Hour)
		if err != nil {
			t.Fatalf("allow key A %d: %v", i, err)
		}
	}

	dA, err := store.Allow(context.Background(), "ip:10.0.0.1", 2, time.Hour)
	if err != nil {
		t.Fatalf("allow key A over limit: %v", err)
	}
	if dA.Allowed {
		t.Fatal("key A should be blocked")
	}

	// Key B should still be allowed.
	dB, err := store.Allow(context.Background(), "ip:10.0.0.2", 2, time.Hour)
	if err != nil {
		t.Fatalf("allow key B: %v", err)
	}
	if !dB.Allowed {
		t.Fatal("key B should be allowed independently")
	}
}

func TestAllowDifferentWindows(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	// Limit=1 per 1h and limit=10 per 24h share the same key.
	// First call should be allowed in both.
	d1h, err := store.Allow(context.Background(), "addr:0xDEF", 1, time.Hour)
	if err != nil {
		t.Fatalf("allow 1h: %v", err)
	}
	if !d1h.Allowed {
		t.Fatal("expected allowed for 1h window")
	}

	d24h, err := store.Allow(context.Background(), "addr:0xDEF", 10, 24*time.Hour)
	if err != nil {
		t.Fatalf("allow 24h: %v", err)
	}
	if !d24h.Allowed {
		t.Fatal("expected allowed for 24h window (independent counter)")
	}
}

func TestCooldownViaRateLimiter(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	key := "cooldown:addr:0xCAFE"
	cooldown := 24 * time.Hour

	// First claim: allowed.
	d, err := store.Allow(context.Background(), key, 1, cooldown)
	if err != nil {
		t.Fatalf("first allow: %v", err)
	}
	if !d.Allowed {
		t.Fatal("first claim should be allowed")
	}

	// Second claim within same window: blocked (cooldown).
	d2, err := store.Allow(context.Background(), key, 1, cooldown)
	if err != nil {
		t.Fatalf("second allow: %v", err)
	}
	if d2.Allowed {
		t.Fatal("second claim within cooldown window should be blocked")
	}
	if d2.RetryAfter <= 0 {
		t.Fatalf("retry after = %v, want > 0", d2.RetryAfter)
	}
}

// ── QueueStore tests ─────────────────────────────────────────────────────────

func TestEnqueueTransitionsToQueued(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	claim := testClaim("q1")
	claim.Status = domain.ClaimStatusReceived
	if _, err := store.CreateClaim(context.Background(), claim); err != nil {
		t.Fatalf("create claim: %v", err)
	}

	if err := store.Enqueue(context.Background(), claim.ID); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	got, err := store.GetClaim(context.Background(), claim.ID)
	if err != nil {
		t.Fatalf("get claim: %v", err)
	}
	if got.Status != domain.ClaimStatusQueued {
		t.Fatalf("status = %q, want queued", got.Status)
	}
}

func TestDequeueBatchTransitionsToSending(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	for i, id := range []string{"q1", "q2", "q3"} {
		c := testClaim(id)
		c.CreatedAt = c.CreatedAt.Add(time.Duration(i) * time.Second)
		c.UpdatedAt = c.CreatedAt
		if _, err := store.CreateClaim(context.Background(), c); err != nil {
			t.Fatalf("create %s: %v", id, err)
		}
	}

	batch, err := store.DequeueBatch(context.Background(), 2)
	if err != nil {
		t.Fatalf("dequeue: %v", err)
	}
	if len(batch) != 2 {
		t.Fatalf("batch len = %d, want 2", len(batch))
	}

	for _, c := range batch {
		got, err := store.GetClaim(context.Background(), c.ID)
		if err != nil {
			t.Fatalf("get %s: %v", c.ID, err)
		}
		if got.Status != domain.ClaimStatusSending {
			t.Fatalf("%s status = %q, want sending", c.ID, got.Status)
		}
	}

	// Third claim should still be queued.
	remaining, err := store.DequeueBatch(context.Background(), 10)
	if err != nil {
		t.Fatalf("second dequeue: %v", err)
	}
	if len(remaining) != 1 || remaining[0].ID != "q3" {
		t.Fatalf("remaining = %v", remaining)
	}
}

func TestAckTransitionsToSent(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	if _, err := store.CreateClaim(context.Background(), testClaim("c1")); err != nil {
		t.Fatalf("create: %v", err)
	}

	batch, err := store.DequeueBatch(context.Background(), 1)
	if err != nil || len(batch) != 1 {
		t.Fatalf("dequeue: %v / len=%d", err, len(batch))
	}

	if err := store.Ack(context.Background(), "c1", domain.Transaction{}); err != nil {
		t.Fatalf("ack: %v", err)
	}

	got, err := store.GetClaim(context.Background(), "c1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != domain.ClaimStatusSent {
		t.Fatalf("status = %q, want sent", got.Status)
	}
}

func TestAckRecordsTransactionRow(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	if _, err := store.CreateClaim(context.Background(), testClaim("tx1")); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := store.DequeueBatch(context.Background(), 1); err != nil {
		t.Fatalf("dequeue: %v", err)
	}

	from := common.HexToAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266")
	to := common.HexToAddress("0x70997970C51812dc3A010C7d01b50e0d17dc79C8")
	txHash := common.HexToHash("0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab")
	now := time.Now().UTC()
	tx := domain.Transaction{
		Hash:      txHash,
		From:      from,
		To:        to,
		ValueWei:  big.NewInt(1e18),
		Status:    domain.ClaimStatusSent,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := store.Ack(context.Background(), "tx1", tx); err != nil {
		t.Fatalf("ack: %v", err)
	}

	var storedHash string
	err := store.db.QueryRow(`SELECT tx_hash FROM transactions WHERE request_id = ?`, "tx1").Scan(&storedHash)
	if err != nil {
		t.Fatalf("query transaction row: %v", err)
	}
	if storedHash != txHash.Hex() {
		t.Fatalf("tx_hash = %q, want %q", storedHash, txHash.Hex())
	}
}

func TestFailWithRetryRequeues(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	if _, err := store.CreateClaim(context.Background(), testClaim("c1")); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := store.DequeueBatch(context.Background(), 1); err != nil {
		t.Fatalf("dequeue: %v", err)
	}

	if err := store.Fail(context.Background(), "c1", "send error", 3); err != nil {
		t.Fatalf("fail: %v", err)
	}

	got, err := store.GetClaim(context.Background(), "c1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != domain.ClaimStatusQueued {
		t.Fatalf("status = %q, want queued", got.Status)
	}
	if got.RetryCount != 1 {
		t.Fatalf("retry_count = %d, want 1", got.RetryCount)
	}
	if got.NextAttemptAt == nil {
		t.Fatal("next_attempt_at should be set after fail")
	}
	if !got.NextAttemptAt.After(time.Now()) {
		t.Fatalf("next_attempt_at %v should be in the future", got.NextAttemptAt)
	}
}

func TestFailExhaustRetriesDeadLetters(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	const maxRetries = 2

	if _, err := store.CreateClaim(context.Background(), testClaim("c1")); err != nil {
		t.Fatalf("create: %v", err)
	}

	// First fail: retry_count becomes 1 (< maxRetries=2) → re-queued.
	if _, err := store.DequeueBatch(context.Background(), 1); err != nil {
		t.Fatalf("first dequeue: %v", err)
	}
	if err := store.Fail(context.Background(), "c1", "err", maxRetries); err != nil {
		t.Fatalf("first fail: %v", err)
	}
	got, _ := store.GetClaim(context.Background(), "c1")
	if got.Status != domain.ClaimStatusQueued {
		t.Fatalf("after 1st fail status = %q, want queued", got.Status)
	}

	// Manually clear next_attempt_at so DequeueBatch picks it up.
	if _, err := store.db.Exec(`UPDATE requests SET next_attempt_at = NULL WHERE id = 'c1'`); err != nil {
		t.Fatalf("clear next_attempt_at: %v", err)
	}

	// Second fail: retry_count becomes 2 (== maxRetries=2) → dead-letter.
	if _, err := store.DequeueBatch(context.Background(), 1); err != nil {
		t.Fatalf("second dequeue: %v", err)
	}
	if err := store.Fail(context.Background(), "c1", "final err", maxRetries); err != nil {
		t.Fatalf("second fail: %v", err)
	}
	got, _ = store.GetClaim(context.Background(), "c1")
	if got.Status != domain.ClaimStatusFailed {
		t.Fatalf("after 2nd fail status = %q, want failed", got.Status)
	}
	if got.RetryCount != 2 {
		t.Fatalf("retry_count = %d, want 2", got.RetryCount)
	}
}

func TestDequeueBatchSkipsFutureNextAttemptAt(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	if _, err := store.CreateClaim(context.Background(), testClaim("c1")); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Dequeue to move to sending, then fail once — next_attempt_at is 30s in future.
	if _, err := store.DequeueBatch(context.Background(), 1); err != nil {
		t.Fatalf("dequeue: %v", err)
	}
	if err := store.Fail(context.Background(), "c1", "err", 3); err != nil {
		t.Fatalf("fail: %v", err)
	}

	// DequeueBatch must not return c1 because next_attempt_at is in the future.
	batch, err := store.DequeueBatch(context.Background(), 10)
	if err != nil {
		t.Fatalf("second dequeue: %v", err)
	}
	for _, c := range batch {
		if c.ID == "c1" {
			t.Fatal("claim c1 should be skipped due to future next_attempt_at")
		}
	}
}

func openTempStore(t *testing.T) *Store {
	t.Helper()

	path := filepath.Join(t.TempDir(), "faucet.db")
	store, err := Open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	return store
}

func testClaim(id string) domain.Claim {
	now := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	return domain.Claim{
		ID:        id,
		Address:   domain.MustValidateAddress("0x52908400098527886E0F7030069857D2E4169EE7"),
		AmountWei: big.NewInt(42),
		Status:    domain.ClaimStatusQueued,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func tableExists(t *testing.T, db *sql.DB, table string) bool {
	t.Helper()

	var count int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&count); err != nil {
		t.Fatalf("query table %s: %v", table, err)
	}
	return count == 1
}

func indexExists(t *testing.T, db *sql.DB, index string) bool {
	t.Helper()

	var count int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'index' AND name = ?`, index).Scan(&count); err != nil {
		t.Fatalf("query index %s: %v", index, err)
	}
	return count == 1
}

// ── WatcherStore tests ────────────────────────────────────────────────────────

func TestListPendingTransactionsReturnsSentClaimsWithTx(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	// Create a claim, send it (Ack with a real hash) → status becomes 'sent'.
	if _, err := store.CreateClaim(context.Background(), testClaim("p1")); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := store.DequeueBatch(context.Background(), 1); err != nil {
		t.Fatalf("dequeue: %v", err)
	}
	txHash := common.HexToHash("0xabc123")
	tx := domain.Transaction{
		Hash:      txHash,
		From:      common.HexToAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"),
		To:        common.HexToAddress("0x70997970C51812dc3A010C7d01b50e0d17dc79C8"),
		ValueWei:  big.NewInt(1e18),
		Status:    domain.ClaimStatusSent,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := store.Ack(context.Background(), "p1", tx); err != nil {
		t.Fatalf("ack: %v", err)
	}

	pending, err := store.ListPendingTransactions(context.Background(), 10)
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("pending len = %d, want 1", len(pending))
	}
	if pending[0].ClaimID != "p1" {
		t.Fatalf("claimID = %q, want p1", pending[0].ClaimID)
	}
	if pending[0].TxHash != txHash {
		t.Fatalf("txHash = %s, want %s", pending[0].TxHash.Hex(), txHash.Hex())
	}
}

func TestConfirmTransactionUpdatesStatusAndRecord(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	// Setup: create → dequeue → ack (sent).
	if _, err := store.CreateClaim(context.Background(), testClaim("conf1")); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := store.DequeueBatch(context.Background(), 1); err != nil {
		t.Fatalf("dequeue: %v", err)
	}
	now := time.Now().UTC()
	tx := domain.Transaction{
		Hash:      common.HexToHash("0xdead01"),
		From:      common.HexToAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"),
		To:        common.HexToAddress("0x70997970C51812dc3A010C7d01b50e0d17dc79C8"),
		ValueWei:  big.NewInt(1e18),
		Status:    domain.ClaimStatusSent,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := store.Ack(context.Background(), "conf1", tx); err != nil {
		t.Fatalf("ack: %v", err)
	}

	if err := store.ConfirmTransaction(context.Background(), "conf1", 500, 21000); err != nil {
		t.Fatalf("confirm: %v", err)
	}

	got, err := store.GetClaim(context.Background(), "conf1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != domain.ClaimStatusConfirmed {
		t.Fatalf("status = %q, want confirmed", got.Status)
	}

	var blockNum, gasUsed uint64
	err = store.db.QueryRow(`SELECT block_number, gas_used FROM transactions WHERE request_id = ?`, "conf1").Scan(&blockNum, &gasUsed)
	if err != nil {
		t.Fatalf("query tx record: %v", err)
	}
	if blockNum != 500 {
		t.Fatalf("block_number = %d, want 500", blockNum)
	}
	if gasUsed != 21000 {
		t.Fatalf("gas_used = %d, want 21000", gasUsed)
	}
}

func TestFailTransactionMarksFailed(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	// Setup: create → dequeue → ack (sent).
	if _, err := store.CreateClaim(context.Background(), testClaim("fail1")); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := store.DequeueBatch(context.Background(), 1); err != nil {
		t.Fatalf("dequeue: %v", err)
	}
	now := time.Now().UTC()
	tx := domain.Transaction{
		Hash:      common.HexToHash("0xdead02"),
		From:      common.HexToAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"),
		To:        common.HexToAddress("0x70997970C51812dc3A010C7d01b50e0d17dc79C8"),
		ValueWei:  big.NewInt(1e18),
		Status:    domain.ClaimStatusSent,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := store.Ack(context.Background(), "fail1", tx); err != nil {
		t.Fatalf("ack: %v", err)
	}

	if err := store.FailTransaction(context.Background(), "fail1", "transaction reverted on-chain"); err != nil {
		t.Fatalf("fail transaction: %v", err)
	}

	got, err := store.GetClaim(context.Background(), "fail1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != domain.ClaimStatusFailed {
		t.Fatalf("status = %q, want failed", got.Status)
	}
	if got.Reason != "transaction reverted on-chain" {
		t.Fatalf("reason = %q", got.Reason)
	}
}

func TestListStuckSendingReturnsSendingOlderThanCutoff(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	// Create two claims and dequeue (moves to 'sending').
	c1 := testClaim("stuck1")
	c1.Status = domain.ClaimStatusQueued
	c2 := testClaim("stuck2")
	c2.Status = domain.ClaimStatusQueued
	if _, err := store.CreateClaim(context.Background(), c1); err != nil {
		t.Fatalf("create c1: %v", err)
	}
	if _, err := store.CreateClaim(context.Background(), c2); err != nil {
		t.Fatalf("create c2: %v", err)
	}
	if _, err := store.DequeueBatch(context.Background(), 2); err != nil {
		t.Fatalf("dequeue: %v", err)
	}

	// Backdate updated_at to simulate stuck claims.
	past := formatTime(time.Now().UTC().Add(-10 * time.Minute))
	if _, err := store.db.Exec(`UPDATE requests SET updated_at = ? WHERE status = 'sending'`, past); err != nil {
		t.Fatalf("backdate: %v", err)
	}

	stuck, err := store.ListStuckSending(context.Background(), 5*time.Minute, 10)
	if err != nil {
		t.Fatalf("list stuck: %v", err)
	}
	if len(stuck) != 2 {
		t.Fatalf("stuck len = %d, want 2", len(stuck))
	}
}

func TestListStuckSendingExcludesRecentClaims(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	if _, err := store.CreateClaim(context.Background(), testClaim("fresh1")); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := store.DequeueBatch(context.Background(), 1); err != nil {
		t.Fatalf("dequeue: %v", err)
	}

	// updated_at is just now — should not be stuck.
	stuck, err := store.ListStuckSending(context.Background(), 5*time.Minute, 10)
	if err != nil {
		t.Fatalf("list stuck: %v", err)
	}
	if len(stuck) != 0 {
		t.Fatalf("expected 0 stuck claims, got %d", len(stuck))
	}
}

func TestRecordAndListAbuseSignals(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	now := time.Date(2026, 5, 4, 13, 0, 0, 0, time.UTC)
	address := common.HexToAddress("0x52908400098527886E0F7030069857D2E4169EE7")
	if err := store.RecordAbuseSignal(context.Background(), domain.AbuseSignal{
		Kind:        domain.AbuseSignalCaptchaFailed,
		Address:     address,
		RemoteIP:    " 203.0.113.10 ",
		Fingerprint: " browser-1 ",
		UserAgent:   " wallet-test/1.0 ",
		ClaimID:     " claim_1 ",
		Reason:      " bad captcha ",
		Score:       7,
		CreatedAt:   now,
	}); err != nil {
		t.Fatalf("record abuse signal: %v", err)
	}

	signals, err := store.ListAbuseSignals(context.Background(), 10)
	if err != nil {
		t.Fatalf("list abuse signals: %v", err)
	}
	if len(signals) != 1 {
		t.Fatalf("signals len = %d, want 1", len(signals))
	}
	signal := signals[0]
	if signal.Kind != domain.AbuseSignalCaptchaFailed {
		t.Fatalf("kind = %q", signal.Kind)
	}
	if signal.Address != address {
		t.Fatalf("address = %s", signal.Address.Hex())
	}
	if signal.RemoteIP != "203.0.113.10" {
		t.Fatalf("remote ip = %q", signal.RemoteIP)
	}
	if signal.Fingerprint != "browser-1" || signal.ClaimID != "claim_1" || signal.Reason != "bad captcha" {
		t.Fatalf("unexpected signal metadata: %+v", signal)
	}
	if signal.Score != 7 {
		t.Fatalf("score = %d, want 7", signal.Score)
	}
	if !signal.CreatedAt.Equal(now) {
		t.Fatalf("created_at = %s, want %s", signal.CreatedAt, now)
	}
}

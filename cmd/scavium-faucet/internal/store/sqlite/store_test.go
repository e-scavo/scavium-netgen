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
)

func TestMigrateCreatesRequiredTables(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	for _, table := range []string{"requests", "transactions", "rate_limits", "config"} {
		if !tableExists(t, store.db, table) {
			t.Fatalf("table %s does not exist", table)
		}
	}
}

func TestMigrateCreatesIndexes(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	for _, index := range []string{"idx_requests_address", "idx_requests_status", "idx_requests_created_at"} {
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

func TestIdempotencyConstraint(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	_, err := store.CreateClaimWithIdempotency(context.Background(), testClaim("claim_1"), "idem-key")
	if err != nil {
		t.Fatalf("create first claim: %v", err)
	}

	_, err = store.CreateClaimWithIdempotency(context.Background(), testClaim("claim_2"), "idem-key")
	if err == nil {
		t.Fatal("create duplicate idempotency key returned nil")
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

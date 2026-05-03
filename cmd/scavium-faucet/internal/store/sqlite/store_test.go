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

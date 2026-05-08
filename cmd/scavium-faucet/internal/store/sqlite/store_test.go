package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"math/big"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"scavium-netgen/cmd/scavium-faucet/internal/abuse"
	"scavium-netgen/cmd/scavium-faucet/internal/domain"
	"scavium-netgen/cmd/scavium-faucet/internal/faucet"

	"github.com/ethereum/go-ethereum/common"
)

func TestSQLiteDSNAppendsBasePragmasAfterCustomQuery(t *testing.T) {
	dsn := sqliteDSN("/tmp/faucet.db?_pragma=synchronous(OFF)")
	if !strings.HasPrefix(dsn, "file:/tmp/faucet.db?_pragma=synchronous(OFF)&") {
		t.Fatalf("dsn = %q, want custom query preserved before base pragmas", dsn)
	}
	if !strings.Contains(dsn, "_pragma=journal_mode(WAL)") {
		t.Fatalf("dsn = %q, want WAL pragma", dsn)
	}
	if !strings.Contains(dsn, "_pragma=busy_timeout(5000)") {
		t.Fatalf("dsn = %q, want busy_timeout pragma", dsn)
	}
}

func TestMigrateCreatesRequiredTables(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	for _, table := range []string{"requests", "transactions", "rate_limits", "config", "abuse_signals", "admin_audit_logs", "admin_blocklist", "runtime_policy", "wallet_challenges", "schema_migrations"} {
		if !tableExists(t, store.db, table) {
			t.Fatalf("table %s does not exist", table)
		}
	}
}

func TestMigrateCreatesIndexes(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	for _, index := range []string{"idx_requests_address", "idx_requests_status", "idx_requests_created_at", "idx_abuse_signals_kind", "idx_abuse_signals_remote_ip", "idx_admin_audit_logs_created_at", "idx_admin_blocklist_type_value", "idx_admin_blocklist_blocked_at", "idx_runtime_policy_updated_at", "idx_wallet_challenges_address", "idx_wallet_challenges_expires_at"} {
		if !indexExists(t, store.db, index) {
			t.Fatalf("index %s does not exist", index)
		}
	}
}

func TestMigrateHandlesPartiallyAppliedAddColumnMigrations(t *testing.T) {
	db, err := sql.Open("sqlite", sqliteDSN(testDatabasePath(t, "partial.db")))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	// Reproduce a production database that has already received some ALTER TABLE
	// columns but does not have the schema_migrations rows for those migrations.
	// Phase 30 must finish the missing columns/indexes instead of crashing on the
	// first duplicate column name.
	_, err = db.Exec(`
		CREATE TABLE requests (
			id TEXT PRIMARY KEY,
			address TEXT NOT NULL,
			amount_wei TEXT NOT NULL,
			status TEXT NOT NULL,
			reason TEXT NOT NULL DEFAULT '',
			idempotency_key TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			retry_count INTEGER NOT NULL DEFAULT 0,
			next_attempt_at TEXT,
			token_id TEXT NOT NULL DEFAULT 'native',
			UNIQUE(idempotency_key)
		);
		CREATE TABLE transactions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			request_id TEXT NOT NULL,
			tx_hash TEXT NOT NULL UNIQUE,
			from_address TEXT NOT NULL,
			to_address TEXT NOT NULL,
			value_wei TEXT NOT NULL,
			status TEXT NOT NULL,
			block_number INTEGER NOT NULL DEFAULT 0,
			gas_used INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			token_id TEXT NOT NULL DEFAULT 'native'
		);
		CREATE TABLE rate_limits (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			limit_key TEXT NOT NULL,
			window_start TEXT NOT NULL,
			window_seconds INTEGER NOT NULL,
			count INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			UNIQUE(limit_key, window_start, window_seconds)
		);
		CREATE TABLE config (key TEXT PRIMARY KEY, value TEXT NOT NULL, updated_at TEXT NOT NULL);
	`)
	if err != nil {
		t.Fatalf("seed partial schema: %v", err)
	}

	store := New(db)
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate partial schema: %v", err)
	}

	for _, column := range []string{"token_id", "token_symbol", "token_type", "token_address", "token_decimals", "campaign_id", "invitation_code"} {
		if !columnExists(t, db, "requests", column) {
			t.Fatalf("requests.%s does not exist", column)
		}
	}
	for _, column := range []string{"token_id", "token_symbol", "token_type", "token_address", "token_decimals"} {
		if !columnExists(t, db, "transactions", column) {
			t.Fatalf("transactions.%s does not exist", column)
		}
	}
	for _, migration := range []string{"002_queue.sql", "004_token_claim_metadata.sql", "008_campaigns_allowlists_invites.sql"} {
		if !migrationRecorded(t, db, migration) {
			t.Fatalf("migration %s was not recorded", migration)
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

func TestListAdminClaims(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	first := testClaim("claim_1")
	first.CreatedAt = time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	first.UpdatedAt = first.CreatedAt
	second := testClaim("claim_2")
	second.CreatedAt = time.Date(2026, 5, 3, 11, 0, 0, 0, time.UTC)
	second.UpdatedAt = second.CreatedAt
	third := testClaim("claim_3")
	third.CreatedAt = time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	third.UpdatedAt = third.CreatedAt

	for _, claim := range []domain.Claim{first, second, third} {
		if _, err := store.CreateClaim(context.Background(), claim); err != nil {
			t.Fatalf("create claim %s: %v", claim.ID, err)
		}
	}

	claims, err := store.ListAdminClaims(context.Background(), 2, 1)
	if err != nil {
		t.Fatalf("list admin claims: %v", err)
	}
	if len(claims) != 2 {
		t.Fatalf("claims length = %d, want 2", len(claims))
	}
	if claims[0].ID != "claim_2" || claims[1].ID != "claim_1" {
		t.Fatalf("claims order = %q, %q; want claim_2, claim_1", claims[0].ID, claims[1].ID)
	}
}

func TestAdminQueueCountsAndListAdminQueueClaims(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	now := time.Now().UTC()
	ready := testClaim("ready")
	ready.Status = domain.ClaimStatusQueued
	ready.CreatedAt = now.Add(-6 * time.Minute)
	ready.UpdatedAt = now.Add(-6 * time.Minute)
	delayed := testClaim("delayed")
	delayed.Status = domain.ClaimStatusQueued
	delayed.CreatedAt = now.Add(-5 * time.Minute)
	delayed.UpdatedAt = now.Add(-5 * time.Minute)
	sending := testClaim("sending")
	sending.Status = domain.ClaimStatusSending
	sending.CreatedAt = now.Add(-4 * time.Minute)
	sending.UpdatedAt = now.Add(-4 * time.Minute)
	sent := testClaim("sent")
	sent.Status = domain.ClaimStatusSent
	sent.CreatedAt = now.Add(-3 * time.Minute)
	sent.UpdatedAt = now.Add(-3 * time.Minute)
	failed := testClaim("failed")
	failed.Status = domain.ClaimStatusFailed
	failed.CreatedAt = now.Add(-2 * time.Minute)
	failed.UpdatedAt = now.Add(-2 * time.Minute)
	confirmed := testClaim("confirmed")
	confirmed.Status = domain.ClaimStatusConfirmed
	confirmed.CreatedAt = now.Add(-1 * time.Minute)
	confirmed.UpdatedAt = now.Add(-1 * time.Minute)

	for _, claim := range []domain.Claim{ready, delayed, sending, sent, failed, confirmed} {
		if _, err := store.CreateClaim(context.Background(), claim); err != nil {
			t.Fatalf("create claim %s: %v", claim.ID, err)
		}
	}
	if err := store.Fail(context.Background(), delayed.ID, "retry later", 3); err != nil {
		t.Fatalf("mark delayed claim for retry: %v", err)
	}

	counts, readyCount, delayedCount, inFlight, pendingTx, terminal, err := store.AdminQueueCounts(context.Background(), now)
	if err != nil {
		t.Fatalf("admin queue counts: %v", err)
	}
	if readyCount != 1 || delayedCount != 1 || inFlight != 1 || pendingTx != 1 || terminal != 2 {
		t.Fatalf("summary = ready:%d delayed:%d inFlight:%d pending:%d terminal:%d", readyCount, delayedCount, inFlight, pendingTx, terminal)
	}
	if counts[string(domain.ClaimStatusQueued)] != 2 || counts[string(domain.ClaimStatusSending)] != 1 || counts[string(domain.ClaimStatusSent)] != 1 || counts[string(domain.ClaimStatusFailed)] != 1 || counts[string(domain.ClaimStatusConfirmed)] != 1 {
		t.Fatalf("counts = %#v", counts)
	}

	items, err := store.ListAdminQueueClaims(context.Background(), 10)
	if err != nil {
		t.Fatalf("list admin queue claims: %v", err)
	}
	if len(items) != 5 {
		t.Fatalf("items length = %d, want 5", len(items))
	}
	for _, item := range items {
		if item.ID == "confirmed" {
			t.Fatal("confirmed claim should not be included in queue items")
		}
	}
}

func TestAdminRetryClaimRequeuesEligibleClaim(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	claim := testClaim("retry_claim")
	claim.Status = domain.ClaimStatusFailed
	claim.Reason = "worker failure"
	if _, err := store.CreateClaim(context.Background(), claim); err != nil {
		t.Fatalf("create claim: %v", err)
	}
	if _, err := store.db.Exec(`UPDATE requests SET next_attempt_at = ? WHERE id = ?`, formatTime(time.Now().UTC().Add(5*time.Minute)), claim.ID); err != nil {
		t.Fatalf("set next_attempt_at: %v", err)
	}

	updated, err := store.AdminRetryClaim(context.Background(), claim.ID)
	if err != nil {
		t.Fatalf("admin retry claim: %v", err)
	}
	if !updated {
		t.Fatal("updated = false, want true")
	}

	got, err := store.GetClaim(context.Background(), claim.ID)
	if err != nil {
		t.Fatalf("get claim: %v", err)
	}
	if got.Status != domain.ClaimStatusQueued {
		t.Fatalf("status = %q, want queued", got.Status)
	}
	if got.Reason != "" {
		t.Fatalf("reason = %q, want empty", got.Reason)
	}
	if got.NextAttemptAt != nil {
		t.Fatalf("next_attempt_at = %v, want nil", got.NextAttemptAt)
	}
}

func TestAdminRetryClaimReturnsFalseWhenNotEligible(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	claim := testClaim("retry_not_eligible")
	claim.Status = domain.ClaimStatusQueued
	if _, err := store.CreateClaim(context.Background(), claim); err != nil {
		t.Fatalf("create claim: %v", err)
	}

	updated, err := store.AdminRetryClaim(context.Background(), claim.ID)
	if err != nil {
		t.Fatalf("admin retry claim: %v", err)
	}
	if updated {
		t.Fatal("updated = true, want false")
	}
}

func TestAdminCancelClaimRejectsEligibleClaim(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	claim := testClaim("cancel_claim")
	claim.Status = domain.ClaimStatusQueued
	if _, err := store.CreateClaim(context.Background(), claim); err != nil {
		t.Fatalf("create claim: %v", err)
	}

	updated, err := store.AdminCancelClaim(context.Background(), claim.ID, "cancelled by admin")
	if err != nil {
		t.Fatalf("admin cancel claim: %v", err)
	}
	if !updated {
		t.Fatal("updated = false, want true")
	}

	got, err := store.GetClaim(context.Background(), claim.ID)
	if err != nil {
		t.Fatalf("get claim: %v", err)
	}
	if got.Status != domain.ClaimStatusRejected {
		t.Fatalf("status = %q, want rejected", got.Status)
	}
	if got.Reason != "cancelled by admin" {
		t.Fatalf("reason = %q, want cancelled by admin", got.Reason)
	}
}

func TestAdminCancelClaimReturnsFalseWhenNotEligible(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	claim := testClaim("cancel_not_eligible")
	claim.Status = domain.ClaimStatusSent
	if _, err := store.CreateClaim(context.Background(), claim); err != nil {
		t.Fatalf("create claim: %v", err)
	}

	updated, err := store.AdminCancelClaim(context.Background(), claim.ID, "cancelled by admin")
	if err != nil {
		t.Fatalf("admin cancel claim: %v", err)
	}
	if updated {
		t.Fatal("updated = true, want false")
	}
}

func TestAppendAndListAdminAudit(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	base := time.Date(2026, 5, 7, 10, 0, 0, 0, time.UTC)
	for i, entry := range []struct {
		action string
		target string
	}{
		{action: "set_mode", target: "faucet"},
		{action: "retry_claim", target: "claim_1"},
		{action: "blocklist_add", target: "blocklist"},
	} {
		if err := store.AppendAdminAudit(context.Background(), domain.AdminAuditEntry{
			Action:    entry.action,
			Actor:     "127.0.0.1",
			Target:    entry.target,
			Detail:    "safe",
			CreatedAt: formatTime(base.Add(time.Duration(i) * time.Minute)),
		}); err != nil {
			t.Fatalf("append admin audit: %v", err)
		}
	}

	entries, err := store.ListAdminAudit(context.Background(), 2)
	if err != nil {
		t.Fatalf("list admin audit: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries len = %d, want 2", len(entries))
	}
	if entries[0].Action != "retry_claim" || entries[1].Action != "blocklist_add" {
		t.Fatalf("actions = %q, %q; want retry_claim, blocklist_add", entries[0].Action, entries[1].Action)
	}
}

func TestAdminBlocklistAddListRemoveAndIsBlocked(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	if err := store.AdminBlocklistAdd(context.Background(), abuse.KeyTypeIP, " 203.0.113.10 ", " abuse "); err != nil {
		t.Fatalf("admin blocklist add ip: %v", err)
	}
	if err := store.AdminBlocklistAdd(context.Background(), abuse.KeyTypeAddress, " 0x52908400098527886E0F7030069857D2E4169EE7 ", "spam"); err != nil {
		t.Fatalf("admin blocklist add address: %v", err)
	}
	if err := store.AdminBlocklistAdd(context.Background(), abuse.KeyTypeFingerprint, " Browser-1 ", "bot"); err != nil {
		t.Fatalf("admin blocklist add fingerprint: %v", err)
	}

	entries, err := store.ListAdminBlocklist(context.Background())
	if err != nil {
		t.Fatalf("list admin blocklist: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("entries len = %d, want 3", len(entries))
	}

	ipBlocked, _, err := store.IsBlocked(context.Background(), abuse.KeyTypeIP, "203.0.113.10")
	if err != nil {
		t.Fatalf("is blocked ip: %v", err)
	}
	if !ipBlocked {
		t.Fatal("expected IP to be blocked")
	}

	addressBlocked, _, err := store.IsBlocked(context.Background(), abuse.KeyTypeAddress, "0x52908400098527886e0f7030069857d2e4169ee7")
	if err != nil {
		t.Fatalf("is blocked address: %v", err)
	}
	if !addressBlocked {
		t.Fatal("expected address to be blocked")
	}

	fingerprintBlocked, _, err := store.IsBlocked(context.Background(), abuse.KeyTypeFingerprint, "browser-1")
	if err != nil {
		t.Fatalf("is blocked fingerprint: %v", err)
	}
	if !fingerprintBlocked {
		t.Fatal("expected fingerprint to be blocked")
	}

	if err := store.AdminBlocklistRemove(context.Background(), abuse.KeyTypeIP, "203.0.113.10"); err != nil {
		t.Fatalf("admin blocklist remove ip: %v", err)
	}

	ipBlocked, _, err = store.IsBlocked(context.Background(), abuse.KeyTypeIP, "203.0.113.10")
	if err != nil {
		t.Fatalf("is blocked ip after remove: %v", err)
	}
	if ipBlocked {
		t.Fatal("expected IP to be unblocked")
	}
}

func TestLastClaimByAddressAndToken(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	nativeClaim := testClaim("claim_native")
	nativeClaim.TokenID = "native"
	nativeClaim.CreatedAt = time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	nativeClaim.UpdatedAt = nativeClaim.CreatedAt
	otherTokenClaim := testClaim("claim_scat")
	otherTokenClaim.TokenID = "scat"
	otherTokenClaim.CreatedAt = time.Date(2026, 5, 3, 11, 0, 0, 0, time.UTC)
	otherTokenClaim.UpdatedAt = otherTokenClaim.CreatedAt

	if _, err := store.CreateClaim(context.Background(), nativeClaim); err != nil {
		t.Fatalf("create native claim: %v", err)
	}
	if _, err := store.CreateClaim(context.Background(), otherTokenClaim); err != nil {
		t.Fatalf("create other token claim: %v", err)
	}

	lastNative, err := store.LastClaimByAddressAndToken(context.Background(), nativeClaim.Address, "native")
	if err != nil {
		t.Fatalf("last native claim: %v", err)
	}
	if lastNative.ID != nativeClaim.ID {
		t.Fatalf("last native claim = %q, want %q", lastNative.ID, nativeClaim.ID)
	}

	lastOther, err := store.LastClaimByAddressAndToken(context.Background(), otherTokenClaim.Address, "scat")
	if err != nil {
		t.Fatalf("last other token claim: %v", err)
	}
	if lastOther.ID != otherTokenClaim.ID {
		t.Fatalf("last other token claim = %q, want %q", lastOther.ID, otherTokenClaim.ID)
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

	store, err := Open(testDatabasePath(t, "faucet.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	return store
}

func testDatabasePath(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join(t.TempDir(), name) + "?_pragma=synchronous(OFF)"
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

func columnExists(t *testing.T, db *sql.DB, table string, column string) bool {
	t.Helper()

	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		t.Fatalf("query columns for %s: %v", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			t.Fatalf("scan column for %s: %v", table, err)
		}
		if name == column {
			return true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate columns for %s: %v", table, err)
	}
	return false
}

func migrationRecorded(t *testing.T, db *sql.DB, migration string) bool {
	t.Helper()

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE name = ?`, migration).Scan(&count); err != nil {
		t.Fatalf("query migration %s: %v", migration, err)
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

func TestCountRecentAbuseSignalsScopesByIP(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	now := time.Date(2026, 5, 4, 16, 0, 0, 0, time.UTC)
	for _, signal := range []domain.AbuseSignal{
		{Kind: domain.AbuseSignalCaptchaFailed, RemoteIP: "203.0.113.10", CreatedAt: now.Add(-10 * time.Minute)},
		{Kind: domain.AbuseSignalRateLimited, RemoteIP: "203.0.113.10", CreatedAt: now.Add(-5 * time.Minute)},
		{Kind: domain.AbuseSignalClaimAccepted, RemoteIP: "203.0.113.10", CreatedAt: now.Add(-4 * time.Minute)},
		{Kind: domain.AbuseSignalCaptchaFailed, RemoteIP: "203.0.113.20", CreatedAt: now.Add(-3 * time.Minute)},
		{Kind: domain.AbuseSignalCaptchaFailed, RemoteIP: "203.0.113.10", CreatedAt: now.Add(-2 * time.Hour)},
	} {
		if err := store.RecordAbuseSignal(context.Background(), signal); err != nil {
			t.Fatalf("record signal: %v", err)
		}
	}

	count, err := store.CountRecentAbuseSignals(context.Background(), domain.AbuseSignalFilter{
		Kinds: []domain.AbuseSignalKind{
			domain.AbuseSignalCaptchaFailed,
			domain.AbuseSignalRateLimited,
		},
		RemoteIP: "203.0.113.10",
		Since:    now.Add(-time.Hour),
	})
	if err != nil {
		t.Fatalf("count recent abuse signals: %v", err)
	}
	if count != 2 {
		t.Fatalf("count = %d, want 2", count)
	}
}

func TestPruneAbuseSignalsRemovesOnlyExpiredRows(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	now := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	for _, signal := range []domain.AbuseSignal{
		{Kind: domain.AbuseSignalCaptchaFailed, RemoteIP: "203.0.113.10", CreatedAt: now.Add(-40 * 24 * time.Hour)},
		{Kind: domain.AbuseSignalRateLimited, RemoteIP: "203.0.113.10", CreatedAt: now.Add(-31 * 24 * time.Hour)},
		{Kind: domain.AbuseSignalClaimAccepted, RemoteIP: "203.0.113.10", CreatedAt: now.Add(-3 * 24 * time.Hour)},
	} {
		if err := store.RecordAbuseSignal(context.Background(), signal); err != nil {
			t.Fatalf("record signal: %v", err)
		}
	}

	removed, err := store.PruneAbuseSignals(context.Background(), now.Add(-30*24*time.Hour))
	if err != nil {
		t.Fatalf("prune abuse signals: %v", err)
	}
	if removed != 2 {
		t.Fatalf("removed = %d, want 2", removed)
	}

	signals, err := store.ListAbuseSignals(context.Background(), 10)
	if err != nil {
		t.Fatalf("list abuse signals: %v", err)
	}
	if len(signals) != 1 || signals[0].Kind != domain.AbuseSignalClaimAccepted {
		t.Fatalf("remaining signals = %+v", signals)
	}
}

func TestListAbuseSignalSummariesGroupsRecentKinds(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	now := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	for _, signal := range []domain.AbuseSignal{
		{Kind: domain.AbuseSignalCaptchaFailed, CreatedAt: now.Add(-10 * time.Minute)},
		{Kind: domain.AbuseSignalCaptchaFailed, CreatedAt: now.Add(-9 * time.Minute)},
		{Kind: domain.AbuseSignalRateLimited, CreatedAt: now.Add(-8 * time.Minute)},
		{Kind: domain.AbuseSignalClaimAccepted, CreatedAt: now.Add(-2 * time.Hour)},
	} {
		if err := store.RecordAbuseSignal(context.Background(), signal); err != nil {
			t.Fatalf("record signal: %v", err)
		}
	}

	summaries, err := store.ListAbuseSignalSummaries(context.Background(), now.Add(-time.Hour), 10)
	if err != nil {
		t.Fatalf("list summaries: %v", err)
	}
	if len(summaries) != 2 {
		t.Fatalf("summaries len = %d, want 2: %+v", len(summaries), summaries)
	}
	if summaries[0].Kind != domain.AbuseSignalCaptchaFailed || summaries[0].Count != 2 {
		t.Fatalf("first summary = %+v", summaries[0])
	}
	if summaries[1].Kind != domain.AbuseSignalRateLimited || summaries[1].Count != 1 {
		t.Fatalf("second summary = %+v", summaries[1])
	}
}

func TestRuntimePolicyRejectsInvalidPersistenceInput(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	cases := []domain.RuntimePolicy{
		{CooldownSeconds: -1},
		{RateLimitIPPerHour: -1},
		{RateLimitAddrPerDay: -1},
		{DailyBudgetWei: big.NewInt(-1)},
		{TokenDailyBudgetWei: map[string]*big.Int{"native": big.NewInt(-1)}},
		{TokenDailyBudgetWei: map[string]*big.Int{"": big.NewInt(1)}},
	}
	for _, policy := range cases {
		if err := store.SetRuntimePolicy(context.Background(), policy); err == nil {
			t.Fatalf("SetRuntimePolicy(%#v) succeeded, want validation error", policy)
		}
	}
}

func TestRuntimePolicyPersistsClearsAndIgnoresInvalidRows(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	policy := domain.RuntimePolicy{
		CooldownSeconds:     45,
		RateLimitIPPerHour:  6,
		RateLimitAddrPerDay: 7,
		DailyBudgetWei:      big.NewInt(1000),
		TokenDailyBudgetWei: map[string]*big.Int{"native": big.NewInt(500)},
	}
	if err := store.SetRuntimePolicy(context.Background(), policy); err != nil {
		t.Fatalf("set runtime policy: %v", err)
	}
	got, err := store.GetRuntimePolicy(context.Background())
	if err != nil {
		t.Fatalf("get runtime policy: %v", err)
	}
	if got.CooldownSeconds != 45 || got.RateLimitIPPerHour != 6 || got.RateLimitAddrPerDay != 7 {
		t.Fatalf("runtime policy ints = %#v", got)
	}
	if got.DailyBudgetWei == nil || got.DailyBudgetWei.String() != "1000" || got.TokenDailyBudgetWei["native"].String() != "500" {
		t.Fatalf("runtime policy budgets = %#v", got)
	}
	if _, err := store.db.Exec(`INSERT OR REPLACE INTO runtime_policy (key, value, updated_at) VALUES ('cooldown_seconds', 'bad', 'now')`); err != nil {
		t.Fatalf("insert invalid policy: %v", err)
	}
	got, err = store.GetRuntimePolicy(context.Background())
	if err != nil {
		t.Fatalf("get runtime policy after invalid row: %v", err)
	}
	if got.CooldownSeconds != 0 {
		t.Fatalf("invalid cooldown = %d, want 0 fallback", got.CooldownSeconds)
	}
	if err := store.ClearRuntimePolicy(context.Background()); err != nil {
		t.Fatalf("clear runtime policy: %v", err)
	}
	got, err = store.GetRuntimePolicy(context.Background())
	if err != nil {
		t.Fatalf("get cleared runtime policy: %v", err)
	}
	if got.CooldownSeconds != 0 || got.DailyBudgetWei != nil || len(got.TokenDailyBudgetWei) != 0 {
		t.Fatalf("cleared runtime policy = %#v", got)
	}
}

func TestCampaignPersistenceAndUsage(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	campaign := domain.Campaign{ID: "camp-1", Name: "Phase 29", Scope: domain.CampaignScopeInvite, TokenID: "native", BudgetWei: big.NewInt(84), Enabled: true, CreatedAt: now, UpdatedAt: now}
	created, err := store.CreateCampaign(context.Background(), campaign)
	if err != nil {
		t.Fatalf("create campaign: %v", err)
	}
	if created.ID != campaign.ID {
		t.Fatalf("campaign id = %q", created.ID)
	}
	got, err := store.GetCampaign(context.Background(), campaign.ID)
	if err != nil {
		t.Fatalf("get campaign: %v", err)
	}
	if got.Scope != domain.CampaignScopeInvite || got.BudgetWei.Cmp(big.NewInt(84)) != 0 {
		t.Fatalf("campaign = %#v", got)
	}

	claim := testClaim("campaign_claim")
	claim.CampaignID = campaign.ID
	if _, err := store.CreateClaim(context.Background(), claim); err != nil {
		t.Fatalf("create campaign claim: %v", err)
	}
	usage, err := store.CampaignUsage(context.Background(), campaign.ID, []domain.ClaimStatus{domain.ClaimStatusQueued})
	if err != nil {
		t.Fatalf("campaign usage: %v", err)
	}
	if usage.ClaimCount != 1 || usage.UsedWei.Cmp(big.NewInt(42)) != 0 {
		t.Fatalf("usage = %#v", usage)
	}
}

func TestCreateClaimWithIdempotencyAndBudgetsRejectsCampaignBudgetAtomically(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	_, err := store.CreateCampaign(context.Background(), domain.Campaign{ID: "atomic-budget", Name: "Atomic Budget", Scope: domain.CampaignScopePublic, BudgetWei: big.NewInt(50), Enabled: true, CreatedAt: now, UpdatedAt: now})
	if err != nil {
		t.Fatalf("create campaign: %v", err)
	}
	existing := testClaim("atomic_existing")
	existing.CampaignID = "atomic-budget"
	existing.AmountWei = big.NewInt(40)
	if _, err := store.CreateClaim(context.Background(), existing); err != nil {
		t.Fatalf("create existing claim: %v", err)
	}

	blocked := testClaim("atomic_blocked")
	blocked.CampaignID = "atomic-budget"
	blocked.AmountWei = big.NewInt(20)
	created, used, reason, err := store.CreateClaimWithIdempotencyAndBudgets(context.Background(), blocked, "", blocked.TokenID, now.Add(-time.Hour), now.Add(time.Hour), nil, "atomic-budget", big.NewInt(50), []domain.ClaimStatus{domain.ClaimStatusQueued})
	if err != nil {
		t.Fatalf("create with campaign budget: %v", err)
	}
	if reason != "campaign_budget_exceeded" {
		t.Fatalf("reason = %q, want campaign_budget_exceeded", reason)
	}
	if used == nil || used.Cmp(big.NewInt(40)) != 0 {
		t.Fatalf("used = %v, want 40", used)
	}
	if created.ID != "" {
		t.Fatalf("created = %#v, want zero claim", created)
	}
	if _, err := store.GetClaim(context.Background(), "atomic_blocked"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("blocked claim lookup error = %v, want not found", err)
	}
}

func TestInvitationAndAllowlistPersistence(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	_, err := store.CreateCampaign(context.Background(), domain.Campaign{ID: "camp-allow", Name: "Allow", Scope: domain.CampaignScopeAllowlist, Enabled: true, CreatedAt: now, UpdatedAt: now})
	if err != nil {
		t.Fatalf("create campaign: %v", err)
	}
	code, err := store.CreateInvitationCode(context.Background(), domain.InvitationCode{Code: "INVITE-1", CampaignID: "camp-allow", MaxUses: 1, Enabled: true, CreatedAt: now, UpdatedAt: now})
	if err != nil {
		t.Fatalf("create invitation: %v", err)
	}
	if code.Uses != 0 {
		t.Fatalf("initial uses = %d", code.Uses)
	}
	if err := store.ConsumeInvitationCode(context.Background(), code.Code); err != nil {
		t.Fatalf("consume invitation: %v", err)
	}
	if err := store.ConsumeInvitationCode(context.Background(), code.Code); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("second consume err = %v, want not found", err)
	}

	addr := domain.MustValidateAddress("0x52908400098527886E0F7030069857D2E4169EE7")
	if err := store.AddCampaignAllowlistEntry(context.Background(), domain.CampaignAllowlistEntry{CampaignID: "camp-allow", Address: addr, CreatedAt: now}); err != nil {
		t.Fatalf("allowlist add: %v", err)
	}
	allowed, err := store.IsAddressAllowlisted(context.Background(), "camp-allow", addr)
	if err != nil {
		t.Fatalf("allowlist check: %v", err)
	}
	if !allowed {
		t.Fatal("address not allowlisted")
	}
}

func TestCampaignReferencesRequireExistingCampaign(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	_, err := store.CreateInvitationCode(context.Background(), domain.InvitationCode{Code: "MISSING", CampaignID: "missing", MaxUses: 1, Enabled: true, CreatedAt: now, UpdatedAt: now})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("CreateInvitationCode error = %v, want ErrNotFound", err)
	}
	addr := domain.MustValidateAddress("0x52908400098527886E0F7030069857D2E4169EE7")
	if err := store.AddCampaignAllowlistEntry(context.Background(), domain.CampaignAllowlistEntry{CampaignID: "missing", Address: addr, CreatedAt: now}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("AddCampaignAllowlistEntry error = %v, want ErrNotFound", err)
	}
}

func TestCampaignAllowlistAddIsIdempotentWithoutReplacingExistingEntry(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	_, err := store.CreateCampaign(context.Background(), domain.Campaign{ID: "camp-idem", Name: "Idempotent", Scope: domain.CampaignScopeAllowlist, Enabled: true, CreatedAt: now, UpdatedAt: now})
	if err != nil {
		t.Fatalf("create campaign: %v", err)
	}
	addr := domain.MustValidateAddress("0x52908400098527886E0F7030069857D2E4169EE7")
	if err := store.AddCampaignAllowlistEntry(context.Background(), domain.CampaignAllowlistEntry{CampaignID: "camp-idem", Address: addr, Note: "first", CreatedAt: now}); err != nil {
		t.Fatalf("first allowlist add: %v", err)
	}
	if err := store.AddCampaignAllowlistEntry(context.Background(), domain.CampaignAllowlistEntry{CampaignID: "camp-idem", Address: addr, Note: "second", CreatedAt: now.Add(time.Hour)}); err != nil {
		t.Fatalf("second allowlist add: %v", err)
	}
	var note string
	if err := store.db.QueryRow(`SELECT note FROM campaign_allowlist WHERE campaign_id = ? AND address = ?`, "camp-idem", addr.Hex()).Scan(&note); err != nil {
		t.Fatalf("read allowlist note: %v", err)
	}
	if note != "first" {
		t.Fatalf("allowlist note = %q, want first", note)
	}
}

func TestCampaignUpdatePersistence(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()

	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	created, err := store.CreateCampaign(context.Background(), domain.Campaign{ID: "camp-update", Name: "Before", Scope: domain.CampaignScopePublic, Enabled: true, CreatedAt: now, UpdatedAt: now})
	if err != nil {
		t.Fatalf("create campaign: %v", err)
	}
	created.Name = "After"
	created.Scope = domain.CampaignScopeInvite
	created.BudgetWei = big.NewInt(99)
	created.Enabled = false
	created.UpdatedAt = now.Add(time.Hour)
	updated, err := store.UpdateCampaign(context.Background(), created)
	if err != nil {
		t.Fatalf("update campaign: %v", err)
	}
	if updated.Name != "After" || updated.Scope != domain.CampaignScopeInvite || updated.BudgetWei.String() != "99" || updated.Enabled {
		t.Fatalf("updated = %#v", updated)
	}
	got, err := store.GetCampaign(context.Background(), "camp-update")
	if err != nil {
		t.Fatalf("get campaign: %v", err)
	}
	if got.Name != "After" || got.Scope != domain.CampaignScopeInvite || got.BudgetWei.String() != "99" || got.Enabled {
		t.Fatalf("got = %#v", got)
	}
}

func TestWalletChallengesPersistExpireAndReplay(t *testing.T) {
	store := openTempStore(t)
	defer store.Close()
	address := common.HexToAddress("0x52908400098527886E0F7030069857D2E4169EE7")
	now := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	challenge := faucet.WalletChallenge{ID: "wch_test", Address: address, Nonce: "nonce", Message: "message", CreatedAt: now, ExpiresAt: now.Add(5 * time.Minute)}
	if _, err := store.CreateWalletChallenge(context.Background(), challenge); err != nil {
		t.Fatalf("create challenge: %v", err)
	}
	consumed, err := store.ConsumeWalletChallenge(context.Background(), challenge.ID, address, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("consume challenge: %v", err)
	}
	if consumed.ConsumedAt == nil {
		t.Fatal("consumed_at is nil")
	}
	if _, err := store.ConsumeWalletChallenge(context.Background(), challenge.ID, address, now.Add(2*time.Minute)); !errors.Is(err, faucet.ErrWalletChallengeInvalid) {
		t.Fatalf("replay err = %v, want invalid", err)
	}

	expired := faucet.WalletChallenge{ID: "wch_expired", Address: address, Nonce: "nonce2", Message: "message2", CreatedAt: now, ExpiresAt: now.Add(time.Minute)}
	if _, err := store.CreateWalletChallenge(context.Background(), expired); err != nil {
		t.Fatalf("create expired challenge: %v", err)
	}
	if _, err := store.ConsumeWalletChallenge(context.Background(), expired.ID, address, now.Add(2*time.Minute)); !errors.Is(err, faucet.ErrWalletChallengeInvalid) {
		t.Fatalf("expired err = %v, want invalid", err)
	}
}

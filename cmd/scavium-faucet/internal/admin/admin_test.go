package admin

import (
	"context"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"scavium-netgen/cmd/scavium-faucet/internal/abuse"
	"scavium-netgen/cmd/scavium-faucet/internal/domain"
	storesqlite "scavium-netgen/cmd/scavium-faucet/internal/store/sqlite"

	"github.com/ethereum/go-ethereum/common"
)

// ---- AuditLog tests -----------------------------------------------------

func TestAuditLogAppendAndRecent(t *testing.T) {
	al := NewAuditLog(10)
	al.Append(AuditEntry{Action: "a1", Actor: "op", Target: "t1", CreatedAt: "2026-05-03T10:00:00Z"})
	al.Append(AuditEntry{Action: "a2", Actor: "op", Target: "t2", CreatedAt: "2026-05-03T10:01:00Z"})

	entries := al.Recent(10)
	if len(entries) != 2 {
		t.Fatalf("len = %d, want 2", len(entries))
	}
	if entries[0].Action != "a1" {
		t.Fatalf("first action = %q, want a1", entries[0].Action)
	}
}

func TestAuditLogRecentLimitsResults(t *testing.T) {
	al := NewAuditLog(50)
	for i := 0; i < 5; i++ {
		al.Append(AuditEntry{Action: "x"})
	}
	got := al.Recent(3)
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
}

func TestAuditLogEvictsOldestWhenFull(t *testing.T) {
	al := NewAuditLog(3)
	al.Append(AuditEntry{Action: "first"})
	al.Append(AuditEntry{Action: "second"})
	al.Append(AuditEntry{Action: "third"})
	al.Append(AuditEntry{Action: "fourth"}) // should evict "first"

	entries := al.Recent(10)
	if len(entries) != 3 {
		t.Fatalf("len = %d, want 3", len(entries))
	}
	if entries[0].Action != "second" {
		t.Fatalf("oldest = %q, want second", entries[0].Action)
	}
}

func TestAuditLogRecentOnEmpty(t *testing.T) {
	al := NewAuditLog(10)
	if entries := al.Recent(5); entries != nil {
		t.Fatalf("expected nil, got %v", entries)
	}
}

// ---- TokenAuthMiddleware tests ------------------------------------------

func TestTokenAuthMiddlewarePasses(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := TokenAuthMiddleware("secret-token", inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestTokenAuthMiddlewareRejectsWrongToken(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := TokenAuthMiddleware("secret-token", inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestTokenAuthMiddlewareRejectsMissingToken(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := TokenAuthMiddleware("secret-token", inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestTokenAuthMiddlewareDisabled(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := TokenAuthMiddleware("", inner) // empty token = disabled

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer anything")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}

// ---- InMemoryAdminService tests -----------------------------------------

func testClaim(id string, status domain.ClaimStatus) domain.Claim {
	return domain.Claim{
		ID:        id,
		Address:   common.HexToAddress("0x52908400098527886E0F7030069857D2E4169EE7"),
		AmountWei: big.NewInt(42),
		Status:    status,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
}

func TestInMemoryAdminServiceDashboard(t *testing.T) {
	svc := NewInMemoryAdminService()
	svc.AddClaim(testClaim("c1", domain.ClaimStatusQueued))
	svc.AddClaim(testClaim("c2", domain.ClaimStatusConfirmed))
	svc.AddClaim(testClaim("c3", domain.ClaimStatusQueued))

	dash, err := svc.Dashboard(context.Background())
	if err != nil {
		t.Fatalf("Dashboard: %v", err)
	}
	if dash.Mode != "active" {
		t.Fatalf("mode = %q, want active", dash.Mode)
	}
	if dash.ClaimCounts["queued"] != 2 {
		t.Fatalf("queued = %d, want 2", dash.ClaimCounts["queued"])
	}
	if dash.ClaimCounts["confirmed"] != 1 {
		t.Fatalf("confirmed = %d, want 1", dash.ClaimCounts["confirmed"])
	}
}

func TestInMemoryAdminServiceSetMode(t *testing.T) {
	svc := NewInMemoryAdminService()

	if err := svc.SetMode(context.Background(), "paused", "test-actor"); err != nil {
		t.Fatalf("SetMode: %v", err)
	}

	dash, _ := svc.Dashboard(context.Background())
	if dash.Mode != "paused" {
		t.Fatalf("mode = %q, want paused", dash.Mode)
	}

	// Audit entry should be recorded.
	entries, _ := svc.RecentAuditLog(context.Background(), 10)
	if len(entries) == 0 {
		t.Fatal("expected audit entry after SetMode")
	}
	if entries[len(entries)-1].Action != "set_mode" {
		t.Fatalf("audit action = %q, want set_mode", entries[len(entries)-1].Action)
	}
}

func TestInMemoryAdminServiceSetModeRejectsInvalidMode(t *testing.T) {
	svc := NewInMemoryAdminService()

	if err := svc.SetMode(context.Background(), "drain-all-funds", "test-actor"); err != ErrInvalidMode {
		t.Fatalf("error = %v, want ErrInvalidMode", err)
	}

	dash, _ := svc.Dashboard(context.Background())
	if dash.Mode != "active" {
		t.Fatalf("mode = %q, want active", dash.Mode)
	}
	entries, _ := svc.RecentAuditLog(context.Background(), 10)
	if len(entries) != 0 {
		t.Fatalf("audit entries = %d, want 0", len(entries))
	}
}

func TestInMemoryAdminServiceGetClaim(t *testing.T) {
	svc := NewInMemoryAdminService()
	svc.AddClaim(testClaim("c1", domain.ClaimStatusQueued))

	c, found, err := svc.GetClaim(context.Background(), "c1")
	if err != nil {
		t.Fatalf("GetClaim: %v", err)
	}
	if !found {
		t.Fatal("claim not found")
	}
	if c.ID != "c1" {
		t.Fatalf("id = %q", c.ID)
	}
}

func TestInMemoryAdminServiceGetClaimNotFound(t *testing.T) {
	svc := NewInMemoryAdminService()
	_, found, err := svc.GetClaim(context.Background(), "missing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Fatal("expected not found")
	}
}

func TestInMemoryAdminServiceRetryClaim(t *testing.T) {
	svc := NewInMemoryAdminService()
	svc.AddClaim(testClaim("c1", domain.ClaimStatusFailed))

	if err := svc.RetryClaim(context.Background(), "c1", "admin"); err != nil {
		t.Fatalf("RetryClaim: %v", err)
	}

	c, found, _ := svc.GetClaim(context.Background(), "c1")
	if !found || c.Status != domain.ClaimStatusQueued {
		t.Fatalf("status = %q, want queued", c.Status)
	}
}

func TestInMemoryAdminServiceRetryClaimNotRetryable(t *testing.T) {
	svc := NewInMemoryAdminService()
	svc.AddClaim(testClaim("c1", domain.ClaimStatusConfirmed))

	err := svc.RetryClaim(context.Background(), "c1", "admin")
	if err != ErrNotRetryable {
		t.Fatalf("error = %v, want ErrNotRetryable", err)
	}
}

func TestInMemoryAdminServiceRetryClaimNotFound(t *testing.T) {
	svc := NewInMemoryAdminService()
	err := svc.RetryClaim(context.Background(), "missing", "admin")
	if err != ErrNotFound {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

func TestInMemoryAdminServiceCancelClaim(t *testing.T) {
	svc := NewInMemoryAdminService()
	svc.AddClaim(testClaim("c1", domain.ClaimStatusQueued))

	if err := svc.CancelClaim(context.Background(), "c1", "admin"); err != nil {
		t.Fatalf("CancelClaim: %v", err)
	}

	c, _, _ := svc.GetClaim(context.Background(), "c1")
	if c.Status != domain.ClaimStatusRejected {
		t.Fatalf("status = %q, want rejected", c.Status)
	}
	if c.Reason != "cancelled by admin" {
		t.Fatalf("reason = %q", c.Reason)
	}
}

func TestInMemoryAdminServiceCancelClaimNotCancellable(t *testing.T) {
	svc := NewInMemoryAdminService()
	svc.AddClaim(testClaim("c1", domain.ClaimStatusSent))

	err := svc.CancelClaim(context.Background(), "c1", "admin")
	if err != ErrNotCancellable {
		t.Fatalf("error = %v, want ErrNotCancellable", err)
	}
}

func TestInMemoryAdminServiceBlocklist(t *testing.T) {
	svc := NewInMemoryAdminService()

	if err := svc.BlocklistAdd(context.Background(), abuse.KeyTypeIP, "1.2.3.4", "spam", "admin"); err != nil {
		t.Fatalf("BlocklistAdd: %v", err)
	}

	entries, err := svc.BlocklistList(context.Background())
	if err != nil {
		t.Fatalf("BlocklistList: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len = %d, want 1", len(entries))
	}

	if err := svc.BlocklistRemove(context.Background(), abuse.KeyTypeIP, "1.2.3.4", "admin"); err != nil {
		t.Fatalf("BlocklistRemove: %v", err)
	}

	entries, _ = svc.BlocklistList(context.Background())
	if len(entries) != 0 {
		t.Fatalf("len = %d after remove, want 0", len(entries))
	}
}

func TestInMemoryAdminServiceListClaims(t *testing.T) {
	svc := NewInMemoryAdminService()
	svc.AddClaim(testClaim("c1", domain.ClaimStatusQueued))
	svc.AddClaim(testClaim("c2", domain.ClaimStatusFailed))

	claims, err := svc.ListClaims(context.Background(), 10, 0)
	if err != nil {
		t.Fatalf("ListClaims: %v", err)
	}
	if len(claims) != 2 {
		t.Fatalf("len = %d, want 2", len(claims))
	}
}

func TestInMemoryAdminServiceListClaimsOffset(t *testing.T) {
	svc := NewInMemoryAdminService()
	svc.AddClaim(testClaim("c1", domain.ClaimStatusQueued))
	claims, err := svc.ListClaims(context.Background(), 10, 5)
	if err != nil {
		t.Fatalf("ListClaims: %v", err)
	}
	if len(claims) != 0 {
		t.Fatalf("expected empty with offset > len, got %d", len(claims))
	}
}

func TestInMemoryAdminServiceQueue(t *testing.T) {
	now := time.Date(2026, 5, 5, 10, 0, 0, 0, time.UTC)
	svc := NewInMemoryAdminService().withClock(func() time.Time { return now })
	ready := testClaim("ready", domain.ClaimStatusQueued)
	ready.CreatedAt = now.Add(-3 * time.Minute)
	ready.UpdatedAt = now.Add(-3 * time.Minute)
	delayedAt := now.Add(10 * time.Minute)
	delayed := testClaim("delayed", domain.ClaimStatusQueued)
	delayed.NextAttemptAt = &delayedAt
	delayed.CreatedAt = now.Add(-2 * time.Minute)
	delayed.UpdatedAt = now.Add(-2 * time.Minute)
	sending := testClaim("sending", domain.ClaimStatusSending)
	sending.CreatedAt = now.Add(-1 * time.Minute)
	sending.UpdatedAt = now.Add(-1 * time.Minute)
	sent := testClaim("sent", domain.ClaimStatusSent)
	failed := testClaim("failed", domain.ClaimStatusFailed)
	confirmed := testClaim("confirmed", domain.ClaimStatusConfirmed)

	svc.AddClaim(ready)
	svc.AddClaim(delayed)
	svc.AddClaim(sending)
	svc.AddClaim(sent)
	svc.AddClaim(failed)
	svc.AddClaim(confirmed)

	queue, err := svc.Queue(context.Background(), 10)
	if err != nil {
		t.Fatalf("Queue: %v", err)
	}
	if queue.Ready != 1 || queue.Delayed != 1 || queue.InFlight != 1 || queue.PendingTx != 1 || queue.Terminal != 2 {
		t.Fatalf("summary = %#v", queue)
	}
	if queue.Counts["queued"] != 2 || queue.Counts["sending"] != 1 || queue.Counts["sent"] != 1 || queue.Counts["failed"] != 1 || queue.Counts["confirmed"] != 1 {
		t.Fatalf("counts = %#v", queue.Counts)
	}
	if len(queue.Items) != 5 {
		t.Fatalf("items len = %d, want 5", len(queue.Items))
	}
	for _, item := range queue.Items {
		if item.ID == "confirmed" {
			t.Fatal("confirmed claim should not be included in queue items")
		}
	}
}

func TestInMemoryAdminServiceQueueLimit(t *testing.T) {
	svc := NewInMemoryAdminService()
	svc.AddClaim(testClaim("c1", domain.ClaimStatusQueued))
	svc.AddClaim(testClaim("c2", domain.ClaimStatusQueued))

	queue, err := svc.Queue(context.Background(), 1)
	if err != nil {
		t.Fatalf("Queue: %v", err)
	}
	if len(queue.Items) != 1 {
		t.Fatalf("items len = %d, want 1", len(queue.Items))
	}
	if queue.Counts["queued"] != 2 {
		t.Fatalf("queued count = %d, want 2", queue.Counts["queued"])
	}
}

func TestInMemoryAdminServiceRecentAuditLog(t *testing.T) {
	svc := NewInMemoryAdminService()
	_ = svc.SetMode(context.Background(), "paused", "a")
	_ = svc.SetMode(context.Background(), "active", "a")

	entries, err := svc.RecentAuditLog(context.Background(), 1)
	if err != nil {
		t.Fatalf("RecentAuditLog: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len = %d, want 1", len(entries))
	}
	if entries[0].Detail != "active" {
		t.Fatalf("detail = %q, want active", entries[0].Detail)
	}
}

type testModeController struct {
	mode string
}

func (c *testModeController) SetFaucetMode(mode string) {
	c.mode = mode
}

func TestInMemoryAdminServiceSetModePropagatesToController(t *testing.T) {
	controller := &testModeController{}
	svc := NewInMemoryAdminService()
	svc.SetModeController(controller)

	if err := svc.SetMode(context.Background(), "maintenance", "operator"); err != nil {
		t.Fatalf("SetMode: %v", err)
	}
	if controller.mode != "maintenance" {
		t.Fatalf("controller mode = %q, want maintenance", controller.mode)
	}
}

func TestInMemoryAdminServiceSetModeDoesNotPropagateInvalidMode(t *testing.T) {
	controller := &testModeController{mode: "active"}
	svc := NewInMemoryAdminService()
	svc.SetModeController(controller)

	if err := svc.SetMode(context.Background(), "invalid", "operator"); err != ErrInvalidMode {
		t.Fatalf("SetMode err = %v, want ErrInvalidMode", err)
	}
	if controller.mode != "active" {
		t.Fatalf("controller mode = %q, want active", controller.mode)
	}
}

func TestSQLiteReadAdminServiceReadsPersistedClaimsAndQueue(t *testing.T) {
	store, err := storesqlite.Open(filepath.Join(t.TempDir(), "admin.db") + "?_pragma=synchronous(OFF)")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	now := time.Now().UTC()
	ready := testClaim("ready", domain.ClaimStatusQueued)
	ready.CreatedAt = now.Add(-6 * time.Minute)
	ready.UpdatedAt = now.Add(-6 * time.Minute)
	delayed := testClaim("delayed", domain.ClaimStatusQueued)
	delayed.CreatedAt = now.Add(-5 * time.Minute)
	delayed.UpdatedAt = now.Add(-5 * time.Minute)
	sending := testClaim("sending", domain.ClaimStatusSending)
	sending.CreatedAt = now.Add(-4 * time.Minute)
	sending.UpdatedAt = now.Add(-4 * time.Minute)
	sent := testClaim("sent", domain.ClaimStatusSent)
	sent.CreatedAt = now.Add(-3 * time.Minute)
	sent.UpdatedAt = now.Add(-3 * time.Minute)
	failed := testClaim("failed", domain.ClaimStatusFailed)
	failed.CreatedAt = now.Add(-2 * time.Minute)
	failed.UpdatedAt = now.Add(-2 * time.Minute)
	confirmed := testClaim("confirmed", domain.ClaimStatusConfirmed)
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

	svc := NewSQLiteReadAdminService(store).withClock(time.Now)

	dash, err := svc.Dashboard(context.Background())
	if err != nil {
		t.Fatalf("dashboard: %v", err)
	}
	if dash.ClaimCounts["queued"] != 2 || dash.ClaimCounts["confirmed"] != 1 {
		t.Fatalf("claim counts = %#v", dash.ClaimCounts)
	}

	queue, err := svc.Queue(context.Background(), 10)
	if err != nil {
		t.Fatalf("queue: %v", err)
	}
	if queue.Ready != 1 || queue.Delayed != 1 || queue.InFlight != 1 || queue.PendingTx != 1 || queue.Terminal != 2 {
		t.Fatalf("summary = %#v", queue)
	}
	if len(queue.Items) != 5 {
		t.Fatalf("items len = %d, want 5", len(queue.Items))
	}
	for _, item := range queue.Items {
		if item.ID == "confirmed" {
			t.Fatal("confirmed claim should not be included in queue items")
		}
	}

	claims, err := svc.ListClaims(context.Background(), 2, 1)
	if err != nil {
		t.Fatalf("list claims: %v", err)
	}
	if len(claims) != 2 || claims[0].ID != "failed" || claims[1].ID != "sent" {
		t.Fatalf("claims = %#v", claims)
	}

	claim, found, err := svc.GetClaim(context.Background(), ready.ID)
	if err != nil {
		t.Fatalf("get claim: %v", err)
	}
	if !found || claim.ID != ready.ID {
		t.Fatalf("found = %v, id = %q", found, claim.ID)
	}
}

func TestSQLiteReadAdminServiceUsesPersistedRetryCancel(t *testing.T) {
	store, err := storesqlite.Open(filepath.Join(t.TempDir(), "admin.db") + "?_pragma=synchronous(OFF)")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	controller := &testModeController{}
	svc := NewSQLiteReadAdminService(store)
	svc.SetModeController(controller)

	retryClaim := testClaim("retry_claim", domain.ClaimStatusFailed)
	retryClaim.CreatedAt = time.Now().UTC().Add(-2 * time.Minute)
	retryClaim.UpdatedAt = retryClaim.CreatedAt
	cancelClaim := testClaim("cancel_claim", domain.ClaimStatusQueued)
	cancelClaim.CreatedAt = time.Now().UTC().Add(-1 * time.Minute)
	cancelClaim.UpdatedAt = cancelClaim.CreatedAt
	queuedClaim := testClaim("queued_claim", domain.ClaimStatusQueued)
	sentClaim := testClaim("sent_claim", domain.ClaimStatusSent)

	for _, claim := range []domain.Claim{retryClaim, cancelClaim, queuedClaim, sentClaim} {
		if _, err := store.CreateClaim(context.Background(), claim); err != nil {
			t.Fatalf("create claim %s: %v", claim.ID, err)
		}
	}

	if err := svc.RetryClaim(context.Background(), retryClaim.ID, "operator"); err != nil {
		t.Fatalf("retry claim: %v", err)
	}
	retried, err := store.GetClaim(context.Background(), retryClaim.ID)
	if err != nil {
		t.Fatalf("get retried claim: %v", err)
	}
	if retried.Status != domain.ClaimStatusQueued {
		t.Fatalf("status = %q, want queued", retried.Status)
	}

	if err := svc.CancelClaim(context.Background(), cancelClaim.ID, "operator"); err != nil {
		t.Fatalf("cancel claim: %v", err)
	}
	cancelled, err := store.GetClaim(context.Background(), cancelClaim.ID)
	if err != nil {
		t.Fatalf("get cancelled claim: %v", err)
	}
	if cancelled.Status != domain.ClaimStatusRejected {
		t.Fatalf("status = %q, want rejected", cancelled.Status)
	}
	if cancelled.Reason != "cancelled by admin" {
		t.Fatalf("reason = %q, want cancelled by admin", cancelled.Reason)
	}

	if err := svc.RetryClaim(context.Background(), queuedClaim.ID, "operator"); err != ErrNotRetryable {
		t.Fatalf("retry err = %v, want ErrNotRetryable", err)
	}
	if err := svc.CancelClaim(context.Background(), sentClaim.ID, "operator"); err != ErrNotCancellable {
		t.Fatalf("cancel err = %v, want ErrNotCancellable", err)
	}
	if err := svc.RetryClaim(context.Background(), "missing", "operator"); err != ErrNotFound {
		t.Fatalf("retry missing err = %v, want ErrNotFound", err)
	}

	if err := svc.SetMode(context.Background(), "maintenance", "operator"); err != nil {
		t.Fatalf("set mode: %v", err)
	}
	if controller.mode != "maintenance" {
		t.Fatalf("controller mode = %q, want maintenance", controller.mode)
	}

	dash, err := svc.Dashboard(context.Background())
	if err != nil {
		t.Fatalf("dashboard: %v", err)
	}
	if dash.Mode != "maintenance" {
		t.Fatalf("mode = %q, want maintenance", dash.Mode)
	}

	if err := svc.BlocklistAdd(context.Background(), abuse.KeyTypeIP, "203.0.113.10", "spam", "operator"); err != nil {
		t.Fatalf("blocklist add: %v", err)
	}
	persisted := NewSQLiteReadAdminService(store)
	entries, err := svc.BlocklistList(context.Background())
	if err != nil {
		t.Fatalf("blocklist list: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries len = %d, want 1", len(entries))
	}
	persistedEntries, err := persisted.BlocklistList(context.Background())
	if err != nil {
		t.Fatalf("persisted blocklist list: %v", err)
	}
	if len(persistedEntries) != 1 {
		t.Fatalf("persisted entries len = %d, want 1", len(persistedEntries))
	}
	if persistedEntries[0].Key != "203.0.113.10" {
		t.Fatalf("persisted key = %q, want 203.0.113.10", persistedEntries[0].Key)
	}

	logs, err := svc.RecentAuditLog(context.Background(), 20)
	if err != nil {
		t.Fatalf("audit log: %v", err)
	}
	if len(logs) < 4 {
		t.Fatalf("audit entries = %d, want at least 4", len(logs))
	}
	for _, entry := range logs {
		if entry.Action != "blocklist_add" {
			continue
		}
		if entry.Target != "blocklist" {
			t.Fatalf("blocklist audit target = %q, want blocklist", entry.Target)
		}
		if entry.Detail != string(abuse.KeyTypeIP) {
			t.Fatalf("blocklist audit detail = %q, want %q", entry.Detail, abuse.KeyTypeIP)
		}
	}
}

type failingRuntimePolicyAuditStore struct {
	policy    domain.RuntimePolicy
	appendErr error
}

func (s *failingRuntimePolicyAuditStore) GetClaim(context.Context, string) (domain.Claim, error) {
	return domain.Claim{}, nil
}

func (s *failingRuntimePolicyAuditStore) ListAdminClaims(context.Context, int, int) ([]domain.Claim, error) {
	return nil, nil
}

func (s *failingRuntimePolicyAuditStore) AdminClaimCounts(context.Context) (map[string]int, error) {
	return nil, nil
}

func (s *failingRuntimePolicyAuditStore) AdminQueueCounts(context.Context, time.Time) (map[string]int, int, int, int, int, int, error) {
	return nil, 0, 0, 0, 0, 0, nil
}

func (s *failingRuntimePolicyAuditStore) ListAdminQueueClaims(context.Context, int) ([]domain.Claim, error) {
	return nil, nil
}

func (s *failingRuntimePolicyAuditStore) GetRuntimePolicy(context.Context) (domain.RuntimePolicy, error) {
	return domain.CopyRuntimePolicy(s.policy), nil
}

func (s *failingRuntimePolicyAuditStore) SetRuntimePolicy(_ context.Context, policy domain.RuntimePolicy) error {
	s.policy = domain.CopyRuntimePolicy(policy)
	return nil
}

func (s *failingRuntimePolicyAuditStore) ClearRuntimePolicy(context.Context) error {
	s.policy = domain.RuntimePolicy{}
	return nil
}

func (s *failingRuntimePolicyAuditStore) AppendAdminAudit(context.Context, domain.AdminAuditEntry) error {
	return s.appendErr
}

func (s *failingRuntimePolicyAuditStore) ListAdminAudit(context.Context, int) ([]domain.AdminAuditEntry, error) {
	return nil, nil
}

func TestSQLiteReadAdminServiceRollsBackRuntimePolicyWhenAuditFails(t *testing.T) {
	auditErr := errors.New("audit unavailable")
	store := &failingRuntimePolicyAuditStore{
		policy:    domain.RuntimePolicy{CooldownSeconds: 30, DailyBudgetWei: big.NewInt(100)},
		appendErr: auditErr,
	}
	svc := NewSQLiteReadAdminService(store)

	_, err := svc.SetRuntimePolicy(context.Background(), SetRuntimePolicyRequest{CooldownSeconds: 90, DailyBudgetWei: "200"}, "operator")
	if !errors.Is(err, auditErr) {
		t.Fatalf("SetRuntimePolicy error = %v, want audit error", err)
	}
	if store.policy.CooldownSeconds != 30 || store.policy.DailyBudgetWei == nil || store.policy.DailyBudgetWei.String() != "100" {
		t.Fatalf("policy after failed audited set = %#v, want original", store.policy)
	}
}

func TestSQLiteReadAdminServiceRollsBackRuntimePolicyClearWhenAuditFails(t *testing.T) {
	auditErr := errors.New("audit unavailable")
	store := &failingRuntimePolicyAuditStore{
		policy:    domain.RuntimePolicy{CooldownSeconds: 30, TokenDailyBudgetWei: map[string]*big.Int{"native": big.NewInt(100)}},
		appendErr: auditErr,
	}
	svc := NewSQLiteReadAdminService(store)

	err := svc.ClearRuntimePolicy(context.Background(), "operator")
	if !errors.Is(err, auditErr) {
		t.Fatalf("ClearRuntimePolicy error = %v, want audit error", err)
	}
	if store.policy.CooldownSeconds != 30 || store.policy.TokenDailyBudgetWei["native"] == nil || store.policy.TokenDailyBudgetWei["native"].String() != "100" {
		t.Fatalf("policy after failed audited clear = %#v, want original", store.policy)
	}
}

func TestInMemoryAdminServiceRejectsInvalidRuntimePolicyWithDedicatedError(t *testing.T) {
	svc := NewInMemoryAdminService()

	_, err := svc.SetRuntimePolicy(context.Background(), SetRuntimePolicyRequest{CooldownSeconds: -1}, "operator")
	if !errors.Is(err, ErrInvalidRuntimePolicy) {
		t.Fatalf("SetRuntimePolicy error = %v, want ErrInvalidRuntimePolicy", err)
	}
	if errors.Is(err, ErrInvalidMode) {
		t.Fatalf("SetRuntimePolicy error = %v, must not be ErrInvalidMode", err)
	}
}

func TestInMemoryAdminServicePersistsRuntimePolicyView(t *testing.T) {
	svc := NewInMemoryAdminService()

	initial, err := svc.RuntimePolicy(context.Background())
	if err != nil {
		t.Fatalf("initial runtime policy: %v", err)
	}
	if initial.Source != "env" {
		t.Fatalf("initial source = %q, want env", initial.Source)
	}

	updated, err := svc.SetRuntimePolicy(context.Background(), SetRuntimePolicyRequest{
		CooldownSeconds:     45,
		RateLimitIPPerHour:  7,
		RateLimitAddrPerDay: 3,
		DailyBudgetWei:      "1000",
		TokenDailyBudgetWei: map[string]string{"native": "250"},
	}, "operator")
	if err != nil {
		t.Fatalf("set runtime policy: %v", err)
	}
	if updated.Source != "runtime" || updated.CooldownSeconds != 45 || updated.DailyBudgetWei != "1000" || updated.TokenDailyBudgetWei["native"] != "250" {
		t.Fatalf("updated runtime policy = %#v", updated)
	}

	got, err := svc.RuntimePolicy(context.Background())
	if err != nil {
		t.Fatalf("get runtime policy: %v", err)
	}
	if got.Source != "runtime" || got.RateLimitIPPerHour != 7 || got.RateLimitAddrPerDay != 3 || got.TokenDailyBudgetWei["native"] != "250" {
		t.Fatalf("persisted runtime policy = %#v", got)
	}

	if err := svc.ClearRuntimePolicy(context.Background(), "operator"); err != nil {
		t.Fatalf("clear runtime policy: %v", err)
	}
	cleared, err := svc.RuntimePolicy(context.Background())
	if err != nil {
		t.Fatalf("cleared runtime policy: %v", err)
	}
	if cleared.Source != "env" || cleared.CooldownSeconds != 0 || cleared.DailyBudgetWei != "" || len(cleared.TokenDailyBudgetWei) != 0 {
		t.Fatalf("cleared runtime policy = %#v", cleared)
	}
}

func TestInMemoryAdminServicePersistsCampaignControls(t *testing.T) {
	svc := NewInMemoryAdminService()

	created, err := svc.CreateCampaign(context.Background(), CampaignRequest{
		ID:        "camp1",
		Name:      "Launch",
		TokenID:   "native",
		Scope:     "invite",
		BudgetWei: "1000",
		Enabled:   true,
	}, "operator")
	if err != nil {
		t.Fatalf("CreateCampaign error = %v", err)
	}
	if created.ID != "camp1" || created.BudgetWei == nil || created.BudgetWei.String() != "1000" {
		t.Fatalf("created campaign = %#v", created)
	}

	campaigns, err := svc.ListCampaigns(context.Background(), 100, 0)
	if err != nil {
		t.Fatalf("ListCampaigns error = %v", err)
	}
	if len(campaigns) != 1 || campaigns[0].ID != "camp1" || !campaigns[0].Enabled {
		t.Fatalf("campaigns = %#v", campaigns)
	}

	code, err := svc.CreateInvitationCode(context.Background(), InvitationCodeRequest{Code: "CODE1", CampaignID: "camp1", MaxUses: 1, Enabled: true}, "operator")
	if err != nil {
		t.Fatalf("CreateInvitationCode error = %v", err)
	}
	if code.Code != "CODE1" || code.CampaignID != "camp1" || !code.Enabled {
		t.Fatalf("invitation code = %#v", code)
	}

	if err := svc.AddAllowlistEntry(context.Background(), AllowlistAddRequest{CampaignID: "camp1", Address: "0x1111111111111111111111111111111111111111", Note: "seed"}, "operator"); err != nil {
		t.Fatalf("AddAllowlistEntry error = %v", err)
	}

	if err := svc.DisableCampaign(context.Background(), "camp1", "operator"); err != nil {
		t.Fatalf("DisableCampaign error = %v", err)
	}
	campaigns, err = svc.ListCampaigns(context.Background(), 100, 0)
	if err != nil {
		t.Fatalf("ListCampaigns after disable error = %v", err)
	}
	if len(campaigns) != 1 || campaigns[0].Enabled {
		t.Fatalf("campaigns after disable = %#v", campaigns)
	}
}

func TestInMemoryAdminServiceRejectsMissingCampaignReferences(t *testing.T) {
	svc := NewInMemoryAdminService()

	if _, err := svc.CreateInvitationCode(context.Background(), InvitationCodeRequest{Code: "CODE1", CampaignID: "missing", MaxUses: 1, Enabled: true}, "operator"); !errors.Is(err, ErrInvalidCampaign) {
		t.Fatalf("CreateInvitationCode error = %v, want ErrInvalidCampaign", err)
	}
	if err := svc.AddAllowlistEntry(context.Background(), AllowlistAddRequest{CampaignID: "missing", Address: "0x1111111111111111111111111111111111111111"}, "operator"); !errors.Is(err, ErrInvalidCampaign) {
		t.Fatalf("AddAllowlistEntry error = %v, want ErrInvalidCampaign", err)
	}
	if err := svc.DisableCampaign(context.Background(), "missing", "operator"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("DisableCampaign error = %v, want ErrNotFound", err)
	}
}

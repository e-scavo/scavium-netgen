package admin

import (
	"context"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"scavium-netgen/cmd/scavium-faucet/internal/abuse"
	"scavium-netgen/cmd/scavium-faucet/internal/domain"

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

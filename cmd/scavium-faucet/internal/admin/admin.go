package admin

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"scavium-netgen/cmd/scavium-faucet/internal/abuse"
	"scavium-netgen/cmd/scavium-faucet/internal/domain"
)

// --- Role ---------------------------------------------------------------

// Role represents an admin permission level.
// For MVP a single bearer token grants the admin role.
// Multi-role enforcement is reserved for a later step.
type Role string

const (
	RoleAdmin    Role = "admin"
	RoleOperator Role = "operator"
	RoleViewer   Role = "viewer"
)

// --- AuditLog -----------------------------------------------------------

// AuditEntry records one admin action.
type AuditEntry struct {
	Action    string `json:"action"`
	Actor     string `json:"actor"`
	Target    string `json:"target"`
	Detail    string `json:"detail,omitempty"`
	CreatedAt string `json:"created_at"`
}

// AuditLog is a thread-safe in-memory ring buffer of AuditEntry values.
type AuditLog struct {
	mu      sync.Mutex
	entries []AuditEntry
	maxSize int
}

// NewAuditLog creates an AuditLog retaining up to maxSize entries.
// maxSize is clamped to 1 if non-positive.
func NewAuditLog(maxSize int) *AuditLog {
	if maxSize <= 0 {
		maxSize = 500
	}
	return &AuditLog{maxSize: maxSize}
}

// Append records a new entry, evicting the oldest when at capacity.
func (al *AuditLog) Append(e AuditEntry) {
	al.mu.Lock()
	defer al.mu.Unlock()
	al.entries = append(al.entries, e)
	if len(al.entries) > al.maxSize {
		al.entries = al.entries[len(al.entries)-al.maxSize:]
	}
}

// Recent returns up to n recent entries in chronological order.
func (al *AuditLog) Recent(n int) []AuditEntry {
	al.mu.Lock()
	defer al.mu.Unlock()
	if n <= 0 || len(al.entries) == 0 {
		return nil
	}
	start := len(al.entries) - n
	if start < 0 {
		start = 0
	}
	out := make([]AuditEntry, len(al.entries)-start)
	copy(out, al.entries[start:])
	return out
}

// --- Response types -----------------------------------------------------

// DashboardResponse summarises the faucet admin dashboard.
type DashboardResponse struct {
	Mode          string         `json:"mode"`
	ClaimCounts   map[string]int `json:"claim_counts"`
	BlocklistSize int            `json:"blocklist_size"`
	UpdatedAt     string         `json:"updated_at"`
}

// QueueItemResponse is a compact, admin-safe queue row.
type QueueItemResponse struct {
	ID            string `json:"id"`
	Status        string `json:"status"`
	TokenID       string `json:"token_id,omitempty"`
	TokenSymbol   string `json:"token_symbol,omitempty"`
	RetryCount    int    `json:"retry_count"`
	NextAttemptAt string `json:"next_attempt_at,omitempty"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

// QueueResponse summarises operational queue visibility for the admin plane.
type QueueResponse struct {
	Counts    map[string]int      `json:"counts"`
	Ready     int                 `json:"ready"`
	Delayed   int                 `json:"delayed"`
	InFlight  int                 `json:"in_flight"`
	PendingTx int                 `json:"pending_tx"`
	Terminal  int                 `json:"terminal"`
	Items     []QueueItemResponse `json:"items"`
	UpdatedAt string              `json:"updated_at"`
}

// BlocklistAddRequest is the body for POST /api/v1/admin/blocklist.
type BlocklistAddRequest struct {
	KeyType string `json:"key_type"`
	Value   string `json:"value"`
	Reason  string `json:"reason"`
}

// SetModeRequest is the body for POST /api/v1/admin/faucet/mode.
type SetModeRequest struct {
	Mode string `json:"mode"`
}

// --- Sentinel errors ----------------------------------------------------

var (
	ErrNotFound       = errors.New("admin: not found")
	ErrNotRetryable   = errors.New("admin: claim is not in a retryable state")
	ErrNotCancellable = errors.New("admin: claim cannot be cancelled in its current state")
)

// --- AdminService interface ---------------------------------------------

// AdminService provides admin-plane operations on the faucet.
// The InMemoryAdminService is the default implementation (suitable for tests
// and standalone mode).  A SQLite-backed implementation is planned for a
// later step.
type AdminService interface {
	Dashboard(ctx context.Context) (DashboardResponse, error)
	Queue(ctx context.Context, limit int) (QueueResponse, error)
	ListClaims(ctx context.Context, limit, offset int) ([]domain.Claim, error)
	GetClaim(ctx context.Context, id string) (domain.Claim, bool, error)
	// SetMode changes the operational mode ("active", "paused", "maintenance").
	SetMode(ctx context.Context, mode, actor string) error
	// RetryClaim moves a failed or rejected claim back to queued.
	RetryClaim(ctx context.Context, id, actor string) error
	// CancelClaim rejects a claim that has not yet been sent.
	CancelClaim(ctx context.Context, id, actor string) error
	BlocklistList(ctx context.Context) ([]abuse.BlocklistEntry, error)
	BlocklistAdd(ctx context.Context, kt abuse.KeyType, value, reason, actor string) error
	BlocklistRemove(ctx context.Context, kt abuse.KeyType, value, actor string) error
	RecentAuditLog(ctx context.Context, limit int) ([]AuditEntry, error)
}

// --- InMemoryAdminService -----------------------------------------------

// InMemoryAdminService is a standalone in-memory implementation of AdminService.
type InMemoryAdminService struct {
	mu        sync.RWMutex
	mode      string
	claims    map[string]domain.Claim
	blocklist *abuse.Blocklist
	auditLog  *AuditLog
	now       func() time.Time
}

// NewInMemoryAdminService creates an in-memory admin service in "active" mode.
func NewInMemoryAdminService() *InMemoryAdminService {
	return &InMemoryAdminService{
		mode:      "active",
		claims:    make(map[string]domain.Claim),
		blocklist: abuse.NewBlocklist(),
		auditLog:  NewAuditLog(500),
		now:       time.Now,
	}
}

// withClock replaces the time source.  Useful for testing.
func (s *InMemoryAdminService) withClock(now func() time.Time) *InMemoryAdminService {
	s.now = now
	return s
}

// AddClaim inserts a claim into the in-memory store.  Used in tests and
// to seed the service with existing claim data.
func (s *InMemoryAdminService) AddClaim(c domain.Claim) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.claims[c.ID] = c
}

func (s *InMemoryAdminService) Dashboard(_ context.Context) (DashboardResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	counts := make(map[string]int)
	for _, c := range s.claims {
		counts[string(c.Status)]++
	}
	return DashboardResponse{
		Mode:          s.mode,
		ClaimCounts:   counts,
		BlocklistSize: len(s.blocklist.Entries()),
		UpdatedAt:     s.now().UTC().Format(time.RFC3339),
	}, nil
}

func (s *InMemoryAdminService) Queue(_ context.Context, limit int) (QueueResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 {
		limit = 50
	}
	now := s.now().UTC()
	claims := make([]domain.Claim, 0, len(s.claims))
	for _, c := range s.claims {
		claims = append(claims, c)
	}
	sort.SliceStable(claims, func(i, j int) bool {
		if claims[i].UpdatedAt.Equal(claims[j].UpdatedAt) {
			return claims[i].ID < claims[j].ID
		}
		return claims[i].UpdatedAt.After(claims[j].UpdatedAt)
	})

	resp := QueueResponse{
		Counts:    make(map[string]int),
		UpdatedAt: now.Format(time.RFC3339),
	}
	for _, c := range claims {
		resp.Counts[string(c.Status)]++
		switch c.Status {
		case domain.ClaimStatusQueued:
			if c.NextAttemptAt != nil && c.NextAttemptAt.After(now) {
				resp.Delayed++
			} else {
				resp.Ready++
			}
		case domain.ClaimStatusSending:
			resp.InFlight++
		case domain.ClaimStatusSent:
			resp.PendingTx++
		case domain.ClaimStatusConfirmed, domain.ClaimStatusFailed, domain.ClaimStatusRejected:
			resp.Terminal++
		}
	}

	for _, c := range claims {
		if len(resp.Items) >= limit {
			break
		}
		if c.Status != domain.ClaimStatusQueued && c.Status != domain.ClaimStatusSending && c.Status != domain.ClaimStatusSent && c.Status != domain.ClaimStatusFailed {
			continue
		}
		resp.Items = append(resp.Items, queueItemResponse(c))
	}
	return resp, nil
}

func queueItemResponse(c domain.Claim) QueueItemResponse {
	item := QueueItemResponse{
		ID:          c.ID,
		Status:      string(c.Status),
		TokenID:     c.TokenID,
		TokenSymbol: c.TokenSymbol,
		RetryCount:  c.RetryCount,
		CreatedAt:   c.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   c.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if c.NextAttemptAt != nil {
		item.NextAttemptAt = c.NextAttemptAt.UTC().Format(time.RFC3339)
	}
	return item
}

func (s *InMemoryAdminService) ListClaims(_ context.Context, limit, offset int) ([]domain.Claim, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	all := make([]domain.Claim, 0, len(s.claims))
	for _, c := range s.claims {
		all = append(all, c)
	}
	if offset >= len(all) {
		return nil, nil
	}
	end := offset + limit
	if limit <= 0 || end > len(all) {
		end = len(all)
	}
	return all[offset:end], nil
}

func (s *InMemoryAdminService) GetClaim(_ context.Context, id string) (domain.Claim, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.claims[id]
	return c, ok, nil
}

func (s *InMemoryAdminService) SetMode(_ context.Context, mode, actor string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mode = mode
	s.auditLog.Append(AuditEntry{
		Action:    "set_mode",
		Actor:     actor,
		Target:    "faucet",
		Detail:    mode,
		CreatedAt: s.now().UTC().Format(time.RFC3339),
	})
	return nil
}

func (s *InMemoryAdminService) RetryClaim(_ context.Context, id, actor string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.claims[id]
	if !ok {
		return ErrNotFound
	}
	if c.Status != domain.ClaimStatusFailed && c.Status != domain.ClaimStatusRejected {
		return ErrNotRetryable
	}
	c.Status = domain.ClaimStatusQueued
	c.Reason = ""
	c.UpdatedAt = s.now().UTC()
	s.claims[id] = c
	s.auditLog.Append(AuditEntry{
		Action:    "retry_claim",
		Actor:     actor,
		Target:    id,
		CreatedAt: s.now().UTC().Format(time.RFC3339),
	})
	return nil
}

func (s *InMemoryAdminService) CancelClaim(_ context.Context, id, actor string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.claims[id]
	if !ok {
		return ErrNotFound
	}
	if c.Status == domain.ClaimStatusSent ||
		c.Status == domain.ClaimStatusConfirmed ||
		c.Status == domain.ClaimStatusSending {
		return ErrNotCancellable
	}
	c.Status = domain.ClaimStatusRejected
	c.Reason = "cancelled by admin"
	c.UpdatedAt = s.now().UTC()
	s.claims[id] = c
	s.auditLog.Append(AuditEntry{
		Action:    "cancel_claim",
		Actor:     actor,
		Target:    id,
		CreatedAt: s.now().UTC().Format(time.RFC3339),
	})
	return nil
}

func (s *InMemoryAdminService) BlocklistList(_ context.Context) ([]abuse.BlocklistEntry, error) {
	return s.blocklist.Entries(), nil
}

func (s *InMemoryAdminService) BlocklistAdd(_ context.Context, kt abuse.KeyType, value, reason, actor string) error {
	s.blocklist.Block(kt, value, reason)
	s.auditLog.Append(AuditEntry{
		Action:    "blocklist_add",
		Actor:     actor,
		Target:    string(kt) + ":" + value,
		Detail:    reason,
		CreatedAt: s.now().UTC().Format(time.RFC3339),
	})
	return nil
}

func (s *InMemoryAdminService) BlocklistRemove(_ context.Context, kt abuse.KeyType, value, actor string) error {
	s.blocklist.Unblock(kt, value)
	s.auditLog.Append(AuditEntry{
		Action:    "blocklist_remove",
		Actor:     actor,
		Target:    string(kt) + ":" + value,
		CreatedAt: s.now().UTC().Format(time.RFC3339),
	})
	return nil
}

func (s *InMemoryAdminService) RecentAuditLog(_ context.Context, limit int) ([]AuditEntry, error) {
	return s.auditLog.Recent(limit), nil
}

// --- TokenAuthMiddleware ------------------------------------------------

// TokenAuthMiddleware wraps next with Bearer-token authentication for the
// admin API.
//   - If token is empty, all requests respond 503 (admin API not configured).
//   - Comparison is constant-time to prevent timing side-channels.
//   - Never log the token value.
func TokenAuthMiddleware(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"code":"admin_disabled","message":"admin api is not configured"}`))
			return
		}
		bearer := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		if subtle.ConstantTimeCompare([]byte(bearer), []byte(token)) != 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"code":"unauthorized","message":"invalid or missing admin token"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

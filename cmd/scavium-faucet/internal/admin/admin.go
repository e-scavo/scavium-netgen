package admin

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"scavium-netgen/cmd/scavium-faucet/internal/abuse"
	"scavium-netgen/cmd/scavium-faucet/internal/domain"

	"github.com/ethereum/go-ethereum/common"
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

// RuntimePolicyResponse is the admin-safe view of mutable runtime policy.
type RuntimePolicyResponse struct {
	CooldownSeconds     int               `json:"cooldown_seconds"`
	RateLimitIPPerHour  int               `json:"rate_limit_ip_per_hour"`
	RateLimitAddrPerDay int               `json:"rate_limit_addr_per_day"`
	DailyBudgetWei      string            `json:"daily_budget_wei,omitempty"`
	TokenDailyBudgetWei map[string]string `json:"token_daily_budget_wei,omitempty"`
	Source              string            `json:"source"`
}

// SetRuntimePolicyRequest is the body for PUT /api/v1/admin/policy.
type SetRuntimePolicyRequest struct {
	CooldownSeconds     int               `json:"cooldown_seconds"`
	RateLimitIPPerHour  int               `json:"rate_limit_ip_per_hour"`
	RateLimitAddrPerDay int               `json:"rate_limit_addr_per_day"`
	DailyBudgetWei      string            `json:"daily_budget_wei"`
	TokenDailyBudgetWei map[string]string `json:"token_daily_budget_wei"`
}

// QueueControlRequest is the body for POST /api/v1/admin/queue/{retry,cancel}.
type QueueControlRequest struct {
	ID string `json:"id"`
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

// CampaignRequest is the admin body for creating campaigns.
type CampaignRequest struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	TokenID   string `json:"token_id"`
	Scope     string `json:"scope"`
	BudgetWei string `json:"budget_wei"`
	Enabled   bool   `json:"enabled"`
	StartsAt  string `json:"starts_at"`
	EndsAt    string `json:"ends_at"`
}

// InvitationCodeRequest is the admin body for creating invitation codes.
type InvitationCodeRequest struct {
	Code       string `json:"code"`
	CampaignID string `json:"campaign_id"`
	MaxUses    int    `json:"max_uses"`
	Enabled    bool   `json:"enabled"`
}

// AllowlistAddRequest is the admin body for adding one campaign allowlist address.
type AllowlistAddRequest struct {
	CampaignID string `json:"campaign_id"`
	Address    string `json:"address"`
	Note       string `json:"note"`
}

// --- Sentinel errors ----------------------------------------------------

var (
	ErrNotFound             = errors.New("admin: not found")
	ErrNotRetryable         = errors.New("admin: claim is not in a retryable state")
	ErrNotCancellable       = errors.New("admin: claim cannot be cancelled in its current state")
	ErrInvalidMode          = errors.New("admin: invalid faucet mode")
	ErrInvalidRuntimePolicy = errors.New("admin: invalid runtime policy")
	ErrInvalidCampaign      = errors.New("admin: invalid campaign")
)

// --- AdminService interface ---------------------------------------------

// AdminService provides admin-plane operations on the faucet.
// InMemoryAdminService remains available for tests and standalone use.
// The production wiring uses SQLiteReadAdminService, which reads and mutates
// the Phase 20 durable admin surfaces through SQLite where the backing store
// implements the corresponding read/control/audit/blocklist interfaces.
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
	RuntimePolicy(ctx context.Context) (RuntimePolicyResponse, error)
	SetRuntimePolicy(ctx context.Context, req SetRuntimePolicyRequest, actor string) (RuntimePolicyResponse, error)
	ClearRuntimePolicy(ctx context.Context, actor string) error
	CreateCampaign(ctx context.Context, req CampaignRequest, actor string) (domain.Campaign, error)
	ListCampaigns(ctx context.Context, limit, offset int) ([]domain.Campaign, error)
	DisableCampaign(ctx context.Context, id, actor string) error
	CreateInvitationCode(ctx context.Context, req InvitationCodeRequest, actor string) (domain.InvitationCode, error)
	AddAllowlistEntry(ctx context.Context, req AllowlistAddRequest, actor string) error
	RecentAuditLog(ctx context.Context, limit int) ([]AuditEntry, error)
}

// ReadStore provides persisted admin read-model access without requiring the
// broader durable admin control plane.
type ReadStore interface {
	GetClaim(ctx context.Context, id string) (domain.Claim, error)
	ListAdminClaims(ctx context.Context, limit, offset int) ([]domain.Claim, error)
	AdminClaimCounts(ctx context.Context) (map[string]int, error)
	AdminQueueCounts(ctx context.Context, now time.Time) (map[string]int, int, int, int, int, int, error)
	ListAdminQueueClaims(ctx context.Context, limit int) ([]domain.Claim, error)
}

// ControlStore provides persisted admin control transitions for claim retry
// and cancel operations.
type ControlStore interface {
	AdminRetryClaim(ctx context.Context, id string) (bool, error)
	AdminCancelClaim(ctx context.Context, id, reason string) (bool, error)
}

// AuditStore provides durable admin audit persistence for the admin plane.
type AuditStore interface {
	AppendAdminAudit(ctx context.Context, entry domain.AdminAuditEntry) error
	ListAdminAudit(ctx context.Context, limit int) ([]domain.AdminAuditEntry, error)
}

// RuntimePolicyStore provides durable runtime policy operations.
type CampaignStore interface {
	domain.CampaignStore
}

// campaignRollbackStore exposes best-effort rollback primitives for campaign
// admin writes when durable audit persistence fails after a mutation.
type campaignRollbackStore interface {
	DeleteCampaign(ctx context.Context, id string) error
	SetCampaignEnabled(ctx context.Context, id string, enabled bool) error
	DeleteInvitationCode(ctx context.Context, code string) error
	RemoveCampaignAllowlistEntry(ctx context.Context, campaignID string, address common.Address) error
}

type RuntimePolicyStore interface {
	GetRuntimePolicy(ctx context.Context) (domain.RuntimePolicy, error)
	SetRuntimePolicy(ctx context.Context, policy domain.RuntimePolicy) error
	ClearRuntimePolicy(ctx context.Context) error
}

// BlocklistStore provides durable admin blocklist operations.
type BlocklistStore interface {
	ListAdminBlocklist(ctx context.Context) ([]abuse.BlocklistEntry, error)
	AdminBlocklistAdd(ctx context.Context, keyType abuse.KeyType, value, reason string) error
	AdminBlocklistRemove(ctx context.Context, keyType abuse.KeyType, value string) error
}

// ValidMode reports whether mode is a supported faucet operational mode.
func ValidMode(mode string) bool {
	switch mode {
	case "active", "paused", "maintenance":
		return true
	default:
		return false
	}
}

// --- InMemoryAdminService -----------------------------------------------

// ModeController applies operational faucet mode changes to the live runtime.
type ModeController interface {
	SetFaucetMode(mode string)
}

// InMemoryAdminService is a standalone in-memory implementation of AdminService.
type InMemoryAdminService struct {
	mu            sync.RWMutex
	mode          string
	claims        map[string]domain.Claim
	blocklist     *abuse.Blocklist
	auditLog      *AuditLog
	modeCtl       ModeController
	runtimePolicy domain.RuntimePolicy
	campaigns     map[string]domain.Campaign
	invitations   map[string]domain.InvitationCode
	allowlist     map[string]map[string]domain.CampaignAllowlistEntry
	now           func() time.Time
}

// SQLiteReadAdminService uses SQLite-backed admin read/control/audit/blocklist
// interfaces when available, while retaining an in-memory fallback for tests
// and configurations that only provide a partial admin store.
type SQLiteReadAdminService struct {
	reads    ReadStore
	fallback *InMemoryAdminService
	now      func() time.Time
}

// NewInMemoryAdminService creates an in-memory admin service in "active" mode.
func NewInMemoryAdminService() *InMemoryAdminService {
	return &InMemoryAdminService{
		mode:        "active",
		claims:      make(map[string]domain.Claim),
		blocklist:   abuse.NewBlocklist(),
		auditLog:    NewAuditLog(500),
		campaigns:   make(map[string]domain.Campaign),
		invitations: make(map[string]domain.InvitationCode),
		allowlist:   make(map[string]map[string]domain.CampaignAllowlistEntry),
		now:         time.Now,
	}
}

// NewSQLiteReadAdminService creates a hybrid admin service whose Phase 20
// read/control/audit/blocklist surfaces reflect persisted SQLite state whenever
// the provided store implements those capabilities.
func NewSQLiteReadAdminService(reads ReadStore) *SQLiteReadAdminService {
	return &SQLiteReadAdminService{
		reads:    reads,
		fallback: NewInMemoryAdminService(),
		now:      time.Now,
	}
}

// withClock replaces the time source. Useful for testing.
func (s *InMemoryAdminService) withClock(now func() time.Time) *InMemoryAdminService {
	s.now = now
	return s
}

// withClock replaces the time source. Useful for testing.
func (s *SQLiteReadAdminService) withClock(now func() time.Time) *SQLiteReadAdminService {
	s.now = now
	s.fallback.withClock(now)
	return s
}

// SetModeController connects admin mode changes to the live faucet runtime.
func (s *InMemoryAdminService) SetModeController(controller ModeController) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.modeCtl = controller
}

// SetModeController connects admin mode changes to the live faucet runtime.
func (s *SQLiteReadAdminService) SetModeController(controller ModeController) {
	s.fallback.SetModeController(controller)
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
	mode = strings.TrimSpace(mode)
	if !ValidMode(mode) {
		return ErrInvalidMode
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.mode = mode
	if s.modeCtl != nil {
		s.modeCtl.SetFaucetMode(mode)
	}
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
		Target:    "blocklist",
		Detail:    string(kt),
		CreatedAt: s.now().UTC().Format(time.RFC3339),
	})
	return nil
}

func (s *InMemoryAdminService) BlocklistRemove(_ context.Context, kt abuse.KeyType, value, actor string) error {
	s.blocklist.Unblock(kt, value)
	s.auditLog.Append(AuditEntry{
		Action:    "blocklist_remove",
		Actor:     actor,
		Target:    "blocklist",
		Detail:    string(kt),
		CreatedAt: s.now().UTC().Format(time.RFC3339),
	})
	return nil
}

func (s *InMemoryAdminService) RecentAuditLog(_ context.Context, limit int) ([]AuditEntry, error) {
	return s.auditLog.Recent(limit), nil
}

func (s *SQLiteReadAdminService) Dashboard(ctx context.Context) (DashboardResponse, error) {
	base, err := s.fallback.Dashboard(ctx)
	if err != nil {
		return DashboardResponse{}, err
	}
	if s.reads == nil {
		return base, nil
	}
	counts, err := s.reads.AdminClaimCounts(ctx)
	if err != nil {
		return DashboardResponse{}, err
	}
	base.ClaimCounts = counts
	if blocklist, ok := s.reads.(BlocklistStore); ok {
		entries, err := blocklist.ListAdminBlocklist(ctx)
		if err != nil {
			return DashboardResponse{}, err
		}
		base.BlocklistSize = len(entries)
	}
	base.UpdatedAt = s.now().UTC().Format(time.RFC3339)
	return base, nil
}

func (s *SQLiteReadAdminService) Queue(ctx context.Context, limit int) (QueueResponse, error) {
	if s.reads == nil {
		return s.fallback.Queue(ctx, limit)
	}
	now := s.now().UTC()
	counts, ready, delayed, inFlight, pendingTx, terminal, err := s.reads.AdminQueueCounts(ctx, now)
	if err != nil {
		return QueueResponse{}, err
	}
	items, err := s.reads.ListAdminQueueClaims(ctx, limit)
	if err != nil {
		return QueueResponse{}, err
	}
	resp := QueueResponse{
		Counts:    counts,
		Ready:     ready,
		Delayed:   delayed,
		InFlight:  inFlight,
		PendingTx: pendingTx,
		Terminal:  terminal,
		UpdatedAt: now.Format(time.RFC3339),
	}
	for _, claim := range items {
		resp.Items = append(resp.Items, queueItemResponse(claim))
	}
	return resp, nil
}

func (s *SQLiteReadAdminService) ListClaims(ctx context.Context, limit, offset int) ([]domain.Claim, error) {
	if s.reads == nil {
		return s.fallback.ListClaims(ctx, limit, offset)
	}
	return s.reads.ListAdminClaims(ctx, limit, offset)
}

func (s *SQLiteReadAdminService) GetClaim(ctx context.Context, id string) (domain.Claim, bool, error) {
	if s.reads == nil {
		return s.fallback.GetClaim(ctx, id)
	}
	claim, err := s.reads.GetClaim(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.Claim{}, false, nil
		}
		return domain.Claim{}, false, err
	}
	return claim, true, nil
}

func (s *SQLiteReadAdminService) SetMode(ctx context.Context, mode, actor string) error {
	if err := s.fallback.SetMode(ctx, mode, actor); err != nil {
		return err
	}
	if s.reads == nil {
		return nil
	}
	audits, ok := s.reads.(AuditStore)
	if !ok {
		return nil
	}
	return audits.AppendAdminAudit(ctx, domain.AdminAuditEntry{
		Action:    "set_mode",
		Actor:     actor,
		Target:    "faucet",
		Detail:    mode,
		CreatedAt: s.now().UTC().Format(time.RFC3339),
	})
}

func (s *SQLiteReadAdminService) RetryClaim(ctx context.Context, id, actor string) error {
	if s.reads == nil {
		return s.fallback.RetryClaim(ctx, id, actor)
	}
	claim, err := s.reads.GetClaim(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	if claim.Status != domain.ClaimStatusFailed && claim.Status != domain.ClaimStatusRejected {
		return ErrNotRetryable
	}
	control, ok := s.reads.(ControlStore)
	if !ok {
		return s.fallback.RetryClaim(ctx, id, actor)
	}
	updated, err := control.AdminRetryClaim(ctx, id)
	if err != nil {
		return err
	}
	if !updated {
		latest, readErr := s.reads.GetClaim(ctx, id)
		if readErr != nil {
			if errors.Is(readErr, domain.ErrNotFound) {
				return ErrNotFound
			}
			return readErr
		}
		if latest.Status != domain.ClaimStatusFailed && latest.Status != domain.ClaimStatusRejected {
			return ErrNotRetryable
		}
		return ErrNotFound
	}
	if err := s.appendAudit(ctx, AuditEntry{
		Action:    "retry_claim",
		Actor:     actor,
		Target:    id,
		CreatedAt: s.now().UTC().Format(time.RFC3339),
	}); err != nil {
		return err
	}
	return nil
}

func (s *SQLiteReadAdminService) CancelClaim(ctx context.Context, id, actor string) error {
	if s.reads == nil {
		return s.fallback.CancelClaim(ctx, id, actor)
	}
	claim, err := s.reads.GetClaim(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	if claim.Status == domain.ClaimStatusSent || claim.Status == domain.ClaimStatusConfirmed || claim.Status == domain.ClaimStatusSending {
		return ErrNotCancellable
	}
	control, ok := s.reads.(ControlStore)
	if !ok {
		return s.fallback.CancelClaim(ctx, id, actor)
	}
	updated, err := control.AdminCancelClaim(ctx, id, "cancelled by admin")
	if err != nil {
		return err
	}
	if !updated {
		latest, readErr := s.reads.GetClaim(ctx, id)
		if readErr != nil {
			if errors.Is(readErr, domain.ErrNotFound) {
				return ErrNotFound
			}
			return readErr
		}
		if latest.Status == domain.ClaimStatusSent || latest.Status == domain.ClaimStatusConfirmed || latest.Status == domain.ClaimStatusSending {
			return ErrNotCancellable
		}
		return ErrNotFound
	}
	if err := s.appendAudit(ctx, AuditEntry{
		Action:    "cancel_claim",
		Actor:     actor,
		Target:    id,
		CreatedAt: s.now().UTC().Format(time.RFC3339),
	}); err != nil {
		return err
	}
	return nil
}

func (s *SQLiteReadAdminService) BlocklistList(ctx context.Context) ([]abuse.BlocklistEntry, error) {
	if s.reads != nil {
		if blocklist, ok := s.reads.(BlocklistStore); ok {
			return blocklist.ListAdminBlocklist(ctx)
		}
	}
	return s.fallback.BlocklistList(ctx)
}

func (s *SQLiteReadAdminService) BlocklistAdd(ctx context.Context, kt abuse.KeyType, value, reason, actor string) error {
	if s.reads != nil {
		if blocklist, ok := s.reads.(BlocklistStore); ok {
			if err := blocklist.AdminBlocklistAdd(ctx, kt, value, reason); err != nil {
				return err
			}
			return s.appendAudit(ctx, AuditEntry{
				Action:    "blocklist_add",
				Actor:     actor,
				Target:    "blocklist",
				Detail:    string(kt),
				CreatedAt: s.now().UTC().Format(time.RFC3339),
			})
		}
	}
	return s.fallback.BlocklistAdd(ctx, kt, value, reason, actor)
}

func (s *SQLiteReadAdminService) BlocklistRemove(ctx context.Context, kt abuse.KeyType, value, actor string) error {
	if s.reads != nil {
		if blocklist, ok := s.reads.(BlocklistStore); ok {
			if err := blocklist.AdminBlocklistRemove(ctx, kt, value); err != nil {
				return err
			}
			return s.appendAudit(ctx, AuditEntry{
				Action:    "blocklist_remove",
				Actor:     actor,
				Target:    "blocklist",
				Detail:    string(kt),
				CreatedAt: s.now().UTC().Format(time.RFC3339),
			})
		}
	}
	return s.fallback.BlocklistRemove(ctx, kt, value, actor)
}

func (s *SQLiteReadAdminService) RecentAuditLog(ctx context.Context, limit int) ([]AuditEntry, error) {
	if s.reads != nil {
		if audits, ok := s.reads.(AuditStore); ok {
			entries, err := audits.ListAdminAudit(ctx, limit)
			if err != nil {
				return nil, err
			}
			out := make([]AuditEntry, 0, len(entries))
			for _, entry := range entries {
				out = append(out, AuditEntry(entry))
			}
			return out, nil
		}
	}
	return s.fallback.RecentAuditLog(ctx, limit)
}

func (s *SQLiteReadAdminService) appendAudit(ctx context.Context, entry AuditEntry) error {
	if s.reads != nil {
		if audits, ok := s.reads.(AuditStore); ok {
			if err := audits.AppendAdminAudit(ctx, domain.AdminAuditEntry(entry)); err != nil {
				return err
			}
		}
	}
	s.fallback.auditLog.Append(entry)
	return nil
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

func (s *InMemoryAdminService) RuntimePolicy(context.Context) (RuntimePolicyResponse, error) {
	s.mu.RLock()
	policy := domain.CopyRuntimePolicy(s.runtimePolicy)
	s.mu.RUnlock()
	return runtimePolicyResponse(policy, runtimePolicySource(policy)), nil
}

func (s *InMemoryAdminService) SetRuntimePolicy(_ context.Context, req SetRuntimePolicyRequest, actor string) (RuntimePolicyResponse, error) {
	policy, err := runtimePolicyFromRequest(req)
	if err != nil {
		return RuntimePolicyResponse{}, err
	}
	s.mu.Lock()
	s.runtimePolicy = domain.CopyRuntimePolicy(policy)
	s.mu.Unlock()
	resp := runtimePolicyResponse(policy, runtimePolicySource(policy))
	s.auditLog.Append(AuditEntry{Action: "set_runtime_policy", Actor: actor, Target: "policy", Detail: runtimePolicySummary(resp), CreatedAt: s.now().UTC().Format(time.RFC3339)})
	return resp, nil
}

func (s *InMemoryAdminService) ClearRuntimePolicy(_ context.Context, actor string) error {
	s.mu.Lock()
	s.runtimePolicy = domain.RuntimePolicy{}
	s.mu.Unlock()
	s.auditLog.Append(AuditEntry{Action: "clear_runtime_policy", Actor: actor, Target: "policy", CreatedAt: s.now().UTC().Format(time.RFC3339)})
	return nil
}

func (s *SQLiteReadAdminService) RuntimePolicy(ctx context.Context) (RuntimePolicyResponse, error) {
	store, ok := s.reads.(RuntimePolicyStore)
	if !ok {
		return s.fallback.RuntimePolicy(ctx)
	}
	policy, err := store.GetRuntimePolicy(ctx)
	if err != nil {
		return RuntimePolicyResponse{}, err
	}
	return runtimePolicyResponse(policy, runtimePolicySource(policy)), nil
}

func (s *SQLiteReadAdminService) SetRuntimePolicy(ctx context.Context, req SetRuntimePolicyRequest, actor string) (RuntimePolicyResponse, error) {
	store, ok := s.reads.(RuntimePolicyStore)
	if !ok {
		return s.fallback.SetRuntimePolicy(ctx, req, actor)
	}
	before, err := store.GetRuntimePolicy(ctx)
	if err != nil {
		return RuntimePolicyResponse{}, err
	}
	policy, err := runtimePolicyFromRequest(req)
	if err != nil {
		return RuntimePolicyResponse{}, err
	}
	if err := store.SetRuntimePolicy(ctx, policy); err != nil {
		return RuntimePolicyResponse{}, err
	}
	after := runtimePolicyResponse(policy, runtimePolicySource(policy))
	if err := s.appendAudit(ctx, AuditEntry{Action: "set_runtime_policy", Actor: actor, Target: "policy", Detail: runtimePolicyChangeSummary(runtimePolicyResponse(before, runtimePolicySource(before)), after), CreatedAt: s.now().UTC().Format(time.RFC3339)}); err != nil {
		_ = store.SetRuntimePolicy(ctx, before)
		return RuntimePolicyResponse{}, err
	}
	return after, nil
}

func (s *SQLiteReadAdminService) ClearRuntimePolicy(ctx context.Context, actor string) error {
	store, ok := s.reads.(RuntimePolicyStore)
	if !ok {
		return s.fallback.ClearRuntimePolicy(ctx, actor)
	}
	before, err := store.GetRuntimePolicy(ctx)
	if err != nil {
		return err
	}
	if err := store.ClearRuntimePolicy(ctx); err != nil {
		return err
	}
	if err := s.appendAudit(ctx, AuditEntry{Action: "clear_runtime_policy", Actor: actor, Target: "policy", Detail: runtimePolicySummary(runtimePolicyResponse(before, runtimePolicySource(before))), CreatedAt: s.now().UTC().Format(time.RFC3339)}); err != nil {
		_ = store.SetRuntimePolicy(ctx, before)
		return err
	}
	return nil
}

func runtimePolicyFromRequest(req SetRuntimePolicyRequest) (domain.RuntimePolicy, error) {
	if req.CooldownSeconds < 0 || req.RateLimitIPPerHour < 0 || req.RateLimitAddrPerDay < 0 {
		return domain.RuntimePolicy{}, ErrInvalidRuntimePolicy
	}
	policy := domain.RuntimePolicy{CooldownSeconds: req.CooldownSeconds, RateLimitIPPerHour: req.RateLimitIPPerHour, RateLimitAddrPerDay: req.RateLimitAddrPerDay}
	if strings.TrimSpace(req.DailyBudgetWei) != "" {
		v, ok := new(big.Int).SetString(strings.TrimSpace(req.DailyBudgetWei), 10)
		if !ok || v.Sign() < 0 {
			return domain.RuntimePolicy{}, ErrInvalidRuntimePolicy
		}
		policy.DailyBudgetWei = v
	}
	if len(req.TokenDailyBudgetWei) > 0 {
		policy.TokenDailyBudgetWei = make(map[string]*big.Int, len(req.TokenDailyBudgetWei))
		for tokenID, raw := range req.TokenDailyBudgetWei {
			tokenID = strings.TrimSpace(tokenID)
			if tokenID == "" {
				return domain.RuntimePolicy{}, ErrInvalidRuntimePolicy
			}
			v, ok := new(big.Int).SetString(strings.TrimSpace(raw), 10)
			if !ok || v.Sign() < 0 {
				return domain.RuntimePolicy{}, ErrInvalidRuntimePolicy
			}
			policy.TokenDailyBudgetWei[tokenID] = v
		}
	}
	return policy, nil
}

func runtimePolicySource(policy domain.RuntimePolicy) string {
	if policy.CooldownSeconds > 0 || policy.RateLimitIPPerHour > 0 || policy.RateLimitAddrPerDay > 0 || policy.DailyBudgetWei != nil || len(policy.TokenDailyBudgetWei) > 0 {
		return "runtime"
	}
	return "env"
}

func runtimePolicyResponse(policy domain.RuntimePolicy, source string) RuntimePolicyResponse {
	policy = domain.CopyRuntimePolicy(policy)
	resp := RuntimePolicyResponse{CooldownSeconds: policy.CooldownSeconds, RateLimitIPPerHour: policy.RateLimitIPPerHour, RateLimitAddrPerDay: policy.RateLimitAddrPerDay, Source: source}
	if policy.DailyBudgetWei != nil {
		resp.DailyBudgetWei = policy.DailyBudgetWei.String()
	}
	if len(policy.TokenDailyBudgetWei) > 0 {
		resp.TokenDailyBudgetWei = make(map[string]string, len(policy.TokenDailyBudgetWei))
		for tokenID, budget := range policy.TokenDailyBudgetWei {
			if budget != nil {
				resp.TokenDailyBudgetWei[tokenID] = budget.String()
			}
		}
	}
	return resp
}

func runtimePolicySummary(resp RuntimePolicyResponse) string {
	return fmt.Sprintf("cooldown=%d ip_per_hour=%d addr_per_day=%d daily_budget=%s token_budgets=%d", resp.CooldownSeconds, resp.RateLimitIPPerHour, resp.RateLimitAddrPerDay, resp.DailyBudgetWei, len(resp.TokenDailyBudgetWei))
}

func runtimePolicyChangeSummary(before, after RuntimePolicyResponse) string {
	return "before{" + runtimePolicySummary(before) + "} after{" + runtimePolicySummary(after) + "}"
}

func (s *InMemoryAdminService) CreateCampaign(_ context.Context, req CampaignRequest, actor string) (domain.Campaign, error) {
	campaign, err := campaignFromRequest(req)
	if err != nil {
		return domain.Campaign{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.campaigns[campaign.ID]; exists {
		return domain.Campaign{}, ErrInvalidCampaign
	}
	s.campaigns[campaign.ID] = copyCampaign(campaign)
	s.auditLog.Append(AuditEntry{Action: "campaign_create", Actor: actor, Target: campaign.ID, CreatedAt: s.now().UTC().Format(time.RFC3339)})
	return copyCampaign(campaign), nil
}

func (s *InMemoryAdminService) ListCampaigns(_ context.Context, limit, offset int) ([]domain.Campaign, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	campaigns := make([]domain.Campaign, 0, len(s.campaigns))
	for _, c := range s.campaigns {
		campaigns = append(campaigns, copyCampaign(c))
	}
	sort.SliceStable(campaigns, func(i, j int) bool {
		if campaigns[i].CreatedAt.Equal(campaigns[j].CreatedAt) {
			return campaigns[i].ID < campaigns[j].ID
		}
		return campaigns[i].CreatedAt.After(campaigns[j].CreatedAt)
	})
	if offset >= len(campaigns) {
		return []domain.Campaign{}, nil
	}
	end := offset + limit
	if end > len(campaigns) {
		end = len(campaigns)
	}
	return campaigns[offset:end], nil
}

func (s *InMemoryAdminService) DisableCampaign(_ context.Context, id, actor string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return ErrInvalidCampaign
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	campaign, ok := s.campaigns[id]
	if !ok {
		return ErrNotFound
	}
	campaign.Enabled = false
	campaign.UpdatedAt = s.now().UTC()
	s.campaigns[id] = campaign
	s.auditLog.Append(AuditEntry{Action: "campaign_disable", Actor: actor, Target: id, CreatedAt: s.now().UTC().Format(time.RFC3339)})
	return nil
}

func (s *InMemoryAdminService) CreateInvitationCode(_ context.Context, req InvitationCodeRequest, actor string) (domain.InvitationCode, error) {
	now := s.now().UTC()
	code := domain.InvitationCode{Code: strings.TrimSpace(req.Code), CampaignID: strings.TrimSpace(req.CampaignID), MaxUses: req.MaxUses, Enabled: req.Enabled, CreatedAt: now, UpdatedAt: now}
	if code.Code == "" || code.CampaignID == "" || code.MaxUses < 0 {
		return domain.InvitationCode{}, ErrInvalidCampaign
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.campaigns[code.CampaignID]; !ok {
		return domain.InvitationCode{}, ErrInvalidCampaign
	}
	if _, exists := s.invitations[code.Code]; exists {
		return domain.InvitationCode{}, ErrInvalidCampaign
	}
	s.invitations[code.Code] = code
	s.auditLog.Append(AuditEntry{Action: "invitation_create", Actor: actor, Target: code.Code, Detail: code.CampaignID, CreatedAt: now.Format(time.RFC3339)})
	return code, nil
}

func (s *InMemoryAdminService) AddAllowlistEntry(_ context.Context, req AllowlistAddRequest, actor string) error {
	addr, err := domain.ValidateAddress(req.Address)
	if err != nil {
		return ErrInvalidCampaign
	}
	entry := domain.CampaignAllowlistEntry{CampaignID: strings.TrimSpace(req.CampaignID), Address: addr, Note: strings.TrimSpace(req.Note), CreatedAt: s.now().UTC()}
	if entry.CampaignID == "" {
		return ErrInvalidCampaign
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.campaigns[entry.CampaignID]; !ok {
		return ErrInvalidCampaign
	}
	if s.allowlist[entry.CampaignID] == nil {
		s.allowlist[entry.CampaignID] = make(map[string]domain.CampaignAllowlistEntry)
	}
	s.allowlist[entry.CampaignID][entry.Address.Hex()] = entry
	s.auditLog.Append(AuditEntry{Action: "campaign_allowlist_add", Actor: actor, Target: entry.CampaignID, CreatedAt: s.now().UTC().Format(time.RFC3339)})
	return nil
}

func (s *SQLiteReadAdminService) CreateCampaign(ctx context.Context, req CampaignRequest, actor string) (domain.Campaign, error) {
	store, ok := s.reads.(CampaignStore)
	if !ok {
		return s.fallback.CreateCampaign(ctx, req, actor)
	}
	campaign, err := campaignFromRequest(req)
	if err != nil {
		return domain.Campaign{}, err
	}
	created, err := store.CreateCampaign(ctx, campaign)
	if err != nil {
		return domain.Campaign{}, err
	}
	if err := s.appendAudit(ctx, AuditEntry{Action: "campaign_create", Actor: actor, Target: created.ID, CreatedAt: s.now().UTC().Format(time.RFC3339)}); err != nil {
		if rollback, ok := store.(campaignRollbackStore); ok {
			_ = rollback.DeleteCampaign(ctx, created.ID)
		}
		return domain.Campaign{}, err
	}
	return created, nil
}

func (s *SQLiteReadAdminService) ListCampaigns(ctx context.Context, limit, offset int) ([]domain.Campaign, error) {
	store, ok := s.reads.(CampaignStore)
	if !ok {
		return s.fallback.ListCampaigns(ctx, limit, offset)
	}
	return store.ListCampaigns(ctx, limit, offset)
}

func (s *SQLiteReadAdminService) DisableCampaign(ctx context.Context, id, actor string) error {
	store, ok := s.reads.(CampaignStore)
	if !ok {
		return s.fallback.DisableCampaign(ctx, id, actor)
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return ErrInvalidCampaign
	}
	previous, getErr := store.GetCampaign(ctx, id)
	if getErr != nil {
		if errors.Is(getErr, domain.ErrNotFound) {
			return ErrNotFound
		}
		return getErr
	}
	if err := store.DisableCampaign(ctx, id); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	if err := s.appendAudit(ctx, AuditEntry{Action: "campaign_disable", Actor: actor, Target: id, CreatedAt: s.now().UTC().Format(time.RFC3339)}); err != nil {
		if rollback, ok := store.(campaignRollbackStore); ok {
			_ = rollback.SetCampaignEnabled(ctx, id, previous.Enabled)
		}
		return err
	}
	return nil
}

func (s *SQLiteReadAdminService) CreateInvitationCode(ctx context.Context, req InvitationCodeRequest, actor string) (domain.InvitationCode, error) {
	store, ok := s.reads.(CampaignStore)
	if !ok {
		return s.fallback.CreateInvitationCode(ctx, req, actor)
	}
	now := s.now().UTC()
	code := domain.InvitationCode{Code: strings.TrimSpace(req.Code), CampaignID: strings.TrimSpace(req.CampaignID), MaxUses: req.MaxUses, Enabled: req.Enabled, CreatedAt: now, UpdatedAt: now}
	if code.Code == "" || code.CampaignID == "" || code.MaxUses < 0 {
		return domain.InvitationCode{}, ErrInvalidCampaign
	}
	if _, err := store.GetCampaign(ctx, code.CampaignID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.InvitationCode{}, ErrInvalidCampaign
		}
		return domain.InvitationCode{}, err
	}
	created, err := store.CreateInvitationCode(ctx, code)
	if err != nil {
		return domain.InvitationCode{}, err
	}
	if err := s.appendAudit(ctx, AuditEntry{Action: "invitation_create", Actor: actor, Target: created.Code, Detail: created.CampaignID, CreatedAt: now.Format(time.RFC3339)}); err != nil {
		if rollback, ok := store.(campaignRollbackStore); ok {
			_ = rollback.DeleteInvitationCode(ctx, created.Code)
		}
		return domain.InvitationCode{}, err
	}
	return created, nil
}

func (s *SQLiteReadAdminService) AddAllowlistEntry(ctx context.Context, req AllowlistAddRequest, actor string) error {
	store, ok := s.reads.(CampaignStore)
	if !ok {
		return s.fallback.AddAllowlistEntry(ctx, req, actor)
	}
	addr, err := domain.ValidateAddress(req.Address)
	if err != nil {
		return ErrInvalidCampaign
	}
	entry := domain.CampaignAllowlistEntry{CampaignID: strings.TrimSpace(req.CampaignID), Address: addr, Note: strings.TrimSpace(req.Note), CreatedAt: s.now().UTC()}
	if entry.CampaignID == "" {
		return ErrInvalidCampaign
	}
	if _, err := store.GetCampaign(ctx, entry.CampaignID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return ErrInvalidCampaign
		}
		return err
	}
	existed, err := store.IsAddressAllowlisted(ctx, entry.CampaignID, entry.Address)
	if err != nil {
		return err
	}
	if err := store.AddCampaignAllowlistEntry(ctx, entry); err != nil {
		return err
	}
	if err := s.appendAudit(ctx, AuditEntry{Action: "campaign_allowlist_add", Actor: actor, Target: entry.CampaignID, CreatedAt: s.now().UTC().Format(time.RFC3339)}); err != nil {
		if !existed {
			if rollback, ok := store.(campaignRollbackStore); ok {
				_ = rollback.RemoveCampaignAllowlistEntry(ctx, entry.CampaignID, entry.Address)
			}
		}
		return err
	}
	return nil
}

func copyCampaign(c domain.Campaign) domain.Campaign {
	if c.BudgetWei != nil {
		c.BudgetWei = new(big.Int).Set(c.BudgetWei)
	}
	if c.StartsAt != nil {
		t := c.StartsAt.UTC()
		c.StartsAt = &t
	}
	if c.EndsAt != nil {
		t := c.EndsAt.UTC()
		c.EndsAt = &t
	}
	return c
}

func campaignFromRequest(req CampaignRequest) (domain.Campaign, error) {
	now := time.Now().UTC()
	campaign := domain.Campaign{ID: strings.TrimSpace(req.ID), Name: strings.TrimSpace(req.Name), TokenID: strings.TrimSpace(req.TokenID), Scope: domain.CampaignScope(strings.TrimSpace(req.Scope)), Enabled: req.Enabled, CreatedAt: now, UpdatedAt: now}
	if campaign.ID == "" || campaign.Name == "" || !domain.IsValidCampaignScope(campaign.Scope) {
		return domain.Campaign{}, ErrInvalidCampaign
	}
	if strings.TrimSpace(req.BudgetWei) != "" {
		v, ok := new(big.Int).SetString(strings.TrimSpace(req.BudgetWei), 10)
		if !ok || v.Sign() < 0 {
			return domain.Campaign{}, ErrInvalidCampaign
		}
		campaign.BudgetWei = v
	}
	if strings.TrimSpace(req.StartsAt) != "" {
		t, err := time.Parse(time.RFC3339, strings.TrimSpace(req.StartsAt))
		if err != nil {
			return domain.Campaign{}, ErrInvalidCampaign
		}
		campaign.StartsAt = &t
	}
	if strings.TrimSpace(req.EndsAt) != "" {
		t, err := time.Parse(time.RFC3339, strings.TrimSpace(req.EndsAt))
		if err != nil {
			return domain.Campaign{}, ErrInvalidCampaign
		}
		campaign.EndsAt = &t
	}
	if campaign.StartsAt != nil && campaign.EndsAt != nil && !campaign.StartsAt.Before(*campaign.EndsAt) {
		return domain.Campaign{}, ErrInvalidCampaign
	}
	return campaign, nil
}

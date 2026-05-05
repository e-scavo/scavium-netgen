package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"scavium-netgen/cmd/scavium-faucet/internal/abuse"
	"scavium-netgen/cmd/scavium-faucet/internal/admin"
	"scavium-netgen/cmd/scavium-faucet/internal/config"
	"scavium-netgen/cmd/scavium-faucet/internal/domain"
	"scavium-netgen/cmd/scavium-faucet/internal/faucet"
	"scavium-netgen/cmd/scavium-faucet/internal/frontend"
	"scavium-netgen/cmd/scavium-faucet/internal/iputil"
	"scavium-netgen/cmd/scavium-faucet/internal/observability"
	"scavium-netgen/cmd/scavium-faucet/internal/ready"
	"scavium-netgen/cmd/scavium-faucet/internal/version"
)

const requestIDHeader = "X-Request-ID"
const correlationIDHeader = "X-Correlation-ID"
const idempotencyKeyHeader = "Idempotency-Key"
const maxAdminListLimit = 500

type requestIDContextKey struct{}
type correlationIDContextKey struct{}

type healthResponse struct {
	Status        string                            `json:"status"`
	Time          string                            `json:"time"`
	UptimeSeconds int64                             `json:"uptime_seconds"`
	Build         observability.RuntimeMetricsBuild `json:"build"`
}

type ErrorEnvelope struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details"`
	RequestID string         `json:"request_id"`
}

type claimRequest struct {
	Address      string `json:"address"`
	TokenID      string `json:"token_id"`
	CaptchaToken string `json:"captcha_token"`
	Fingerprint  string `json:"fingerprint"`
}

// adminRuntimeResponse aggregates admin-plane runtime visibility without
// changing existing public or admin endpoint contracts.
type adminRuntimeResponse struct {
	Time      string                               `json:"time"`
	Dashboard admin.DashboardResponse              `json:"dashboard"`
	Readiness ready.Result                         `json:"readiness"`
	Metrics   observability.RuntimeMetricsSnapshot `json:"metrics"`
}

// Dependencies groups the services and settings required to build the HTTP API.
type Dependencies struct {
	ReadinessChecks []ready.Check
	ReadService     faucet.ReadService
	VersionInfo     version.Info
	AdminService    admin.AdminService
	// AdminToken is the bearer token required for admin endpoints.
	// If empty, admin endpoints respond 503 (disabled).
	// Never log this value.
	AdminToken string
	// TrustedProxy enables X-Forwarded-For/X-Real-IP processing when RemoteAddr
	// matches this proxy address.
	TrustedProxy string
	// CORSOrigins lists exact origins allowed for public API CORS.
	// Empty means CORS is disabled and no CORS headers are emitted.
	CORSOrigins []string
	// Logger receives production-safe request logs when provided.
	Logger *observability.Logger
	// Metrics receives lightweight in-process runtime counters when provided.
	Metrics *observability.RuntimeMetrics
}

// NewHandler builds the public and admin HTTP routes for the faucet service.
func NewHandler(deps Dependencies) http.Handler {
	if deps.ReadinessChecks == nil {
		deps.ReadinessChecks = ready.DefaultChecks()
	}
	if deps.ReadService == nil {
		deps.ReadService = faucet.NewInMemoryReadService(configDefaults())
	}
	if deps.VersionInfo == (version.Info{}) {
		deps.VersionInfo = version.Current()
	}
	if deps.Metrics == nil {
		deps.Metrics = observability.NewRuntimeMetrics(deps.VersionInfo)
	}
	if deps.AdminService == nil {
		deps.AdminService = admin.NewInMemoryAdminService()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", handleHealth(deps.Metrics))
	mux.HandleFunc("/ready", handleReady(deps.ReadinessChecks))
	mux.HandleFunc("/api/v1/status", handleFaucetStatus(deps.ReadService))
	mux.HandleFunc("/api/v1/config", handleFaucetConfig(deps.ReadService))
	mux.HandleFunc("/api/v1/tokens", handleFaucetTokens(deps.ReadService))
	mux.HandleFunc("/api/v1/claim", handleCreateClaim(deps.ReadService, deps.TrustedProxy, deps.Logger, deps.Metrics))
	mux.HandleFunc("/api/v1/claim/", handleGetClaim(deps.ReadService, "/api/v1/claim/"))
	mux.HandleFunc("/api/v1/address/", handleAddressStatus(deps.ReadService, "/api/v1/address/", "/status"))
	mux.HandleFunc("/api/v1/faucet/status", handleFaucetStatus(deps.ReadService))
	mux.HandleFunc("/api/v1/faucet/config", handleFaucetConfig(deps.ReadService))
	mux.HandleFunc("/api/v1/faucet/tokens", handleFaucetTokens(deps.ReadService))
	mux.HandleFunc("/api/v1/faucet/claim", handleCreateClaim(deps.ReadService, deps.TrustedProxy, deps.Logger, deps.Metrics))
	mux.HandleFunc("/api/v1/faucet/claim/", handleGetClaim(deps.ReadService, "/api/v1/faucet/claim/"))
	mux.HandleFunc("/api/v1/faucet/address/", handleAddressStatus(deps.ReadService, "/api/v1/faucet/address/", "/eligibility"))
	mux.HandleFunc("/api/v1/version", handleVersion(deps.VersionInfo))
	// Unknown /api/ paths get a JSON 404; everything else is served by the frontend.
	mux.HandleFunc("/api/", handleNotFound)
	mux.Handle("/", frontend.Handler())

	// Admin routes — all protected by bearer-token auth.
	adminMux := http.NewServeMux()
	adminMux.HandleFunc("/api/v1/admin/dashboard", handleAdminDashboard(deps.AdminService))
	adminMux.HandleFunc("/api/v1/admin/runtime", handleAdminRuntime(deps.AdminService, deps.ReadinessChecks, deps.Metrics))
	adminMux.HandleFunc("/api/v1/admin/queue", handleAdminQueue(deps.AdminService))
	adminMux.HandleFunc("/api/v1/admin/queue/", handleAdminQueueDispatch(deps.AdminService, deps.Logger, deps.TrustedProxy, "/api/v1/admin/queue/"))
	adminMux.HandleFunc("/api/v1/admin/metrics", handleAdminMetrics(deps.Metrics))
	adminMux.HandleFunc("/api/v1/admin/claims", handleAdminListClaims(deps.AdminService))
	adminMux.HandleFunc("/api/v1/admin/claim/", handleAdminClaimDispatch(deps.AdminService, deps.Logger, deps.TrustedProxy, "/api/v1/admin/claim/"))
	adminMux.HandleFunc("/api/v1/admin/faucet/mode", handleAdminSetMode(deps.AdminService, deps.Logger, deps.TrustedProxy))
	adminMux.HandleFunc("/api/v1/admin/blocklist", handleAdminBlocklist(deps.AdminService, deps.Logger, deps.TrustedProxy))
	adminMux.HandleFunc("/api/v1/admin/audit", handleAdminAuditLog(deps.AdminService))
	mux.Handle("/api/v1/admin/", admin.TokenAuthMiddleware(deps.AdminToken, adminMux))

	var handler http.Handler = mux
	if deps.Logger != nil {
		handler = RequestLoggingMiddleware(handler, deps.Logger, deps.TrustedProxy)
	}
	return RequestIDMiddleware(CORSHandler(handler, deps.CORSOrigins))
}

func handleHealth(metrics *observability.RuntimeMetrics) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			WriteError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed", nil)
			return
		}

		now := time.Now().UTC()
		snapshot := metrics.Snapshot(now)
		WriteJSON(w, http.StatusOK, healthResponse{
			Status:        "ok",
			Time:          now.Format(time.RFC3339),
			UptimeSeconds: snapshot.UptimeSeconds,
			Build:         snapshot.Build,
		})
	}
}

func handleNotFound(w http.ResponseWriter, r *http.Request) {
	WriteError(w, r, http.StatusNotFound, "not_found", "not found", nil)
}

func handleReady(checks []ready.Check) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			WriteError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed", nil)
			return
		}

		result := ready.Evaluate(r.Context(), checks)
		statusCode := http.StatusOK
		if result.Status != ready.StatusOK {
			statusCode = http.StatusServiceUnavailable
		}

		WriteJSON(w, statusCode, result)
	}
}

func handleVersion(info version.Info) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			WriteError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed", nil)
			return
		}

		WriteJSON(w, http.StatusOK, info)
	}
}

func handleFaucetStatus(readService faucet.ReadService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			WriteError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed", nil)
			return
		}

		status, err := readService.Status(r.Context())
		if err != nil {
			WriteError(w, r, http.StatusInternalServerError, "status_unavailable", "status unavailable", nil)
			return
		}
		WriteJSON(w, http.StatusOK, status)
	}
}

func handleFaucetConfig(readService faucet.ReadService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			WriteError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed", nil)
			return
		}

		cfg, err := readService.Config(r.Context())
		if err != nil {
			WriteError(w, r, http.StatusInternalServerError, "config_unavailable", "config unavailable", nil)
			return
		}
		WriteJSON(w, http.StatusOK, cfg)
	}
}

func handleFaucetTokens(readService faucet.ReadService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			WriteError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed", nil)
			return
		}

		tokens, err := readService.Tokens(r.Context())
		if err != nil {
			WriteError(w, r, http.StatusInternalServerError, "tokens_unavailable", "tokens unavailable", nil)
			return
		}
		WriteJSON(w, http.StatusOK, map[string]any{
			"tokens": tokens,
		})
	}
}

func handleAddressStatus(readService faucet.ReadService, prefix, suffix string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			WriteError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed", nil)
			return
		}

		addressText, ok := pathMiddle(r.URL.Path, prefix, suffix)
		if !ok {
			handleNotFound(w, r)
			return
		}

		address, err := domain.ValidateAddress(addressText)
		if err != nil {
			WriteError(w, r, http.StatusBadRequest, "invalid_address", "invalid address", map[string]any{
				"reason": err.Error(),
			})
			return
		}

		status, err := readService.AddressStatus(r.Context(), address)
		if err != nil {
			WriteError(w, r, http.StatusInternalServerError, "address_status_unavailable", "address status unavailable", nil)
			return
		}
		WriteJSON(w, http.StatusOK, status)
	}
}

func handleCreateClaim(readService faucet.ReadService, trustedProxy string, logger *observability.Logger, metrics *observability.RuntimeMetrics) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			WriteError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed", nil)
			return
		}

		// TODO(step 5.x): optional address-ownership challenge/signature verification.
		// The wallet may include an `X-Signature` header containing a signed challenge
		// to prove control of the address before the claim is enqueued.

		var body claimRequest
		if err := decodeNoTrailingTokens(json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)), &body); err != nil {
			WriteError(w, r, http.StatusBadRequest, "invalid_json", "invalid JSON body", nil)
			return
		}

		address, err := domain.ValidateAddress(body.Address)
		if err != nil {
			WriteError(w, r, http.StatusBadRequest, "invalid_address", "invalid address", map[string]any{
				"reason": err.Error(),
			})
			return
		}

		claimRequest := faucet.ClaimRequest{
			Address:        address,
			TokenID:        strings.TrimSpace(body.TokenID),
			IdempotencyKey: strings.TrimSpace(r.Header.Get(idempotencyKeyHeader)),
			RemoteIP:       iputil.RealIP(r, trustedProxy),
			UserAgent:      r.UserAgent(),
			CaptchaToken:   strings.TrimSpace(body.CaptchaToken),
			Fingerprint:    strings.TrimSpace(body.Fingerprint),
		}

		claim, err := readService.CreateClaim(r.Context(), claimRequest)
		observedTokenID := observedRequestTokenID(claimRequest.TokenID)
		if err != nil {
			metrics.IncClaimRejectedForToken(observedTokenID, claimErrorCode(err))
			logClaimRejected(logger, r, claimRequest, err)
			handleCreateClaimError(w, r, err)
			return
		}

		metrics.IncClaimAcceptedForToken(observedTokenID)
		logClaimAccepted(logger, r, claimRequest, claim)
		WriteJSON(w, http.StatusAccepted, claim)
	}
}

func handleCreateClaimError(w http.ResponseWriter, r *http.Request, err error) {
	details := claimErrorDetails(err)
	switch {
	case errors.Is(err, faucet.ErrFaucetUnavailable):
		WriteError(w, r, http.StatusServiceUnavailable, "faucet_unavailable", "faucet unavailable", details)
	case errors.Is(err, faucet.ErrCaptchaFailed):
		WriteError(w, r, http.StatusUnprocessableEntity, "captcha_failed", "captcha failed", details)
	case errors.Is(err, faucet.ErrClaimRejected):
		WriteError(w, r, http.StatusForbidden, "claim_rejected", "claim rejected", details)
	case errors.Is(err, faucet.ErrDailyBudgetExceeded):
		WriteError(w, r, http.StatusTooManyRequests, "daily_budget_exceeded", "daily budget exceeded", details)
	case errors.Is(err, faucet.ErrCooldownActive), errors.Is(err, faucet.ErrRateLimited):
		WriteError(w, r, http.StatusTooManyRequests, "rate_limited", "rate limited", details)
	default:
		WriteError(w, r, http.StatusInternalServerError, "claim_unavailable", "claim unavailable", nil)
	}
}

func claimErrorDetails(err error) map[string]any {
	var claimErr *faucet.ClaimError
	if !errors.As(err, &claimErr) {
		return nil
	}

	details := map[string]any{}
	if claimErr.Reason != "" {
		details["reason"] = claimErr.Reason
	}
	if claimErr.RetryAfterSeconds > 0 {
		details["retry_after_seconds"] = claimErr.RetryAfterSeconds
	}
	if len(details) == 0 {
		return nil
	}
	return details
}

func logClaimAccepted(logger *observability.Logger, r *http.Request, request faucet.ClaimRequest, claim faucet.ClaimResponse) {
	if logger == nil {
		return
	}
	fields := claimLogFields(r, request)
	fields["claim_id"] = claim.ID
	fields["token_id"] = claim.TokenID
	fields["claim_status"] = string(claim.Status)
	fields["event"] = "token_claim_accepted"
	logger.Info("claim accepted", fields)
}

func logClaimRejected(logger *observability.Logger, r *http.Request, request faucet.ClaimRequest, err error) {
	if logger == nil {
		return
	}
	fields := claimLogFields(r, request)
	fields["error_code"] = claimErrorCode(err)
	fields["token_id"] = observedRequestTokenID(request.TokenID)
	if details := claimErrorDetails(err); details != nil {
		if reason, ok := details["reason"].(string); ok && reason != "" {
			fields["reason"] = reason
			if reason == "invalid_token" {
				fields["event"] = "token_validation_failed"
			}
		}
		if retryAfter, ok := details["retry_after_seconds"]; ok {
			fields["retry_after_seconds"] = retryAfter
		}
	}
	logger.Info("claim rejected", fields)
}

func claimLogFields(r *http.Request, request faucet.ClaimRequest) map[string]any {
	return map[string]any{
		"request_id":            RequestID(r),
		"correlation_id":        CorrelationID(r),
		"remote_ip":             request.RemoteIP,
		"has_idempotency_key":   request.IdempotencyKey != "",
		"has_fingerprint":       request.Fingerprint != "",
		"captcha_token_present": request.CaptchaToken != "",
	}
}

func observedRequestTokenID(tokenID string) string {
	trimmed := strings.TrimSpace(tokenID)
	if trimmed == "" {
		return "default"
	}
	return sanitizeTokenID(trimmed)
}

// sanitizeTokenID strips non-printable ASCII characters and truncates to 64 bytes
// to prevent log contamination from adversarial token_id values.
func sanitizeTokenID(id string) string {
	const maxLen = 64
	out := make([]byte, 0, len(id))
	for i := 0; i < len(id) && len(out) < maxLen; i++ {
		if id[i] >= 0x20 && id[i] < 0x7f {
			out = append(out, id[i])
		}
	}
	return string(out)
}

func claimErrorCode(err error) string {
	switch {
	case errors.Is(err, faucet.ErrFaucetUnavailable):
		return "faucet_unavailable"
	case errors.Is(err, faucet.ErrCaptchaFailed):
		return "captcha_failed"
	case errors.Is(err, faucet.ErrClaimRejected):
		if claimErr, ok := claimErrorAs(err); ok && claimErr.Reason == "invalid_token" {
			return "invalid_token"
		}
		return "claim_rejected"
	case errors.Is(err, faucet.ErrDailyBudgetExceeded):
		return "daily_budget_exceeded"
	case errors.Is(err, faucet.ErrCooldownActive), errors.Is(err, faucet.ErrRateLimited):
		return "rate_limited"
	default:
		return "claim_unavailable"
	}
}

func claimErrorAs(err error) (*faucet.ClaimError, bool) {
	var claimErr *faucet.ClaimError
	if errors.As(err, &claimErr) {
		return claimErr, true
	}
	return nil, false
}

func handleGetClaim(readService faucet.ReadService, prefix string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			WriteError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed", nil)
			return
		}

		id, ok := pathRemainder(r.URL.Path, prefix)
		if !ok {
			handleNotFound(w, r)
			return
		}

		claim, found, err := readService.GetClaim(r.Context(), id)
		if err != nil {
			WriteError(w, r, http.StatusInternalServerError, "claim_unavailable", "claim unavailable", nil)
			return
		}
		if !found {
			WriteError(w, r, http.StatusNotFound, "claim_not_found", "claim not found", nil)
			return
		}

		WriteJSON(w, http.StatusOK, claim)
	}
}

// RequestIDMiddleware ensures each request carries stable request and correlation IDs.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.TrimSpace(r.Header.Get(requestIDHeader))
		if requestID == "" {
			requestID = newRequestID()
		}

		correlationID := strings.TrimSpace(r.Header.Get(correlationIDHeader))
		if correlationID == "" {
			correlationID = requestID
		}

		w.Header().Set(requestIDHeader, requestID)
		w.Header().Set(correlationIDHeader, correlationID)

		ctx := context.WithValue(r.Context(), requestIDContextKey{}, requestID)
		ctx = context.WithValue(ctx, correlationIDContextKey{}, correlationID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestID returns the request ID attached by RequestIDMiddleware.
func RequestID(r *http.Request) string {
	requestID, _ := r.Context().Value(requestIDContextKey{}).(string)
	return requestID
}

// CorrelationID returns the correlation ID attached by RequestIDMiddleware.
func CorrelationID(r *http.Request) string {
	correlationID, _ := r.Context().Value(correlationIDContextKey{}).(string)
	return correlationID
}

// WriteJSON writes body as JSON with the provided HTTP status code.
func WriteJSON(w http.ResponseWriter, statusCode int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(body)
}

// WriteError writes a normalized JSON error response.
func WriteError(w http.ResponseWriter, r *http.Request, statusCode int, code, message string, details map[string]any) {
	if details == nil {
		details = map[string]any{}
	}

	WriteJSON(w, statusCode, ErrorEnvelope{
		Code:      code,
		Message:   message,
		Details:   details,
		RequestID: RequestID(r),
	})
}

func newRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UTC().UnixNano())
	}
	return fmt.Sprintf("%x", b[:])
}

func pathMiddle(path, prefix, suffix string) (string, bool) {
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return "", false
	}
	middle := strings.TrimSuffix(strings.TrimPrefix(path, prefix), suffix)
	if middle == "" || strings.Contains(middle, "/") {
		return "", false
	}
	return middle, true
}

func pathRemainder(path, prefix string) (string, bool) {
	if !strings.HasPrefix(path, prefix) {
		return "", false
	}
	remainder := strings.TrimPrefix(path, prefix)
	if remainder == "" || strings.Contains(remainder, "/") {
		return "", false
	}
	return remainder, true
}

func configDefaults() config.Config {
	return config.Defaults()
}

func decodeNoTrailingTokens(decoder *json.Decoder, v any) error {
	if err := decoder.Decode(v); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return errors.New("unexpected trailing JSON")
		}
		return err
	}
	return nil
}

// --- Admin handlers --------------------------------------------------------

func handleAdminRuntime(svc admin.AdminService, checks []ready.Check, metrics *observability.RuntimeMetrics) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			WriteError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed", nil)
			return
		}

		dashboard, err := svc.Dashboard(r.Context())
		if err != nil {
			WriteError(w, r, http.StatusInternalServerError, "runtime_unavailable", "runtime unavailable", nil)
			return
		}

		now := time.Now().UTC()
		WriteJSON(w, http.StatusOK, adminRuntimeResponse{
			Time:      now.Format(time.RFC3339),
			Dashboard: dashboard,
			Readiness: ready.Evaluate(r.Context(), checks),
			Metrics:   metrics.Snapshot(now),
		})
	}
}

func adminListLimitFromQuery(r *http.Request, fallback int) int {
	limit := fallback
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > maxAdminListLimit {
		return maxAdminListLimit
	}
	return limit
}

func parseAdminBlocklistKeyType(value string) (abuse.KeyType, bool) {
	switch abuse.KeyType(strings.TrimSpace(value)) {
	case abuse.KeyTypeIP:
		return abuse.KeyTypeIP, true
	case abuse.KeyTypeAddress:
		return abuse.KeyTypeAddress, true
	case abuse.KeyTypeFingerprint:
		return abuse.KeyTypeFingerprint, true
	default:
		return "", false
	}
}

func handleAdminQueue(svc admin.AdminService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			WriteError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed", nil)
			return
		}
		limit := adminListLimitFromQuery(r, 50)
		queue, err := svc.Queue(r.Context(), limit)
		if err != nil {
			WriteError(w, r, http.StatusInternalServerError, "queue_unavailable", "queue unavailable", nil)
			return
		}
		WriteJSON(w, http.StatusOK, queue)
	}
}

func handleAdminMetrics(metrics *observability.RuntimeMetrics) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			WriteError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed", nil)
			return
		}

		WriteJSON(w, http.StatusOK, metrics.Snapshot(time.Now().UTC()))
	}
}

func actorFromRequest(r *http.Request, trustedProxy string) string {
	return iputil.RealIP(r, trustedProxy)
}

func logAdminAudit(logger *observability.Logger, r *http.Request, action, actor, target string, fields map[string]any) {
	if logger == nil {
		return
	}
	entry := map[string]any{
		"request_id":     RequestID(r),
		"correlation_id": CorrelationID(r),
		"admin_action":   action,
		"actor":          actor,
		"target":         target,
		"path":           redactAccessLogPath(r.URL.EscapedPath()),
	}
	for k, v := range fields {
		entry[k] = v
	}
	logger.Info("admin audit", entry)
}

func handleAdminDashboard(svc admin.AdminService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			WriteError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed", nil)
			return
		}
		dash, err := svc.Dashboard(r.Context())
		if err != nil {
			WriteError(w, r, http.StatusInternalServerError, "dashboard_unavailable", "dashboard unavailable", nil)
			return
		}
		WriteJSON(w, http.StatusOK, dash)
	}
}

// handleAdminQueueDispatch exposes queue-oriented control actions without
// changing the existing claim-specific admin endpoints.
//   - POST /api/v1/admin/queue/retry  {"id":"<claim_id>"}
//   - POST /api/v1/admin/queue/cancel {"id":"<claim_id>"}
func handleAdminQueueDispatch(svc admin.AdminService, logger *observability.Logger, trustedProxy, prefix string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		action := strings.Trim(strings.TrimPrefix(r.URL.Path, prefix), "/")
		switch action {
		case "retry", "cancel":
		default:
			handleNotFound(w, r)
			return
		}

		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			WriteError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed", nil)
			return
		}

		var body admin.QueueControlRequest
		if err := decodeNoTrailingTokens(json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)), &body); err != nil {
			WriteError(w, r, http.StatusBadRequest, "invalid_json", "invalid JSON body", nil)
			return
		}
		body.ID = strings.TrimSpace(body.ID)
		if body.ID == "" {
			WriteError(w, r, http.StatusBadRequest, "missing_id", "id is required", nil)
			return
		}

		actor := actorFromRequest(r, trustedProxy)
		switch action {
		case "retry":
			if err := svc.RetryClaim(r.Context(), body.ID, actor); err != nil {
				handleAdminServiceError(w, r, err)
				return
			}
			logAdminAudit(logger, r, "queue_retry", actor, body.ID, map[string]any{"result": "success"})
			WriteJSON(w, http.StatusOK, map[string]string{"status": "retried", "id": body.ID})
		case "cancel":
			if err := svc.CancelClaim(r.Context(), body.ID, actor); err != nil {
				handleAdminServiceError(w, r, err)
				return
			}
			logAdminAudit(logger, r, "queue_cancel", actor, body.ID, map[string]any{"result": "success"})
			WriteJSON(w, http.StatusOK, map[string]string{"status": "cancelled", "id": body.ID})
		}
	}
}

func handleAdminListClaims(svc admin.AdminService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			WriteError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed", nil)
			return
		}
		limit := adminListLimitFromQuery(r, 50)
		offset := 0
		if v := r.URL.Query().Get("offset"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n >= 0 {
				offset = n
			}
		}
		claims, err := svc.ListClaims(r.Context(), limit, offset)
		if err != nil {
			WriteError(w, r, http.StatusInternalServerError, "list_claims_unavailable", "list claims unavailable", nil)
			return
		}
		WriteJSON(w, http.StatusOK, map[string]any{"claims": claims})
	}
}

// handleAdminClaimDispatch dispatches based on the path after the prefix:
//   - GET  {id}         → claim detail
//   - POST {id}/retry   → retry claim
//   - POST {id}/cancel  → cancel claim
func handleAdminClaimDispatch(svc admin.AdminService, logger *observability.Logger, trustedProxy, prefix string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tail := strings.TrimPrefix(r.URL.Path, prefix)
		parts := strings.SplitN(tail, "/", 2)
		id := parts[0]
		if id == "" {
			handleNotFound(w, r)
			return
		}

		if len(parts) == 1 {
			// GET /api/v1/admin/claim/{id}
			if r.Method != http.MethodGet {
				w.Header().Set("Allow", http.MethodGet)
				WriteError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed", nil)
				return
			}
			c, found, err := svc.GetClaim(r.Context(), id)
			if err != nil {
				WriteError(w, r, http.StatusInternalServerError, "get_claim_unavailable", "get claim unavailable", nil)
				return
			}
			if !found {
				WriteError(w, r, http.StatusNotFound, "claim_not_found", "claim not found", nil)
				return
			}
			WriteJSON(w, http.StatusOK, c)
			return
		}

		action := parts[1]
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			WriteError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed", nil)
			return
		}
		actor := actorFromRequest(r, trustedProxy)
		switch action {
		case "retry":
			if err := svc.RetryClaim(r.Context(), id, actor); err != nil {
				handleAdminServiceError(w, r, err)
				return
			}
			logAdminAudit(logger, r, "claim_retry", actor, id, map[string]any{"result": "success"})
			WriteJSON(w, http.StatusOK, map[string]string{"status": "retried"})
		case "cancel":
			if err := svc.CancelClaim(r.Context(), id, actor); err != nil {
				handleAdminServiceError(w, r, err)
				return
			}
			logAdminAudit(logger, r, "claim_cancel", actor, id, map[string]any{"result": "success"})
			WriteJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
		default:
			handleNotFound(w, r)
		}
	}
}

func handleAdminSetMode(svc admin.AdminService, logger *observability.Logger, trustedProxy string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			WriteError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed", nil)
			return
		}
		var body admin.SetModeRequest
		if err := decodeNoTrailingTokens(json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)), &body); err != nil {
			WriteError(w, r, http.StatusBadRequest, "invalid_json", "invalid JSON body", nil)
			return
		}
		body.Mode = strings.TrimSpace(body.Mode)
		if body.Mode == "" {
			WriteError(w, r, http.StatusBadRequest, "missing_mode", "mode is required", nil)
			return
		}
		actor := actorFromRequest(r, trustedProxy)
		if err := svc.SetMode(r.Context(), body.Mode, actor); err != nil {
			if errors.Is(err, admin.ErrInvalidMode) {
				WriteError(w, r, http.StatusBadRequest, "invalid_mode", "mode must be active, paused, or maintenance", nil)
				return
			}
			WriteError(w, r, http.StatusInternalServerError, "set_mode_failed", "set mode failed", nil)
			return
		}
		logAdminAudit(logger, r, "set_mode", actor, "faucet", map[string]any{"result": "success", "mode": body.Mode})
		WriteJSON(w, http.StatusOK, map[string]string{"mode": body.Mode})
	}
}

func handleAdminBlocklist(svc admin.AdminService, logger *observability.Logger, trustedProxy string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			entries, err := svc.BlocklistList(r.Context())
			if err != nil {
				WriteError(w, r, http.StatusInternalServerError, "blocklist_unavailable", "blocklist unavailable", nil)
				return
			}
			WriteJSON(w, http.StatusOK, map[string]any{"entries": entries})

		case http.MethodPost:
			var body admin.BlocklistAddRequest
			if err := decodeNoTrailingTokens(json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)), &body); err != nil {
				WriteError(w, r, http.StatusBadRequest, "invalid_json", "invalid JSON body", nil)
				return
			}
			if body.KeyType == "" || body.Value == "" {
				WriteError(w, r, http.StatusBadRequest, "missing_fields", "key_type and value are required", nil)
				return
			}
			kt, ok := parseAdminBlocklistKeyType(body.KeyType)
			if !ok {
				WriteError(w, r, http.StatusBadRequest, "invalid_key_type", "key_type must be ip, address, or fingerprint", nil)
				return
			}
			actor := actorFromRequest(r, trustedProxy)
			if err := svc.BlocklistAdd(r.Context(), kt, body.Value, body.Reason, actor); err != nil {
				WriteError(w, r, http.StatusInternalServerError, "blocklist_add_failed", "blocklist add failed", nil)
				return
			}
			logAdminAudit(logger, r, "blocklist_add", actor, "blocklist", map[string]any{"result": "success", "key_type": string(kt), "has_reason": body.Reason != ""})
			WriteJSON(w, http.StatusCreated, map[string]string{"status": "blocked"})

		case http.MethodDelete:
			// Accept key_type and value as query parameters for DELETE.
			kt := r.URL.Query().Get("key_type")
			value := r.URL.Query().Get("value")
			if kt == "" || value == "" {
				WriteError(w, r, http.StatusBadRequest, "missing_fields", "key_type and value query params are required", nil)
				return
			}
			keyType, ok := parseAdminBlocklistKeyType(kt)
			if !ok {
				WriteError(w, r, http.StatusBadRequest, "invalid_key_type", "key_type must be ip, address, or fingerprint", nil)
				return
			}
			actor := actorFromRequest(r, trustedProxy)
			if err := svc.BlocklistRemove(r.Context(), keyType, value, actor); err != nil {
				WriteError(w, r, http.StatusInternalServerError, "blocklist_remove_failed", "blocklist remove failed", nil)
				return
			}
			logAdminAudit(logger, r, "blocklist_remove", actor, "blocklist", map[string]any{"result": "success", "key_type": string(keyType)})
			WriteJSON(w, http.StatusOK, map[string]string{"status": "unblocked"})

		default:
			w.Header().Set("Allow", "GET, POST, DELETE")
			WriteError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed", nil)
		}
	}
}

func handleAdminAuditLog(svc admin.AdminService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			WriteError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed", nil)
			return
		}
		limit := adminListLimitFromQuery(r, 100)
		entries, err := svc.RecentAuditLog(r.Context(), limit)
		if err != nil {
			WriteError(w, r, http.StatusInternalServerError, "audit_log_unavailable", "audit log unavailable", nil)
			return
		}
		WriteJSON(w, http.StatusOK, map[string]any{"entries": entries})
	}
}

// handleAdminServiceError maps admin sentinel errors to HTTP status codes.
func handleAdminServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, admin.ErrNotFound):
		WriteError(w, r, http.StatusNotFound, "not_found", "not found", nil)
	case errors.Is(err, admin.ErrNotRetryable):
		WriteError(w, r, http.StatusConflict, "not_retryable", "claim is not in a retryable state", nil)
	case errors.Is(err, admin.ErrNotCancellable):
		WriteError(w, r, http.StatusConflict, "not_cancellable", "claim cannot be cancelled in its current state", nil)
	default:
		WriteError(w, r, http.StatusInternalServerError, "internal_error", "internal error", nil)
	}
}

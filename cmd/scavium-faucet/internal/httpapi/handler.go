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
	"scavium-netgen/cmd/scavium-faucet/internal/ready"
	"scavium-netgen/cmd/scavium-faucet/internal/version"
)

const requestIDHeader = "X-Request-ID"
const idempotencyKeyHeader = "Idempotency-Key"

type requestIDContextKey struct{}

type healthResponse struct {
	Status string `json:"status"`
	Time   string `json:"time"`
}

type ErrorEnvelope struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details"`
	RequestID string         `json:"request_id"`
}

type claimRequest struct {
	Address string `json:"address"`
}

type Dependencies struct {
	ReadinessChecks []ready.Check
	ReadService     faucet.ReadService
	VersionInfo     version.Info
	AdminService    admin.AdminService
	// AdminToken is the bearer token required for admin endpoints.
	// If empty, admin endpoints respond 503 (disabled).
	// Never log this value.
	AdminToken string
}

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
	if deps.AdminService == nil {
		deps.AdminService = admin.NewInMemoryAdminService()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/ready", handleReady(deps.ReadinessChecks))
	mux.HandleFunc("/api/v1/status", handleFaucetStatus(deps.ReadService))
	mux.HandleFunc("/api/v1/config", handleFaucetConfig(deps.ReadService))
	mux.HandleFunc("/api/v1/claim", handleCreateClaim(deps.ReadService))
	mux.HandleFunc("/api/v1/claim/", handleGetClaim(deps.ReadService, "/api/v1/claim/"))
	mux.HandleFunc("/api/v1/address/", handleAddressStatus(deps.ReadService, "/api/v1/address/", "/status"))
	mux.HandleFunc("/api/v1/faucet/status", handleFaucetStatus(deps.ReadService))
	mux.HandleFunc("/api/v1/faucet/config", handleFaucetConfig(deps.ReadService))
	mux.HandleFunc("/api/v1/faucet/claim", handleCreateClaim(deps.ReadService))
	mux.HandleFunc("/api/v1/faucet/claim/", handleGetClaim(deps.ReadService, "/api/v1/faucet/claim/"))
	mux.HandleFunc("/api/v1/faucet/address/", handleAddressStatus(deps.ReadService, "/api/v1/faucet/address/", "/eligibility"))
	mux.HandleFunc("/api/v1/version", handleVersion(deps.VersionInfo))
	// Unknown /api/ paths get a JSON 404; everything else is served by the frontend.
	mux.HandleFunc("/api/", handleNotFound)
	mux.Handle("/", frontend.Handler())

	// Admin routes — all protected by bearer-token auth.
	adminMux := http.NewServeMux()
	adminMux.HandleFunc("/api/v1/admin/dashboard", handleAdminDashboard(deps.AdminService))
	adminMux.HandleFunc("/api/v1/admin/claims", handleAdminListClaims(deps.AdminService))
	adminMux.HandleFunc("/api/v1/admin/claim/", handleAdminClaimDispatch(deps.AdminService, "/api/v1/admin/claim/"))
	adminMux.HandleFunc("/api/v1/admin/faucet/mode", handleAdminSetMode(deps.AdminService))
	adminMux.HandleFunc("/api/v1/admin/blocklist", handleAdminBlocklist(deps.AdminService))
	adminMux.HandleFunc("/api/v1/admin/audit", handleAdminAuditLog(deps.AdminService))
	mux.Handle("/api/v1/admin/", admin.TokenAuthMiddleware(deps.AdminToken, adminMux))

	return RequestIDMiddleware(mux)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		WriteError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed", nil)
		return
	}

	WriteJSON(w, http.StatusOK, healthResponse{
		Status: "ok",
		Time:   time.Now().UTC().Format(time.RFC3339),
	})
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

func handleCreateClaim(readService faucet.ReadService) http.HandlerFunc {
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

		claim, err := readService.CreateClaim(r.Context(), faucet.ClaimRequest{
			Address:        address,
			IdempotencyKey: strings.TrimSpace(r.Header.Get(idempotencyKeyHeader)),
		})
		if err != nil {
			WriteError(w, r, http.StatusInternalServerError, "claim_unavailable", "claim unavailable", nil)
			return
		}

		WriteJSON(w, http.StatusAccepted, claim)
	}
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

func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get(requestIDHeader)
		if requestID == "" {
			requestID = newRequestID()
		}

		w.Header().Set(requestIDHeader, requestID)
		ctx := context.WithValue(r.Context(), requestIDContextKey{}, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequestID(r *http.Request) string {
	requestID, _ := r.Context().Value(requestIDContextKey{}).(string)
	return requestID
}

func WriteJSON(w http.ResponseWriter, statusCode int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(body)
}

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

func actorFromRequest(r *http.Request) string {
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		addr = addr[:idx]
	}
	return addr
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

func handleAdminListClaims(svc admin.AdminService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			WriteError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed", nil)
			return
		}
		limit := 50
		offset := 0
		if v := r.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				limit = n
			}
		}
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
func handleAdminClaimDispatch(svc admin.AdminService, prefix string) http.HandlerFunc {
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
		actor := actorFromRequest(r)
		switch action {
		case "retry":
			if err := svc.RetryClaim(r.Context(), id, actor); err != nil {
				handleAdminServiceError(w, r, err)
				return
			}
			WriteJSON(w, http.StatusOK, map[string]string{"status": "retried"})
		case "cancel":
			if err := svc.CancelClaim(r.Context(), id, actor); err != nil {
				handleAdminServiceError(w, r, err)
				return
			}
			WriteJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
		default:
			handleNotFound(w, r)
		}
	}
}

func handleAdminSetMode(svc admin.AdminService) http.HandlerFunc {
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
		if body.Mode == "" {
			WriteError(w, r, http.StatusBadRequest, "missing_mode", "mode is required", nil)
			return
		}
		actor := actorFromRequest(r)
		if err := svc.SetMode(r.Context(), body.Mode, actor); err != nil {
			WriteError(w, r, http.StatusInternalServerError, "set_mode_failed", "set mode failed", nil)
			return
		}
		WriteJSON(w, http.StatusOK, map[string]string{"mode": body.Mode})
	}
}

func handleAdminBlocklist(svc admin.AdminService) http.HandlerFunc {
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
			actor := actorFromRequest(r)
			if err := svc.BlocklistAdd(r.Context(), abuse.KeyType(body.KeyType), body.Value, body.Reason, actor); err != nil {
				WriteError(w, r, http.StatusInternalServerError, "blocklist_add_failed", "blocklist add failed", nil)
				return
			}
			WriteJSON(w, http.StatusCreated, map[string]string{"status": "blocked"})

		case http.MethodDelete:
			// Accept key_type and value as query parameters for DELETE.
			kt := r.URL.Query().Get("key_type")
			value := r.URL.Query().Get("value")
			if kt == "" || value == "" {
				WriteError(w, r, http.StatusBadRequest, "missing_fields", "key_type and value query params are required", nil)
				return
			}
			actor := actorFromRequest(r)
			if err := svc.BlocklistRemove(r.Context(), abuse.KeyType(kt), value, actor); err != nil {
				WriteError(w, r, http.StatusInternalServerError, "blocklist_remove_failed", "blocklist remove failed", nil)
				return
			}
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
		limit := 100
		if v := r.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				limit = n
			}
		}
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

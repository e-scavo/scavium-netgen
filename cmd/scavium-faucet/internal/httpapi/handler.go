package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"scavium-netgen/cmd/scavium-faucet/internal/config"
	"scavium-netgen/cmd/scavium-faucet/internal/domain"
	"scavium-netgen/cmd/scavium-faucet/internal/faucet"
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
	mux.HandleFunc("/", handleNotFound)
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

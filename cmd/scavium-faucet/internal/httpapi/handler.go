package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"scavium-netgen/cmd/scavium-faucet/internal/ready"
	"scavium-netgen/cmd/scavium-faucet/internal/version"
)

const requestIDHeader = "X-Request-ID"

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

type Dependencies struct {
	ReadinessChecks []ready.Check
	VersionInfo     version.Info
}

func NewHandler(deps Dependencies) http.Handler {
	if deps.ReadinessChecks == nil {
		deps.ReadinessChecks = ready.DefaultChecks()
	}
	if deps.VersionInfo == (version.Info{}) {
		deps.VersionInfo = version.Current()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/ready", handleReady(deps.ReadinessChecks))
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

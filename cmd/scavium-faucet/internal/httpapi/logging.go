package httpapi

import (
	"net/http"
	"time"

	"scavium-netgen/cmd/scavium-faucet/internal/iputil"
	"scavium-netgen/cmd/scavium-faucet/internal/observability"
)

// RequestLoggingMiddleware emits one production-safe access log entry per request.
func RequestLoggingMiddleware(next http.Handler, logger *observability.Logger, trustedProxy string) http.Handler {
	if logger == nil {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w}

		next.ServeHTTP(recorder, r)

		status := recorder.status
		if status == 0 {
			status = http.StatusOK
		}

		logger.Info("http request", map[string]any{
			"request_id":     RequestID(r),
			"correlation_id": CorrelationID(r),
			"method":         r.Method,
			"path":           redactAccessLogPath(r.URL.EscapedPath()),
			"status":         status,
			"duration":       time.Since(start).String(),
			"remote_ip":      iputil.RealIP(r, trustedProxy),
		})
	})
}

func redactAccessLogPath(path string) string {
	if _, ok := pathMiddle(path, "/api/v1/address/", "/status"); ok {
		return "/api/v1/address/:address/status"
	}
	if _, ok := pathMiddle(path, "/api/v1/address/", "/history"); ok {
		return "/api/v1/address/:address/history"
	}
	if _, ok := pathMiddle(path, "/api/v1/faucet/address/", "/eligibility"); ok {
		return "/api/v1/faucet/address/:address/eligibility"
	}
	if _, ok := pathMiddle(path, "/api/v1/faucet/address/", "/history"); ok {
		return "/api/v1/faucet/address/:address/history"
	}
	return path
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(statusCode int) {
	if r.status != 0 {
		return
	}
	r.status = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(b)
}

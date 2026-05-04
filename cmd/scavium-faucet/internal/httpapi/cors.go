package httpapi

import (
	"net/http"
	"strings"
)

const (
	corsAllowedMethods = "GET, POST, OPTIONS"
	corsAllowedHeaders = "Content-Type, Idempotency-Key, Authorization, X-Request-ID"
)

// CORSHandler applies exact-origin CORS to public API routes only.
func CORSHandler(next http.Handler, allowedOrigins []string) http.Handler {
	allowed := exactOriginSet(allowedOrigins)
	if len(allowed) == 0 {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin == "" || !isCORSPublicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		if _, ok := allowed[origin]; ok {
			header := w.Header()
			addVary(header, "Origin")
			header.Set("Access-Control-Allow-Origin", origin)
			header.Set("Access-Control-Allow-Methods", corsAllowedMethods)
			header.Set("Access-Control-Allow-Headers", corsAllowedHeaders)
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func exactOriginSet(origins []string) map[string]struct{} {
	allowed := map[string]struct{}{}
	for _, origin := range origins {
		origin = strings.TrimSpace(origin)
		if origin == "" || origin == "*" {
			continue
		}
		allowed[origin] = struct{}{}
	}
	return allowed
}

func isCORSPublicPath(path string) bool {
	if strings.HasPrefix(path, "/api/v1/admin/") {
		return false
	}
	return strings.HasPrefix(path, "/api/") ||
		path == "/health" ||
		path == "/ready"
}

func addVary(header http.Header, value string) {
	for _, existing := range header.Values("Vary") {
		for _, part := range strings.Split(existing, ",") {
			if strings.EqualFold(strings.TrimSpace(part), value) {
				return
			}
		}
	}
	header.Add("Vary", value)
}

package httpapi

import (
	"encoding/json"
	"net/http"
)

const maxJSONRequestBodyBytes int64 = 1 << 20

// RequestBodyLimitMiddleware rejects clearly oversized request bodies before
// they reach route handlers. Individual JSON handlers still wrap the body with
// http.MaxBytesReader so streaming or unknown-length requests remain capped at
// the same limit during decoding.
func RequestBodyLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hasRequestBody(r) && r.ContentLength > maxJSONRequestBodyBytes {
			WriteError(w, r, http.StatusRequestEntityTooLarge, "request_body_too_large", "request body too large", map[string]any{
				"max_bytes": maxJSONRequestBodyBytes,
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func hasRequestBody(r *http.Request) bool {
	switch r.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		return true
	default:
		return false
	}
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, v any) error {
	return decodeNoTrailingTokens(json.NewDecoder(http.MaxBytesReader(w, r.Body, maxJSONRequestBodyBytes)), v)
}

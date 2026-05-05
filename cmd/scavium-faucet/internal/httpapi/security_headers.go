package httpapi

import "net/http"

const (
	securityContentSecurityPolicy = "default-src 'self'; base-uri 'self'; object-src 'none'; frame-ancestors 'none'; form-action 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-src https://hcaptcha.com https://*.hcaptcha.com https://challenges.cloudflare.com; script-src 'self' https://hcaptcha.com https://*.hcaptcha.com https://js.hcaptcha.com https://challenges.cloudflare.com"
	securityPermissionsPolicy     = "camera=(), microphone=(), geolocation=(), payment=(), usb=(), interest-cohort=()"
)

// SecurityHeadersMiddleware adds conservative browser hardening headers to all
// responses without changing existing API payloads, status codes, or routing.
// HSTS is intentionally left to the TLS-terminating reverse proxy because the
// Go server normally listens on loopback HTTP behind nginx.
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := w.Header()
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("X-Frame-Options", "DENY")
		header.Set("Referrer-Policy", "no-referrer")
		header.Set("Content-Security-Policy", securityContentSecurityPolicy)
		header.Set("Permissions-Policy", securityPermissionsPolicy)
		header.Set("Cross-Origin-Resource-Policy", "same-origin")

		next.ServeHTTP(w, r)
	})
}

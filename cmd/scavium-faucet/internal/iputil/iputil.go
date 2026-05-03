// Package iputil provides safe extraction of the real client IP address
// when the service runs behind a trusted reverse proxy (e.g. nginx).
package iputil

import (
	"net"
	"net/http"
	"strings"
)

// RealIP returns the originating client IP address.
//
// If trustedProxy is non-empty and the RemoteAddr host matches it exactly,
// the function reads the first value from X-Forwarded-For, falling back to
// X-Real-IP, and finally to RemoteAddr.
//
// When trustedProxy is empty, or the request does not come from the trusted
// proxy, RemoteAddr is returned as-is (host portion only).
//
// Only the immediate upstream proxy is trusted; header chaining is not followed
// to prevent IP spoofing.
func RealIP(r *http.Request, trustedProxy string) string {
	remoteHost := hostOnly(r.RemoteAddr)

	if trustedProxy != "" && remoteHost == trustedProxy {
		if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
			// X-Forwarded-For may contain a comma-separated list; take the first entry.
			if idx := strings.IndexByte(xff, ','); idx >= 0 {
				xff = xff[:idx]
			}
			if ip := strings.TrimSpace(xff); ip != "" {
				return ip
			}
		}
		if xri := strings.TrimSpace(r.Header.Get("X-Real-IP")); xri != "" {
			return xri
		}
	}

	return remoteHost
}

// hostOnly returns the host part of a host:port address.
// If the address has no port, it is returned unchanged.
func hostOnly(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		// No port component; return as-is.
		return addr
	}
	return host
}

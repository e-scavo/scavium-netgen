package iputil

import (
	"net/http"
	"testing"
)

func newRequest(remoteAddr, xff, xri string) *http.Request {
	r, _ := http.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = remoteAddr
	if xff != "" {
		r.Header.Set("X-Forwarded-For", xff)
	}
	if xri != "" {
		r.Header.Set("X-Real-IP", xri)
	}
	return r
}

func TestRealIPNoTrustedProxy(t *testing.T) {
	r := newRequest("1.2.3.4:50000", "9.9.9.9", "8.8.8.8")
	got := RealIP(r, "")
	if got != "1.2.3.4" {
		t.Fatalf("got %q, want 1.2.3.4", got)
	}
}

func TestRealIPTrustedProxyUsesXFF(t *testing.T) {
	r := newRequest("127.0.0.1:80", "5.5.5.5", "")
	got := RealIP(r, "127.0.0.1")
	if got != "5.5.5.5" {
		t.Fatalf("got %q, want 5.5.5.5", got)
	}
}

func TestRealIPTrustedProxyXFFMultipleEntries(t *testing.T) {
	r := newRequest("127.0.0.1:80", "5.5.5.5, 10.0.0.1, 192.168.1.1", "")
	got := RealIP(r, "127.0.0.1")
	// Only the first (leftmost) entry is returned.
	if got != "5.5.5.5" {
		t.Fatalf("got %q, want 5.5.5.5", got)
	}
}

func TestRealIPTrustedProxyFallsBackToXRealIP(t *testing.T) {
	r := newRequest("127.0.0.1:80", "", "6.6.6.6")
	got := RealIP(r, "127.0.0.1")
	if got != "6.6.6.6" {
		t.Fatalf("got %q, want 6.6.6.6", got)
	}
}

func TestRealIPTrustedProxyFallsBackToRemoteAddr(t *testing.T) {
	r := newRequest("127.0.0.1:80", "", "")
	got := RealIP(r, "127.0.0.1")
	if got != "127.0.0.1" {
		t.Fatalf("got %q, want 127.0.0.1", got)
	}
}

func TestRealIPUntrustedProxyIgnoresHeaders(t *testing.T) {
	// RemoteAddr is 10.0.0.5, trustedProxy is 127.0.0.1 → mismatch.
	r := newRequest("10.0.0.5:9999", "attacker-ip", "attacker-ip")
	got := RealIP(r, "127.0.0.1")
	if got != "10.0.0.5" {
		t.Fatalf("got %q, want 10.0.0.5 (untrusted proxy should be ignored)", got)
	}
}

func TestRealIPRemoteAddrWithoutPort(t *testing.T) {
	r := newRequest("203.0.113.7", "", "")
	got := RealIP(r, "")
	if got != "203.0.113.7" {
		t.Fatalf("got %q, want 203.0.113.7", got)
	}
}

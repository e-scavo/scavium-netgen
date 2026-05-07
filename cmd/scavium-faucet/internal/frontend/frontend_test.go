package frontend

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerServesIndexHTML(t *testing.T) {
	h := Handler()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Fatalf("content-type = %q, want text/html", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "SCAVIUM Faucet") {
		t.Fatal("index.html not served: missing expected content")
	}
	if !strings.Contains(body, "<script src=\"/static/faucet.js\" defer></script>") {
		t.Fatal("index.html missing external faucet.js script reference")
	}
	if strings.Contains(body, "onclick=") {
		t.Fatal("index.html should not contain inline event handlers")
	}
	for _, want := range []string{"Check Eligibility", "View Address History", "#privacy", "#terms", `aria-live="polite"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("index.html missing Phase 26 UX marker %q", want)
		}
	}
}

func TestHandlerUnknownPathFallsBackToIndex(t *testing.T) {
	h := Handler()

	req := httptest.NewRequest(http.MethodGet, "/some/unknown/path", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "SCAVIUM Faucet") {
		t.Fatal("fallback to index.html failed: missing expected content")
	}
}

func TestHandlerServesStaticJS(t *testing.T) {
	h := Handler()

	req := httptest.NewRequest(http.MethodGet, "/static/faucet.js", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "addEventListener") {
		t.Fatal("faucet.js missing expected event listener logic")
	}
	if !strings.Contains(body, "Idempotency-Key") {
		t.Fatal("faucet.js missing Idempotency-Key header")
	}
	if !strings.Contains(body, "/api/v1/claim") {
		t.Fatal("faucet.js missing claim endpoint usage")
	}
	if !strings.Contains(body, "Token catalog unavailable") {
		t.Fatal("faucet.js missing token catalog fallback UX")
	}
	if !strings.Contains(body, "formatDecimalAmount") {
		t.Fatal("faucet.js missing token amount display formatting")
	}
	for _, want := range []string{"/api/v1/address/", "loadAddressStatus", "loadAddressHistory", "txExplorerHref", "validExplorerTemplate", "isAbsoluteHTTPURLTemplate", "Loading address history", "data.status || data.mode"} {
		if !strings.Contains(body, want) {
			t.Fatalf("faucet.js missing Phase 26 UX logic %q", want)
		}
	}
}

func TestFrontendExplorerLinksRequireAbsoluteSafeTemplates(t *testing.T) {
	h := Handler()

	req := httptest.NewRequest(http.MethodGet, "/static/faucet.js", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	body := rec.Body.String()
	for _, want := range []string{
		`/^https?:\/\//i.test`,
		`new URL(template.replace("{txHash}"`,
		`new URL(href)`,
		`/^0x[0-9a-fA-F]{64}$/.test`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("faucet.js missing safe explorer guard %q", want)
		}
	}
	if strings.Contains(body, `new URL(href, window.location.origin)`) {
		t.Fatal("faucet.js must not resolve explorer transaction links relative to the faucet origin")
	}
}

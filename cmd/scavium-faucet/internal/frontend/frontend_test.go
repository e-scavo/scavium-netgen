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
}

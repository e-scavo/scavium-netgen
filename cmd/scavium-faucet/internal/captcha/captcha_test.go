package captcha

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDisabledAlwaysPasses(t *testing.T) {
	v := Disabled{}
	dec, err := v.Verify(context.Background(), "", "1.2.3.4")
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !dec.Passed {
		t.Fatalf("Passed = false, want true")
	}
	if dec.Reason == "" {
		t.Fatal("Reason is empty")
	}
}

func TestDevAlwaysPassPassesOnDevBypass(t *testing.T) {
	v := DevAlwaysPass{}
	dec, err := v.Verify(context.Background(), "dev-bypass", "1.2.3.4")
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !dec.Passed {
		t.Fatalf("Passed = false, want true")
	}
}

func TestDevAlwaysPassFailsOnOtherToken(t *testing.T) {
	v := DevAlwaysPass{}
	dec, err := v.Verify(context.Background(), "bad-token", "1.2.3.4")
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if dec.Passed {
		t.Fatalf("Passed = true, want false")
	}
	if dec.Reason == "" {
		t.Fatal("Reason is empty on failure")
	}
}

func TestDevAlwaysPassFailsOnEmptyToken(t *testing.T) {
	v := DevAlwaysPass{}
	dec, err := v.Verify(context.Background(), "", "")
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if dec.Passed {
		t.Fatalf("Passed = true, want false")
	}
}

func TestHTTPVerifierPasses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		if r.FormValue("secret") != "test-secret" {
			t.Errorf("secret = %q, want test-secret", r.FormValue("secret"))
		}
		if r.FormValue("response") != "valid-token" {
			t.Errorf("response = %q, want valid-token", r.FormValue("response"))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
	}))
	defer srv.Close()

	v := NewHTTPVerifier(srv.URL, "test-secret")
	dec, err := v.Verify(context.Background(), "valid-token", "1.2.3.4")
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !dec.Passed {
		t.Fatalf("Passed = false, want true")
	}
}

func TestHTTPVerifierFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success":     false,
			"error-codes": []string{"invalid-input-response"},
		})
	}))
	defer srv.Close()

	v := NewHTTPVerifier(srv.URL, "test-secret")
	dec, err := v.Verify(context.Background(), "bad-token", "1.2.3.4")
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if dec.Passed {
		t.Fatalf("Passed = true, want false")
	}
	if dec.Reason != "invalid-input-response" {
		t.Fatalf("Reason = %q, want invalid-input-response", dec.Reason)
	}
}

func TestHTTPVerifierEmptyTokenFails(t *testing.T) {
	v := NewHTTPVerifier("http://should-not-be-called.test", "secret")
	dec, err := v.Verify(context.Background(), "   ", "")
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if dec.Passed {
		t.Fatalf("Passed = true on empty token, want false")
	}
}

func TestHTTPVerifierSendsRemoteIP(t *testing.T) {
	var gotRemoteIP string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		gotRemoteIP = r.FormValue("remoteip")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
	}))
	defer srv.Close()

	v := NewHTTPVerifier(srv.URL, "s")
	_, _ = v.Verify(context.Background(), "tok", "192.168.1.1")
	if gotRemoteIP != "192.168.1.1" {
		t.Fatalf("remoteip = %q, want 192.168.1.1", gotRemoteIP)
	}
}

package captcha

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"scavium-netgen/cmd/scavium-faucet/internal/domain"
)

// Disabled always passes.  Use it when captcha is not configured.
type Disabled struct{}

func (Disabled) Verify(_ context.Context, _ string, _ string) (domain.CaptchaDecision, error) {
	return domain.CaptchaDecision{Passed: true, Reason: "captcha disabled"}, nil
}

// DevAlwaysPass passes when the token equals "dev-bypass", otherwise fails.
// Useful in development environments to test the failure path with any other value.
type DevAlwaysPass struct{}

func (DevAlwaysPass) Verify(_ context.Context, token string, _ string) (domain.CaptchaDecision, error) {
	if token == "dev-bypass" {
		return domain.CaptchaDecision{Passed: true, Reason: "dev bypass"}, nil
	}
	return domain.CaptchaDecision{Passed: false, Reason: "dev mode: token must be 'dev-bypass'"}, nil
}

// HTTPVerifier calls an hCaptcha/reCAPTCHA/Turnstile-compatible verify endpoint.
// The endpoint must accept POST application/x-www-form-urlencoded with fields
// "secret" and "response" (and optionally "remoteip"), and return JSON with at
// least a "success" boolean field.
type HTTPVerifier struct {
	VerifyURL  string
	Secret     string
	HTTPClient *http.Client
}

// NewHTTPVerifier creates a new HTTPVerifier with a sensible timeout.
func NewHTTPVerifier(verifyURL, secret string) *HTTPVerifier {
	return &HTTPVerifier{
		VerifyURL: verifyURL,
		Secret:    secret,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type verifyResponse struct {
	Success    bool     `json:"success"`
	ErrorCodes []string `json:"error-codes"`
}

func (v *HTTPVerifier) Verify(ctx context.Context, token string, remoteIP string) (domain.CaptchaDecision, error) {
	if strings.TrimSpace(token) == "" {
		return domain.CaptchaDecision{Passed: false, Reason: "missing captcha token"}, nil
	}

	formData := url.Values{
		"secret":   {v.Secret},
		"response": {token},
	}
	if remoteIP != "" {
		formData.Set("remoteip", remoteIP)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.VerifyURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return domain.CaptchaDecision{}, fmt.Errorf("captcha: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := v.HTTPClient.Do(req)
	if err != nil {
		return domain.CaptchaDecision{}, fmt.Errorf("captcha: http: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return domain.CaptchaDecision{}, fmt.Errorf("captcha: read body: %w", err)
	}

	var vr verifyResponse
	if err := json.Unmarshal(body, &vr); err != nil {
		return domain.CaptchaDecision{}, fmt.Errorf("captcha: decode response: %w", err)
	}

	if vr.Success {
		return domain.CaptchaDecision{Passed: true, Reason: "captcha passed"}, nil
	}

	reason := "captcha failed"
	if len(vr.ErrorCodes) > 0 {
		reason = strings.Join(vr.ErrorCodes, ", ")
	}
	return domain.CaptchaDecision{Passed: false, Reason: reason}, nil
}

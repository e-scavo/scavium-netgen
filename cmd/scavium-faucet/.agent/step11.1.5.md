# Step 11.1.5 — Claim Request Signals: Captcha, IP, Fingerprint, Risk

Recommended executor: Codex in VSCode.

## Goal

Connect already-created security packages to the public claim flow without overbuilding the professional anti-abuse layer.

## Required reads

- `cmd/scavium-faucet/internal/httpapi/handler.go`
- `cmd/scavium-faucet/internal/captcha/captcha.go`
- `cmd/scavium-faucet/internal/abuse/abuse.go`
- `cmd/scavium-faucet/internal/iputil/iputil.go`
- `cmd/scavium-faucet/internal/faucet/service.go`
- persistent service from Step 11.1.2

## Required implementation

1. Extend HTTP claim request parsing to accept optional public fields:

```json
{
  "address": "0x...",
  "captcha_token": "...",
  "fingerprint": "..."
}
```

2. Extend internal `faucet.ClaimRequest` with:

- `RemoteIP string`
- `UserAgent string`
- `CaptchaToken string`
- `Fingerprint string`

3. In `httpapi`, derive real client IP using existing `iputil` and trusted proxy configuration if already available. If trusted proxy is not accessible in handler dependencies, add a minimal dependency field.

4. In persistent `CreateClaim`:

- verify captcha when captcha provider is not disabled.
- evaluate risk engine if configured.
- apply persistent rate limit keys using IP/address/fingerprint where available.
- keep behavior deterministic in tests.

5. Do not block local development when captcha provider is disabled.

6. Add tests:

- disabled captcha allows claims.
- failed captcha rejects claim.
- repeated IP/address hits rate limit.
- fingerprint rate limit is enforced when fingerprint is supplied.

## Boundaries

- Do not implement external VPN/proxy APIs here.
- Do not add frontend UI requirements unless required for tests.
- Do not modify docs.

## Commands

```bash
git checkout -b faucet/step11.1.5-claim-security-signals
git status --short
go test ./cmd/scavium-faucet/internal/httpapi ./cmd/scavium-faucet/internal/faucet ./cmd/scavium-faucet/internal/captcha ./cmd/scavium-faucet/internal/abuse ./cmd/scavium-faucet/internal/iputil
go test ./...
```

## Git finalization

```bash
git status --short
git add cmd/scavium-faucet/internal/httpapi cmd/scavium-faucet/internal/faucet cmd/scavium-faucet/internal/captcha cmd/scavium-faucet/internal/abuse cmd/scavium-faucet/internal/iputil
git commit -m "Wire claim captcha and anti-abuse signals"
```

## Expected output

- Files read.
- Files modified.
- Tests run and results.
- Commit hash.

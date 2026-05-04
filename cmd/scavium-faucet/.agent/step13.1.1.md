# Step 13.1.1 — Precise claim error mapping

## Recommended executor

Codex in VSCode.

## Goal

Replace generic `500 claim_unavailable` for expected claim failures with precise normalized HTTP errors.

## Scope

Focus on `POST /api/v1/claim` and `/api/v1/faucet/claim`.

Expected claim failures should map to clear status/code pairs, for example:

| Condition | Suggested HTTP status | Suggested code |
|---|---:|---|
| invalid address | 400 | `invalid_address` |
| faucet paused / maintenance / disabled | 503 | `faucet_unavailable` or `faucet_paused` |
| cooldown active | 429 | `cooldown_active` |
| IP/address/fingerprint rate limit exceeded | 429 | `rate_limit_exceeded` |
| captcha missing/failed | 403 or 400 | `captcha_failed` |
| risk rejection | 403 | `risk_rejected` |
| idempotency conflict if any | 409 | `idempotency_conflict` |
| unexpected store/queue error | 500 | `claim_unavailable` |

Use actual existing sentinel errors/types if present. If they are missing, add small internal/domain or faucet-level sentinel errors/wrappers with tests.

## Files likely to modify

- `cmd/scavium-faucet/internal/faucet/persistent_service.go`
- `cmd/scavium-faucet/internal/faucet/persistent_service_test.go`
- `cmd/scavium-faucet/internal/httpapi/handler.go`
- `cmd/scavium-faucet/internal/httpapi/handler_test.go`
- possibly `cmd/scavium-faucet/internal/domain/interfaces.go` or a focused error file

## Validation

Run:

```bash
gofmt -w <go files changed>
go test ./cmd/scavium-faucet/internal/faucet ./cmd/scavium-faucet/internal/httpapi
go test ./cmd/scavium-faucet/...
go test ./...
go build ./cmd/scavium-faucet
```

## Hard constraints

- Do not change public success response shape.
- Do not modify docs.
- Do not touch deployment files.
- Do not read or use `cmd/scavium-faucet-v0`.
- Keep changes minimal and focused on error semantics.

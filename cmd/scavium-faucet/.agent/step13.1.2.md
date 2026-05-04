# Step 13.1.2 — Configurable safe CORS

## Recommended executor

Codex in VSCode.

## Goal

Add safe, explicit CORS support for public API routes.

## Requirements

- Add configuration for allowed origins.
- Do not allow all origins by default.
- Empty config should mean no CORS headers, not wildcard.
- Support one or more exact origins.
- Handle `OPTIONS` preflight safely.
- Apply only to HTTP API/admin routes or entire handler if simpler, but do not weaken security.
- Include `Vary: Origin` when appropriate.
- Include safe methods/headers:
  - Methods: `GET`, `POST`, `OPTIONS`
  - Headers: `Content-Type`, `Idempotency-Key`, `Authorization`, `X-Request-ID`
- Do not expose secrets or internal headers.

## Suggested config names

Use actual naming style from `config.go`.

Possible env var:

```text
SCAVIUM_FAUCET_CORS_ALLOWED_ORIGINS
```

Comma-separated exact origins.

## Files likely to modify

- `cmd/scavium-faucet/internal/config/config.go`
- `cmd/scavium-faucet/internal/config/config_test.go`
- `cmd/scavium-faucet/internal/httpapi/handler.go` or a new focused middleware file
- `cmd/scavium-faucet/internal/httpapi/handler_test.go`
- `cmd/scavium-faucet/internal/app/app.go`

## Validation

Run:

```bash
gofmt -w <go files changed>
go test ./cmd/scavium-faucet/internal/config ./cmd/scavium-faucet/internal/httpapi ./cmd/scavium-faucet/internal/app
go test ./cmd/scavium-faucet/...
go test ./...
go build ./cmd/scavium-faucet
```

## Hard constraints

- Do not default to `*`.
- Do not modify docs.
- Do not touch deployment files.
- Do not read or use `cmd/scavium-faucet-v0`.

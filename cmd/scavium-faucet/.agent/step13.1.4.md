# Step 13.1.4 — Minimal observability hardening

## Recommended executor

Codex in VSCode.

## Goal

Add minimal production-safe observability without introducing heavy dependencies.

## Requirements

At minimum:

- Structured-ish request logging for HTTP requests.
- Include request ID, method, path, status, duration, remote IP.
- Do not log request bodies.
- Do not log secrets.
- Keep logs compatible with systemd/journald.
- Add tests for middleware behavior where practical.
- Keep `/metrics` only if already implemented; do not add Prometheus dependency unless trivial and justified.

Optional if low risk:

- Add simple in-process counters endpoint protected or localhost-only if current architecture supports it.
- Otherwise leave metrics as documented future work.

## Files likely to modify

- `cmd/scavium-faucet/internal/httpapi/handler.go`
- possibly new `cmd/scavium-faucet/internal/httpapi/logging.go`
- `cmd/scavium-faucet/internal/httpapi/handler_test.go`
- possibly `cmd/scavium-faucet/internal/app/app.go`

## Validation

Run:

```bash
gofmt -w <go files changed>
go test ./cmd/scavium-faucet/internal/httpapi ./cmd/scavium-faucet/internal/app
go test ./cmd/scavium-faucet/...
go test ./...
go build ./cmd/scavium-faucet
```

## Hard constraints

- Do not log secrets: private key, admin token, captcha secret, request body.
- Do not modify docs.
- Do not touch deployment files.
- Do not read or use `cmd/scavium-faucet-v0`.

# Phase 11 — Runtime Wiring & Persistence Activation

## Purpose

Close the gap found after the full ZIP audit: the SQLite store, queue, worker, sender, watcher, captcha/risk packages and admin handler exist, but the production runtime still starts with `faucet.NewInMemoryReadService(cfg)`.

This phase must convert `cmd/scavium-faucet` into a real persistent faucet runtime without touching VPS/nginx/systemd/certbot deployment.

## Source of truth

- Current repository ZIP only.
- Functional target: `docs/scavium_faucet_public_features.md`.
- Runtime gap found in:
  - `cmd/scavium-faucet/internal/app/app.go`
  - `cmd/scavium-faucet/internal/faucet/service.go`
  - `cmd/scavium-faucet/internal/store/sqlite/store.go`
  - `cmd/scavium-faucet/internal/worker/worker.go`
  - `cmd/scavium-faucet/internal/chain/*`
  - `cmd/scavium-faucet/internal/ready/ready.go`

## Hard boundaries

- Do not modify deployment docs, nginx, certbot or VPS files in this phase.
- Do not use `cmd/scavium-faucet-v0` as a source.
- Do not remove the in-memory implementation unless tests require keeping it for unit tests.
- Prefer additive integration and small changes.
- Every step must run `go test ./...` from repo root when possible.
- Every step must report files read, files modified, commands run and test results.

## Execution strategy

Use Copilot CLI first for read-only audits, mechanical inspections and simple grep-based validation. Use Codex in VSCode for structural Go changes, runtime composition, interface changes and tests.

## Final acceptance criteria

- `app.New` no longer uses the in-memory read service by default.
- Runtime opens SQLite using config.
- Runtime applies migrations.
- Claims are persisted in SQL.
- Idempotency is persisted.
- Cooldown and rate limit are enforced through persistent state.
- Claims are enqueued persistently.
- Worker is started by runtime.
- Sender is dry-run in dry-run mode and real RPC signer when dry-run is false.
- Watcher is started when it has a real chain client.
- Admin token from config reaches admin middleware.
- `/ready` checks DB, RPC/wallet where applicable, and queue/store.
- Tests cover runtime persistent claim flow and restart persistence.

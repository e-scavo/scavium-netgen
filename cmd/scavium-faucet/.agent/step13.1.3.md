# Step 13.1.3 — Persistent daily budget enforcement

## Recommended executor

Codex in VSCode.

## Goal

Enforce `SCAVIUM_FAUCET_DAILY_BUDGET_WEI` persistently.

## Requirements

- If daily budget is unset, preserve existing behavior.
- If set, deny claims once the daily budget would be exceeded.
- Use UTC calendar days.
- Enforcement must survive process restarts.
- Prefer using existing claims/transactions schema if sufficient.
- Add a minimal migration only if the current schema cannot answer budget usage safely.
- Avoid race conditions as much as possible for SQLite single-process runtime.
- Return a precise service error that step13.1.1 HTTP mapping can expose as a non-500 error, for example `daily_budget_exceeded`.

## Budget accounting recommendation

For MVP production hardening, count accepted/queued/sending/sent/confirmed claims for the current UTC day by configured amount.

Do not count rejected/failed/cancelled claims unless current state model demands otherwise.

## Files likely to modify

- `cmd/scavium-faucet/internal/faucet/persistent_service.go`
- `cmd/scavium-faucet/internal/faucet/persistent_service_test.go`
- `cmd/scavium-faucet/internal/store/sqlite/store.go`
- `cmd/scavium-faucet/internal/store/sqlite/store_test.go`
- `cmd/scavium-faucet/internal/domain/interfaces.go`
- possibly `cmd/scavium-faucet/migrations/003_daily_budget.sql`

## Validation

Run:

```bash
gofmt -w <go files changed>
go test ./cmd/scavium-faucet/internal/store/sqlite ./cmd/scavium-faucet/internal/faucet ./cmd/scavium-faucet/internal/httpapi
go test ./cmd/scavium-faucet/...
go test ./...
go build ./cmd/scavium-faucet
```

## Hard constraints

- Do not modify docs.
- Do not touch deployment files.
- Do not read or use `cmd/scavium-faucet-v0`.
- Do not add external services.

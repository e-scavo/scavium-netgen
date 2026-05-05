# Step 20.2.0 — SQLite Admin Queue and Claim Control

## Goal

Move admin retry/cancel behavior onto persisted claim/queue state while preserving existing admin endpoint contracts.

## Must read first

- `.agent/rules.md`
- `cmd/scavium-faucet/internal/admin/admin.go`
- `cmd/scavium-faucet/internal/httpapi/handler.go`
- `cmd/scavium-faucet/internal/store/sqlite/store.go`
- `cmd/scavium-faucet/internal/domain/models.go`
- `cmd/scavium-faucet/internal/domain/interfaces.go`
- tests for admin/httpapi/store

## Scope

- Implement SQLite-backed retry for eligible failed/rejected claims.
- Implement SQLite-backed cancel for eligible not-yet-sent claims.
- Keep old error semantics: not found, not retryable, not cancellable.
- Ensure retry returns claim to queued/processable state consistently with worker expectations.
- Ensure cancel does not cancel already sent/confirmed claims.
- Add tests for success and invalid state transitions.
- Update API/runbook/architecture docs.

## Constraints

- No broad worker refactor.
- No public contract changes.
- No leaking addresses/idempotency keys in admin logs.
- Split if this cannot fit into the 24-hour budget.

## Validation

```bash
gofmt -w <go-files-changed>
go test ./cmd/scavium-faucet/internal/store/sqlite/... -count=1 -timeout 300s
go test ./cmd/scavium-faucet/internal/admin/... ./cmd/scavium-faucet/internal/httpapi/... -count=1 -timeout 300s
go test ./... -timeout 300s
make build -B
```

## Delivery

Partial ZIP only; include complete Git commands.

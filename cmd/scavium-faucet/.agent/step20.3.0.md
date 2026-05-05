# Step 20.3.0 — Durable Admin Audit Log

## Goal

Persist admin audit entries to SQLite while preserving safe structured logging and existing `/api/v1/admin/audit` response behavior.

## Must read first

- `.agent/rules.md`
- `cmd/scavium-faucet/internal/admin/admin.go`
- `cmd/scavium-faucet/internal/httpapi/handler.go`
- `cmd/scavium-faucet/internal/store/sqlite/store.go`
- `cmd/scavium-faucet/migrations/*.sql`
- `docs/scavium-faucet/security.md`
- `docs/scavium-faucet/runbook.md`

## Scope

- Add a migration for `admin_audit_logs` if not already present.
- Store action, actor, target, safe detail, timestamp.
- Do not store bearer tokens, raw blocklist values, captcha tokens, request bodies, idempotency keys, or secrets.
- Keep list limits capped.
- Add migration tests and audit read/write tests.
- Update docs.

## Constraints

- No new external audit service.
- No tamper-evident hash chain in this step.
- No roles/2FA in this step.

## Validation

```bash
gofmt -w <go-files-changed>
go test ./cmd/scavium-faucet/internal/store/sqlite/... -count=1 -timeout 300s
go test ./cmd/scavium-faucet/internal/httpapi/... -count=1 -timeout 300s
go test ./... -timeout 300s
make build -B
```

## Delivery

Partial ZIP only; include complete Git commands.

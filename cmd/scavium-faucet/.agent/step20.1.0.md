# Step 20.1.0 — SQLite Admin Read Model Foundation

## Goal

Create the minimal SQLite-backed admin read foundation without changing public API contracts.

## Must read first

- `.agent/rules.md`
- `.agent/commands.md`
- `docs/scavium-faucet/implementation-roadmap-after-phase19.md`
- `docs/scavium_faucet_public_phase-roadmap-post14.md`
- `cmd/scavium-faucet/internal/admin/admin.go`
- `cmd/scavium-faucet/internal/domain/interfaces.go`
- `cmd/scavium-faucet/internal/store/sqlite/store.go`
- `cmd/scavium-faucet/internal/httpapi/handler.go`
- all files under `cmd/scavium-faucet/migrations/`

## Scope

- Add store-level admin-safe claim listing and claim detail methods if current store methods are insufficient.
- Add queue snapshot read methods derived from persisted claim/queue state.
- Do not implement retry/cancel yet.
- Do not change HTTP response shapes unless strictly additive and documented.
- Keep in-memory admin service intact.
- Add tests in `internal/store/sqlite` and/or `internal/admin` as appropriate.
- Update docs to say Phase 20.1 starts SQLite admin reads but control is not yet durable until later steps.

## Constraints

- No new dependencies.
- No schema migration unless a read-only method truly needs it.
- No public endpoint changes.
- Must fit within the renewed 24-hour budget; split before coding if too large.

## Validation

Run:

```bash
gofmt -w <go-files-changed>
go test ./cmd/scavium-faucet/internal/store/sqlite/... -count=1 -timeout 300s
go test ./cmd/scavium-faucet/internal/admin/... ./cmd/scavium-faucet/internal/httpapi/... -count=1 -timeout 300s
go test ./... -timeout 300s
make build -B
```

## Delivery

Return a partial ZIP containing only changed/created files and complete Git commands.

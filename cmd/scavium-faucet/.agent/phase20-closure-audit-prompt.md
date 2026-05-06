# Phase 20 Closure Audit Prompt

Use this prompt to re-audit Phase 20 closure against repository source of truth.

## Objective

Confirm that Phase 20 is fully closed and documentation is aligned with implemented behavior.

## Required checks

1. Verify durable admin state behavior in code:
   - SQLite-backed admin claim/queue reads.
   - SQLite-backed retry/cancel control.
   - Durable admin audit persistence.
   - Persisted admin blocklist and claim-path enforcement.
2. Verify tests cover each closure item and pass.
3. Verify docs do not claim these Phase 20 items are in-memory, in-progress, or deferred.
4. Keep deferred non-Phase-20 items explicit (dynamic config editing, campaigns/allowlist, runtime token mutation, etc.).
5. Report only minimal closure fixes; do not introduce broad refactors.

## Files to inspect first

- cmd/scavium-faucet/internal/admin/admin.go
- cmd/scavium-faucet/internal/store/sqlite/store.go
- cmd/scavium-faucet/internal/faucet/persistent_service.go
- cmd/scavium-faucet/internal/abuse/abuse.go
- cmd/scavium-faucet/migrations/005_admin_audit_logs.sql
- cmd/scavium-faucet/migrations/006_admin_blocklist.sql
- cmd/scavium-faucet/internal/admin/admin_test.go
- cmd/scavium-faucet/internal/store/sqlite/store_test.go
- cmd/scavium-faucet/internal/faucet/persistent_service_test.go
- docs/scavium_faucet_public_phase-roadmap-post14.md
- docs/scavium-faucet/implementation-roadmap-after-phase19.md
- docs/scavium-faucet/architecture.md
- docs/scavium-faucet/api.md
- docs/scavium-faucet/security.md
- docs/scavium-faucet/runbook.md

## Validation commands

```bash
go test ./... -timeout 300s
make build -B
```

## Expected output format

1. Files read
2. Findings (severity ordered)
3. Minimal edits applied (if needed)
4. Validation command results
5. Full git command sequence

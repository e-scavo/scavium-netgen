# Phase 24 — Post-14 Roadmap Closure Audit Prompt

## Context

You are auditing SCAVIUM Faucet after Phase 24. The ZIP/worktree is the only source of truth. The system is production software exposed through nginx to the public Internet, with a Go backend, SQLite persistence, async worker, Besu RPC, admin-protected operational surfaces, and documentation under `docs/scavium-faucet/`.

Phase 24 is a closure audit only. Do not introduce new features, large refactors, new dependencies, public API contract changes, or deployment topology changes unless you find a small mandatory safety/documentation fix.

## Required source files to read

Read these files before concluding:

- `cmd/scavium-faucet/.agent/rules.md`
- `cmd/scavium-faucet/.agent/step24.1.0.md`
- `docs/scavium_faucet_public_phase-roadmap-post14.md`
- `docs/scavium-faucet/implementation-roadmap-after-phase19.md`
- `docs/scavium-faucet/phase24-post14-closure-audit.md`
- `docs/scavium-faucet/index.md`
- `docs/scavium-faucet/api.md`
- `docs/scavium-faucet/architecture.md`
- `docs/scavium-faucet/configuration.md`
- `docs/scavium-faucet/runbook.md`
- `docs/scavium-faucet/security.md`
- `docs/scavium-faucet/deployment-rollback.md`
- `docs/scavium-faucet/deployment/scavium-faucet.env.example`
- `scripts/scavium-faucet-backup.sh`
- `scripts/scavium-faucet-restore.sh`
- `scripts/scavium-faucet-operator-smoke.sh`

Then inspect the relevant code paths:

- `cmd/scavium-faucet/internal/httpapi/handler.go`
- `cmd/scavium-faucet/internal/observability/metrics.go`
- `cmd/scavium-faucet/internal/app/app.go`
- `cmd/scavium-faucet/internal/config/config.go`
- `cmd/scavium-faucet/internal/chain/*.go`
- `cmd/scavium-faucet/internal/worker/worker.go`
- `cmd/scavium-faucet/internal/store/sqlite/*.go`
- `cmd/scavium-faucet/migrations/*.sql`

## Audit checklist

Verify that Phases 15 through 23 are closed exactly as documented:

1. Phase 15: captcha, abuse signals, progressive enforcement, persisted abuse data, and public-contract-preserving rejection behavior.
2. Phase 16: structured logs, request/correlation IDs, admin JSON metrics, health/readiness enrichment.
3. Phase 17: multi-token support, public catalog, token-aware claim validation/rate limits/metrics/frontend.
4. Phase 18: production-safe in-memory admin controls and runtime visibility baseline.
5. Phase 19 and 19.6: HTTP hardening, request limits, security headers, graceful shutdown, post-audit hardening fixes.
6. Phase 20: SQLite-backed admin claim/queue/control/audit/blocklist state and claim-path blocklist enforcement.
7. Phase 21: admin-protected Prometheus export, bounded metrics labels, worker/watcher/queue/abuse/blocklist metrics, alerting runbook, smoke script.
8. Phase 22: startup-only RPC failover, chain-ID validation, admin wallet visibility with native/ERC20 balance status and pending nonce.
9. Phase 23: backup/restore scripts, manifest verification, unsafe tar-entry rejection, WAL/SHM companion handling, wallet refill/rotation/rollback runbooks.
10. Phase 24: roadmap and documentation closure, deferred backlog handoff, and this audit prompt.

## Deferred/backlog boundary

Confirm the remaining items are not accidental gaps in Phases 15 through 24. They are intentionally moved to the broader feature backlog:

- public address history and OpenAPI contract completion
- frontend polish and public terms/privacy links
- advanced anti-abuse/risk scoring/manual review
- dynamic durable budget/config editing
- campaigns/allowlists/invitation codes/CSV export
- wallet integration/login/signature challenge
- multi-network, multi-wallet, HA, distributed locks, PostgreSQL migration, automatic treasury refill, webhooks, tamper-evident audit chain, full reporting suite

## Required validation commands

Run:

```bash
go test ./... -timeout 300s
make build -B
bash -n scripts/*.sh
```

Also run non-mutating operational checks when environment allows. Do not execute restore plan with a placeholder path; it requires a real backup bundle. For a self-contained local closure check, create a disposable bundle first:

```bash
./scripts/scavium-faucet-backup.sh --plan

TMP_DIR="$(mktemp -d)"
printf 'phase24 restore plan fixture\n' > "$TMP_DIR/scavium-faucet.db"
SCAVIUM_FAUCET_DATABASE_PATH="$TMP_DIR/scavium-faucet.db" \
SCAVIUM_FAUCET_BACKUP_DIR="$TMP_DIR/backups" \
SCAVIUM_FAUCET_BACKUP_ID="phase24-restore-plan-check" \
./scripts/scavium-faucet-backup.sh --execute

SCAVIUM_FAUCET_RESTORE_BUNDLE="$TMP_DIR/backups/scavium-faucet-backup-phase24-restore-plan-check.tar.gz" \
./scripts/scavium-faucet-restore.sh --plan
rm -rf "$TMP_DIR"
```

If an operator uses an existing production backup instead, set `SCAVIUM_FAUCET_RESTORE_BUNDLE` to that actual `.tar.gz` file. A missing bundle path is an environment-input failure, not evidence of an unimplemented Phase 23/24 feature.

If validation fails, classify the failure as one of:

- code/runtime defect to fix immediately
- missing local dependency/toolchain issue
- environment-specific operator input required

## Output expected

Produce a concise closure finding with:

- files read
- validation results
- any mandatory fixes applied
- confirmation that the post-Phase-14 roadmap is closed, or a precise list of blocking gaps
- no full-project ZIP; if changes are needed, only a partial ZIP containing changed/new files

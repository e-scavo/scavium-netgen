# Step 11.1.1 — Configurable SQLite Runtime Path & Store Lifecycle

Recommended executor: Codex in VSCode.

## Goal

Add explicit runtime configuration for SQLite persistence and make the app capable of owning resources that must be closed cleanly.

## Scope

Implement only the configuration and lifecycle foundation. Do not yet replace the faucet service if that becomes too large; that is Step 11.1.2.

## Required changes

1. Add configuration fields to `cmd/scavium-faucet/internal/config/config.go`:
   - `DatabasePath string`
   - Optional worker/watcher knobs only if already straightforward:
     - `WorkerEnabled bool`
     - `WorkerPollSeconds int`
     - `WatcherEnabled bool`
     - `WatcherPollSeconds int`
     - `MinConfirmations uint64`

2. Add env vars with safe defaults:
   - `SCAVIUM_FAUCET_DATABASE_PATH`
   - default: `cmd/scavium-faucet/data/scavium-faucet.db` for local dev, or another repo-local safe dev path if preferred.
   - `SCAVIUM_FAUCET_WORKER_ENABLED`
   - default: `true`
   - `SCAVIUM_FAUCET_WATCHER_ENABLED`
   - default: `true` when not dry-run, acceptable default `false` in dry-run.

3. Validate:
   - database path cannot be empty.
   - worker poll seconds must be positive when configured.
   - watcher poll seconds must be positive when configured.

4. Extend `App` lifecycle in `cmd/scavium-faucet/internal/app/app.go`:
   - add `Close(context.Context) error` or `Close() error`.
   - prepare for holding DB store and goroutine cancellation in later steps.
   - maintain compatibility with existing `main.go` or update `main.go` minimally if needed.

5. Add or update tests:
   - config defaults include database path.
   - env override works.
   - invalid DB path fails validation.

## Important boundaries

- Do not modify documentation.
- Do not read or copy from `cmd/scavium-faucet-v0`.
- Do not make deployment assumptions.

## Commands

```bash
git checkout -b faucet/step11.1.1-config-db-lifecycle
git status --short
go test ./cmd/scavium-faucet/internal/config ./cmd/scavium-faucet/internal/app
go test ./...
```

## Git finalization

```bash
git status --short
git add cmd/scavium-faucet/internal/config cmd/scavium-faucet/internal/app cmd/scavium-faucet/main.go
git commit -m "Add faucet database config and app lifecycle"
```

## Expected output

- Files read.
- Files modified.
- Test results.
- Commit hash.

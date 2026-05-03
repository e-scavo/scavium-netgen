# Step 11.1.4 — Readiness, Runtime Health & Queue Checks

Recommended executor: Codex in VSCode. Copilot CLI may be used first to inspect references.

## Goal

Replace placeholder readiness with meaningful runtime checks for DB, queue/store, RPC/wallet where applicable.

## Required implementation

1. Add readiness check constructors, preferably in `cmd/scavium-faucet/internal/ready` or `internal/app`:

   - DB check: `SELECT 1` or a store ping.
   - Queue/store check: lightweight query that confirms queue tables are accessible.
   - RPC check: only when not dry-run and RPC client exists.
   - Wallet/balance check: only when not dry-run and signer/client exist.

2. Update `app.New` to pass real checks to `httpapi.NewHandler`.

3. Preserve local dry-run behavior:

   - dry-run should not be degraded only because no RPC/private key is configured.
   - readiness should still include DB/queue checks.

4. Add tests:

   - dry-run readiness is OK with temp SQLite and no RPC.
   - missing/broken DB readiness degrades.
   - check names remain stable: `db`, `queue`, optionally `rpc`, `wallet`.

## Commands

```bash
git checkout -b faucet/step11.1.4-readiness-runtime-checks
git status --short
go test ./cmd/scavium-faucet/internal/ready ./cmd/scavium-faucet/internal/app ./cmd/scavium-faucet/internal/httpapi
go test ./...
```

## Git finalization

```bash
git status --short
git add cmd/scavium-faucet/internal/ready cmd/scavium-faucet/internal/app cmd/scavium-faucet/internal/httpapi cmd/scavium-faucet/internal/store/sqlite
git commit -m "Add real faucet runtime readiness checks"
```

## Expected output

- Files read.
- Files modified.
- Tests run and results.
- Commit hash.

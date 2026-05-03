# Step 11.1.0 — Runtime Gap Confirmation & Minimal Integration Plan

Recommended executor: Copilot CLI first. If Copilot CLI cannot inspect enough context, use Codex in VSCode.

## Goal

Confirm the exact runtime wiring gap in the current working tree and produce a small implementation plan before editing code.

## Required reads

Read these files fully before proposing changes:

- `cmd/scavium-faucet/internal/app/app.go`
- `cmd/scavium-faucet/internal/config/config.go`
- `cmd/scavium-faucet/internal/faucet/service.go`
- `cmd/scavium-faucet/internal/domain/interfaces.go`
- `cmd/scavium-faucet/internal/store/sqlite/store.go`
- `cmd/scavium-faucet/internal/worker/worker.go`
- `cmd/scavium-faucet/internal/chain/chain.go`
- `cmd/scavium-faucet/internal/chain/sender.go`
- `cmd/scavium-faucet/internal/chain/signer.go`
- `cmd/scavium-faucet/internal/chain/watcher.go`
- `cmd/scavium-faucet/internal/ready/ready.go`
- `cmd/scavium-faucet/internal/httpapi/handler.go`
- `cmd/scavium-faucet/main.go`

## Instructions

1. Do not edit files in this step unless a test-only compile blocker is discovered and trivial.
2. Confirm whether `app.New` still uses `faucet.NewInMemoryReadService`.
3. Confirm which store methods already exist for idempotency, queue, rate limit, watcher and claims.
4. Confirm which HTTP request fields are currently available to `CreateClaim`.
5. Confirm whether admin token is passed to `httpapi.NewHandler`.
6. Confirm whether readiness checks are real or stubbed.
7. Return a short list of files that must be modified in the following steps.

## Commands

```bash
git status --short
grep -R "NewInMemoryReadService" -n cmd/scavium-faucet || true
grep -R "AdminToken" -n cmd/scavium-faucet/internal || true
grep -R "type Config struct" -n cmd/scavium-faucet/internal/config/config.go
grep -R "func Open" -n cmd/scavium-faucet/internal/store/sqlite/store.go
grep -R "CreateClaimWithIdempotency" -n cmd/scavium-faucet || true
go test ./...
```

## Git commands

No commit required if no files are modified.

If any trivial fix is made:

```bash
git checkout -b faucet/step11.1.0-runtime-gap-confirmation
git add <files>
git commit -m "Audit faucet runtime wiring gap"
```

## Expected output

- Files read.
- Files to modify in next steps.
- Confirmation of the exact persistent runtime gap.
- Test command result.

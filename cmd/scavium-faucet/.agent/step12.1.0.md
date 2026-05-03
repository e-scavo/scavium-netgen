# Step 12.1.0 — Documentation audit plan

## Recommended executor

Copilot Chat in VSCode.

## Goal

Produce a documentation-only audit plan for aligning `docs/scavium-faucet` with the real post-Phase 11 runtime.

## Instructions

Read:

- `cmd/scavium-faucet/internal/app/app.go`
- `cmd/scavium-faucet/internal/config/config.go`
- `cmd/scavium-faucet/internal/store/sqlite/store.go`
- `cmd/scavium-faucet/internal/faucet/persistent_service.go`
- `cmd/scavium-faucet/internal/worker/worker.go`
- `cmd/scavium-faucet/internal/chain/sender.go`
- `cmd/scavium-faucet/internal/chain/watcher.go`
- `cmd/scavium-faucet/internal/ready/ready.go`
- `cmd/scavium-faucet/internal/httpapi/handler.go`
- `cmd/scavium-faucet/internal/captcha/captcha.go`
- `cmd/scavium-faucet/internal/abuse/abuse.go`
- `cmd/scavium-faucet/migrations/`
- `docs/scavium-faucet/*.md`
- `docs/scavium_faucet_public_features.md`

Do not edit files in this step.

## Required output

Return:

1. Files read.
2. Documentation files that need edits.
3. Outdated statements found.
4. Correct post-Phase 11 statements to use.
5. Proposed order of documentation edits.
6. Confirmation that no code should be modified.

## Hard constraints

- Do not modify code.
- Do not modify docs in this step.
- Do not read or use `cmd/scavium-faucet-v0`.

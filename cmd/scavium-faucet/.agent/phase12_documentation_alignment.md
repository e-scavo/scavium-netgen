# Phase 12 — Documentation Alignment after Runtime Persistence Activation

## Purpose

Align all SCAVIUM Faucet documentation with the real post-Phase 11 implementation.

This phase is documentation-only.

## Source of truth

The current repository state is the only source of truth.

Primary implementation areas to inspect:

- `cmd/scavium-faucet/main.go`
- `cmd/scavium-faucet/internal/app/app.go`
- `cmd/scavium-faucet/internal/config/config.go`
- `cmd/scavium-faucet/internal/faucet/`
- `cmd/scavium-faucet/internal/store/sqlite/`
- `cmd/scavium-faucet/internal/worker/`
- `cmd/scavium-faucet/internal/chain/`
- `cmd/scavium-faucet/internal/ready/`
- `cmd/scavium-faucet/internal/httpapi/`
- `cmd/scavium-faucet/internal/captcha/`
- `cmd/scavium-faucet/internal/abuse/`
- `cmd/scavium-faucet/migrations/`
- `docs/scavium_faucet_public_features.md`
- `docs/scavium-faucet/`

## Critical correction

Documentation generated before Phase 11 may still claim that the runtime uses:

- in-memory claim state
- stub readiness checks
- unwired SQLite
- unwired captcha
- unwired admin token
- unwired queue/worker/watcher

Those statements are outdated after Phase 11 and must be corrected if the code confirms the runtime is now wired.

## Documentation target

Update documentation so it accurately describes:

- SQLite-backed runtime
- automatic migrations
- persistent claim creation
- persistent queue
- worker runtime
- dry-run sender
- real sender in non-dry mode
- watcher behavior
- real readiness checks
- admin token wiring
- captcha and risk/rate-limit signal wiring
- WAL + busy timeout behavior
- remaining known limitations, if any

## Rules

- Do not modify Go code.
- Do not modify deployment scripts unless documentation files explicitly contain deployment guidance.
- Do not read, copy, or derive from `cmd/scavium-faucet-v0`.
- Preserve existing documentation structure and narrative continuity.
- Do not rewrite docs from scratch unless the current file is clearly small and purpose-specific.
- Prefer incremental edits.
- Keep Markdown readable and consistent.
- If a statement cannot be verified from code, mark it as a known limitation or omit it.

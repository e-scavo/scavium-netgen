# Step 13.1.5 — Final hardening audit

## Recommended executor

Copilot Chat in VSCode.

## Goal

Audit Phase 13 after implementation. Do not edit files unless the issue is a tiny typo in comments.

## Checks

Verify:

- Expected claim failures no longer return generic 500.
- Captcha failures have specific status/code.
- Rate limit and cooldown failures have specific status/code.
- Risk rejection has specific status/code.
- Daily budget has specific status/code and persistent enforcement.
- CORS is safe by default and configurable by exact origin.
- Request logging exists and does not log secrets or bodies.
- Tests cover new behavior.
- `go test ./...` passes.
- `go build ./cmd/scavium-faucet` passes.

## Required output

- Files read.
- Validation commands run.
- Issues found.
- Whether `step13.1.6` is required.

## Hard constraints

- Do not modify Go code.
- Do not modify docs.
- Do not read or use `cmd/scavium-faucet-v0`.

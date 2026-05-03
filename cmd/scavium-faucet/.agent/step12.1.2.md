# Step 12.1.2 — Align configuration, API and security documentation

## Recommended executor

Copilot Chat in VSCode.

## Goal

Update operational contract docs to match post-Phase 11 implementation.

## Files to consider

- `docs/scavium-faucet/configuration.md`
- `docs/scavium-faucet/api.md`
- `docs/scavium-faucet/security.md`
- Related docs under `docs/scavium-faucet/` if directly referencing config/API/security behavior

## Required corrections

Document accurately:

- `SCAVIUM_FAUCET_DATABASE_PATH`
- worker enable/poll settings
- watcher enable/poll/min-confirmation settings
- dry-run vs non-dry behavior
- admin token runtime usage
- trusted proxy behavior and real IP extraction
- claim payload support for `captcha_token` and `fingerprint`
- idempotency behavior
- rate limits by IP/address/fingerprint
- captcha behavior when disabled vs configured
- risk rejection behavior
- readiness endpoint behavior

## Instructions

- Use actual names from `config.go` and `handler.go`.
- Do not invent environment variables.
- Do not claim captcha provider production guarantees beyond current code.
- Keep wording precise: implemented, optional, configured, disabled-by-default, or planned.

## Validation

After editing, run or suggest:

```bash
grep -Rni "SCAVIUM_FAUCET_DATABASE_PATH\|captcha_token\|fingerprint\|TrustedProxy\|AdminToken" docs/scavium-faucet || true
```

## Hard constraints

- Do not modify Go code.
- Do not modify deployment scripts.
- Do not read or use `cmd/scavium-faucet-v0`.

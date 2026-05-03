# Step 12.1.1 — Align architecture and index documentation

## Recommended executor

Copilot Chat in VSCode.

## Goal

Update high-level faucet documentation to reflect the real post-Phase 11 runtime.

## Files to consider

- `docs/scavium-faucet/index.md`
- `docs/scavium-faucet/architecture.md`
- Any directly linked overview file under `docs/scavium-faucet/`

## Required corrections

If the current docs claim otherwise, correct them to say:

- Runtime uses SQLite-backed persistent service by default.
- `app.New` opens SQLite using configured database path.
- Migrations run during SQLite open.
- Public claims are persisted and enqueued.
- Worker processes queue items.
- Dry-run mode uses a dry-run sender.
- Non-dry mode wires RPC client, signer, real sender and watcher.
- Readiness checks are real and include DB/queue, plus RPC/wallet when applicable.
- Admin token is passed into HTTP/admin dependencies.
- Captcha, trusted proxy IP extraction, fingerprint, user-agent, risk evaluator and persistent rate limits are wired.
- SQLite uses WAL and busy timeout to reduce `SQLITE_BUSY`.

## Instructions

- Preserve narrative continuity.
- Do not replace the files wholesale unless they are very small.
- Remove or rewrite outdated warnings about in-memory-only runtime, stub readiness, or unwired persistence.
- Keep known limitations if still true, but make them precise.

## Validation

After editing, run or suggest:

```bash
grep -Rni "in-memory\|stub readiness\|not wired\|SQLite.*not\|admin token.*not\|captcha.*not" docs/scavium-faucet || true
```

## Hard constraints

- Do not modify Go code.
- Do not modify deployment scripts.
- Do not read or use `cmd/scavium-faucet-v0`.

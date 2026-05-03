# Copilot Chat Prompts — Phase 12 Documentation Alignment

Use these prompts in VSCode Copilot Chat, not Codex, unless a step explicitly requires Codex.

The goal is to use Copilot for cheap repository reading, documentation planning, and documentation edits.

---

## Pre-check prompt

Paste this into Copilot Chat before executing step12.1.0:

```text
Inspect the current repository only.
Focus on cmd/scavium-faucet and docs/scavium-faucet.
Do not modify files yet.

Verify the real post-Phase 11 runtime implementation:
- Does app.New use PersistentReadService instead of NewInMemoryReadService?
- Is SQLite opened from config?
- Are migrations run automatically?
- Are claims persisted and enqueued?
- Is the worker started?
- Is the watcher conditionally started?
- Are readiness checks real?
- Is AdminToken wired?
- Are captcha, IP, fingerprint, user-agent, risk, and persistent rate limits wired?
- Is SQLite WAL/busy_timeout configured?

Then list every documentation file under docs/scavium-faucet and identify statements that are now outdated.
Do not read or use cmd/scavium-faucet-v0.
Do not modify code.
Return a concise file-by-file documentation edit plan.
```

---

## Final review prompt

Paste this into Copilot Chat after all Phase 12 documentation steps:

```text
Inspect the current repository only.
Focus on docs/scavium-faucet and cmd/scavium-faucet.
Do not modify files unless you find a documentation-only inconsistency that is clearly wrong.

Verify that the faucet documentation now matches the post-Phase 11 runtime:
- SQLite-backed runtime
- migrations
- persistent claims
- persistent queue
- worker
- watcher
- readiness
- admin token
- captcha/risk/rate-limit wiring
- WAL/busy_timeout
- dry-run vs non-dry behavior

Do not modify Go code.
Do not read or use cmd/scavium-faucet-v0.
Run or suggest a documentation consistency check using grep for outdated phrases:
- in-memory
- stub readiness
- not wired
- SQLite not wired
- admin token not wired
- captcha not wired

Return a concise final verification summary.
```

# Phase 13 — Production Hardening

## Objective

Harden the already-wired SCAVIUM public faucet runtime before real VPS/nginx/systemd/certbot deployment.

This phase is code-first and tests-first. Deployment remains out of scope.

## Current baseline

Post-Phase 11 and Phase 12, the faucet runtime is expected to have:

- SQLite-backed `PersistentReadService`
- automatic migrations
- persistent claims and queue
- worker runtime
- dry-run sender and real sender
- watcher in non-dry runtime
- real readiness checks
- admin token wiring
- captcha/risk/rate-limit signal wiring
- documentation aligned with runtime

## Known gaps targeted by this phase

1. **Error code granularity**
   - Captcha failures, rate limit failures, cooldown, faucet paused/mode failures and risk rejection should not collapse into generic `500 claim_unavailable`.

2. **CORS enforcement**
   - CORS should be explicit and configurable, not implicit or absent.
   - It must not open all origins by default.

3. **Daily budget enforcement**
   - `SCAVIUM_FAUCET_DAILY_BUDGET_WEI` is loaded but not enforced.
   - Enforce it persistently against delivered/queued claim budget according to current schema capabilities or minimal migration.

4. **Observability minimum**
   - Add minimal structured HTTP logging and/or counters that do not leak secrets.
   - Keep Prometheus or heavy metrics optional unless already present.

5. **Hardening validation**
   - Add regression tests and a smoke checklist.

## Out of scope

- VPS provisioning
- nginx configuration
- certbot
- systemd production installation
- wallet UI work
- broad admin persistence redesign
- HA/distributed locks
- multi-wallet/multi-network expansion
- broad documentation rewrite

## Execution strategy

Use Copilot Chat for read-only audits and planning.

Use Codex in VSCode for implementation steps.

Preferred order:

```text
13.1.0 → Copilot Chat → hardening audit plan only
13.1.1 → Codex → precise claim error mapping
13.1.2 → Codex → configurable CORS
13.1.3 → Codex → daily budget enforcement
13.1.4 → Codex → minimal observability/logging
13.1.5 → Copilot Chat → final audit
13.1.6 → Codex only if final audit finds a small fix
```

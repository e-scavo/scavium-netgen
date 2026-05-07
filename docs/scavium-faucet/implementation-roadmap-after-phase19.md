# SCAVIUM Faucet — Implementation Roadmap After Phase 19

## Status at source ZIP

Source of truth: the project ZIP provided after Phase 19 closure.

The current production baseline is closed through Phase 24, including:

- Phase 15: captcha, durable abuse signals, progressive enforcement, retention.
- Phase 16: structured logs, request/correlation IDs, admin metrics, enriched health/readiness.
- Phase 17: token support, public token catalog, token-aware claim validation, token-aware frontend, post-audit fixes.
- Phase 18: production-safe admin control subset.
- Phase 19: production hardening plus post-audit fixes for HTTP headers, request limits, shutdown, content type validation, nginx duplicate header prevention, and test timeout guidance.
- Phase 20: SQLite-backed admin claim/queue read+control, durable admin audit history, persisted admin blocklist, and persisted claim-path blocklist enforcement.
- Phase 21: operator observability, protected Prometheus-compatible metrics, alerting guidance, and smoke checks.
- Phase 22: conservative startup RPC failover and admin-protected wallet/nonce visibility.
- Phase 23: backup/restore scripts, WAL/SHM restore handling, refill/rotation runbooks, and rollback checks.
- Phase 24: post-14 roadmap closure audit and backlog handoff.

This document exists to prevent scope drift after Phase 19. As of Phase 24, the post-Phase-14 production-baseline roadmap is complete; broader feature-list expansion now resumes from `docs/scavium_faucet_public_features.md`.

## Non-negotiable project rules

1. The ZIP/worktree is the only source of truth.
2. SCAVIUM Faucet is the only active project context.
3. The software is production software; do not introduce incomplete runtime behavior.
4. Deliveries must be partial ZIPs containing only intervened and/or newly created files.
5. Preserve public contracts unless a phase explicitly adds a backward-compatible endpoint.
6. Do not perform large refactors.
7. Do not add heavy dependencies.
8. Update project documentation incrementally with enriched narrative for every phase.
9. Keep deployment files under `docs/scavium-faucet/deployment/` maintained when touched.
10. Keep scripts under `scripts/` focused on deployment/operations and never embed secrets.
11. Every implementation phase must validate with:
    - `go test ./...`
    - `make build -B` or, at minimum, `go build ./cmd/scavium-faucet` if unrelated tools fail.
12. The previous 24-hour coding window was exhausted. From this point forward, every phase must be split into steps small enough to finish safely inside a renewed 24-hour coding budget. If a step cannot be completed safely within that window, it must be split before coding.

## Post-14 closure and backlog handoff

The post-Phase-14 roadmap no longer has open production-baseline gaps. Phases 20 through 23 closed the durable admin-state, observability, blockchain-resilience, and operations/runbook items that were still open after Phase 19.

The following topics are not closure gaps. They are broader product/backlog items that must be scheduled explicitly after Phase 24:

### Admin/control-plane maturation backlog

- Dynamic durable budget/config editing.
- CSV/export workflows.
- Allowlist/campaign management.
- Runtime token mutation and database-backed token catalogs.

### Advanced observability backlog

- External monitoring stack integration, if needed by operators.
- Slow-request analytics beyond the current structured logs and metrics.
- Log rotation and retention templates tailored to the final production host policy.

### Production/mainnet scale backlog

- Hot/cold-wallet automation and automatic treasury refill.
- More advanced stuck transaction replacement policy, if the chain and gas model support it safely.
- Multi-instance/high-availability control, distributed locks, PostgreSQL migration, and broader professional-scale architecture.

## Closed phases from the post-14 roadmap

### Phase 20 — SQLite-backed Admin State and Enforcement (Closed)

Goal achieved: the intentional Phase 18 durability limitation for queue/claim/control/audit/blocklist behavior is now closed with SQLite-backed production state.

Closure status:

- 20.1: SQLite admin read model foundation for dashboard claim counts, admin claim listing/detail, and admin queue snapshots.
- 20.2: SQLite-backed retry/cancel control for eligible persisted claims.
- 20.3: Durable admin audit log persisted and read from SQLite.
- 20.4: Persisted admin blocklist and claim-path enforcement for `ip`, `address`, and `fingerprint`.
- 20.5: closure audit and documentation alignment.

Phase 20 preserves existing public/admin HTTP contracts while moving operator state from in-memory behavior to durable SQLite-backed behavior for these surfaces.

### Phase 21 — Operator Observability and Alerting Baseline — CLOSED

Goal: give operators production feedback loops without introducing a heavy monitoring stack requirement.

Scope:

- Add or document a Prometheus-compatible protected export surface if feasible without dependency bloat.
- Expanded metrics around token buckets, worker queue outcomes, watcher blockchain/reconciliation outcomes, plus runbook guidance for abuse/blocklist investigation and RPC readiness correlation.
- Added alert threshold guidance for low balance, RPC unavailable, queue stuck, failed transaction spike, captcha spike, blocklist spike, and high rejection rate.
- Add nginx/journald log-correlation guidance using `X-Request-ID` and `X-Correlation-ID`.
- Add operational smoke-test commands.

Suggested subphases:

- 21.1: Metrics inventory and stable export plan.
- 21.2: Queue/blockchain/abuse metrics expansion.
- 21.3: Alerting runbook and smoke tests.
- 21.4: Observability closure audit.

### Phase 22 — Blockchain and Runtime Resilience — CLOSED

Goal: harden the RPC/transaction path for long-running production operation.

Closure scope:

- RPC failover is implemented as optional, primary-first, startup-only endpoint selection through `SCAVIUM_FAUCET_RPC_SECONDARY_URLS`.
- Every selected RPC endpoint must pass configured chain-ID validation before runtime use.
- `/api/v1/admin/wallet` and `/api/v1/admin/runtime.wallet` expose admin-protected signer address, native balance, pending nonce, and configured token balance status.
- Stuck transaction handling remains bounded to the existing watcher plus persisted admin retry/cancel controls; no unsafe replacement automation was introduced.
- Reorg/min-confirmation behavior remains governed by `SCAVIUM_FAUCET_MIN_CONFIRMATIONS` and documented watcher behavior.
- No multi-instance, distributed lock, load-balancing, or hot/cold-wallet automation redesign was introduced.

### Phase 23 — Operational Runbooks, Backup/Restore, and Wallet Procedures

Status: closed in Phase 23.

Goal: make operations repeatable and auditable before broader feature expansion.

Scope:

- SQLite backup and restore scripts/guides.
- Config backup checklist.
- Wallet rotation runbook.
- Manual refill runbook with hard safety checks.
- Deployment rollback verification.
- Production checklist closure.

Suggested subphases:

- 23.1: Backup/restore scripts and dry-run verification — closed.
- 23.2: Wallet refill and rotation runbooks — closed.
- 23.3: Deployment/rollback operational closure — covered by the Phase 23 runbook/deployment checklist.

### Phase 24 — Post-14 Roadmap Closure Audit — CLOSED

Goal achieved: the post-Phase-14 roadmap is closed before starting broader backlog work from the public features file.

Closure scope:

- Phases 15 through 23 audited against code, tests, docs, scripts, and deployment templates.
- Deferred post-14 items classified as either closed by Phases 20 through 23 or intentionally moved to the broader feature backlog.
- `docs/scavium_faucet_public_phase-roadmap-post14.md` updated with final closure notes.
- `docs/scavium-faucet/phase24-post14-closure-audit.md` added as the manual closure record and validation checklist.
- `cmd/scavium-faucet/.agent/phase24-closure-audit-prompt.md` added for Copilot/manual re-audit.

Next backlog entry point: Phase 25, unless the operator explicitly schedules a different item from `docs/scavium_faucet_public_features.md`.

## Broader feature phases after the post-14 roadmap is satisfied

Phase 24 is closed. Continue with the remaining features from `docs/scavium_faucet_public_features.md`. Stage 4 remains deferred temporarily and partially as already agreed.

### Phase 25 — Public API Completion and OpenAPI

- `GET /api/v1/address/{address}/history` if still absent.
- Wallet-oriented eligibility/status endpoint completion.
- OpenAPI contract document generated/maintained manually or with a lightweight script.
- Pagination conventions where needed.
- Backward-compatible only.

### Phase 26 — Public Frontend Completion

- Claim status/history UX.
- Explorer links.
- Public privacy/terms links.
- Accessibility pass.
- Mobile/responsive polish.
- Maintenance-mode UX verification.

### Phase 27 — Advanced Anti-Abuse

- Risk score expansion.
- Burst detection.
- Address clustering.
- Rotating IP heuristics.
- Optional JS/honeypot challenge.
- Manual review only after admin persistence is stable.

### Phase 28 — Config and Budget Control — CLOSED

- Durable SQLite runtime policy store for a minimal non-secret subset.
- Admin-protected policy read/update/clear API with durable audit.
- Runtime application for cooldown, rate-limit, aggregate budget, and token daily-budget enforcement.
- No campaign system, allowlist behavior, secret mutation, or token catalog mutation introduced.

### Phase 29 — Campaigns, Allowlists, and Invitation Codes

Status: implemented and closure-audited through fix 13 (full SQLite admin disable audit rollback).

- Campaign tables and budgets.
- Allowlist scopes.
- Invitation codes.
- Admin campaign create/update/list/disable controls.
- Admin CSV export if not already complete.

### Phase 30 — SCAVIUM Wallet Integration — CLOSED

Status: implemented and closure-audited through fix 4.

Closure scope:

- Wallet-specific challenge endpoint added at `/api/v1/wallet/challenge` and alias `/api/v1/faucet/wallet/challenge`.
- Short-lived persisted challenges with expiry, replay resistance, and in-memory fallback behavior.
- Optional Ethereum personal-sign verification on `POST /api/v1/claim` through `wallet_challenge_id` + `wallet_signature`; legacy clients remain compatible when both fields are omitted.
- Configurable app-origin defense-in-depth through `SCAVIUM_FAUCET_WALLET_ALLOWED_ORIGINS`; missing `Origin` remains allowed for native/CLI clients.
- Docs, OpenAPI, tests, and SQLite migration updated.

Deferred professional-scale wallet backlog remains explicit: multi-wallet UX negotiation, multi-network routing, external wallet-provider webhooks, HA/distributed challenge stores beyond SQLite, and production origin presets.

- Wallet-specific endpoints and challenge strategy.
- Optional wallet signature challenge.
- App-origin policy.
- Wallet UX contract examples.

### Deferred Stage 4 / professional-scale features

The following remain intentionally deferred until the production single-instance SQLite faucet is mature:

- Multi-network.
- Multi-wallet.
- High availability.
- Distributed locks.
- Redis/PostgreSQL advisory-lock architecture.
- Automatic treasury refill.
- Webhooks.
- Tamper-evident audit chain.
- Full reporting suite.

## Copilot execution model

Use the `.agent` files as execution control:

- `.agent/rules.md`: durable constraints and scope guardrails.
- `.agent/commands.md`: validation and Git commands.
- `stepA.B.C.md`: sequential prompts. Each step must be executed in order.

Every Copilot step must:

1. Read the ZIP/worktree files named in the step.
2. Confirm the real files read.
3. Make the smallest complete implementation.
4. Include tests when code changes.
5. Update docs incrementally.
6. Run validation.
7. Provide a partial ZIP containing only changed/created files.
8. Provide complete Git commands.
9. Stop if the step is too large for the renewed 24-hour budget and split it before implementation.


## Phase 24 closure

Phase 24 closes the post-Phase-14 roadmap. The production baseline now includes security hardening, token support, durable admin state, operator observability, conservative blockchain resilience, and operational backup/restore/refill/rotation runbooks. Future work belongs to the broader feature backlog and must continue to preserve public contracts, production safety, and partial-ZIP delivery discipline.

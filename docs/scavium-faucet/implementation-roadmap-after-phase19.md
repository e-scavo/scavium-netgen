# SCAVIUM Faucet — Implementation Roadmap After Phase 19

## Status at source ZIP

Source of truth: the project ZIP provided after Phase 19 closure.

The current production baseline is closed through Phase 20, including:

- Phase 15: captcha, durable abuse signals, progressive enforcement, retention.
- Phase 16: structured logs, request/correlation IDs, admin metrics, enriched health/readiness.
- Phase 17: token support, public token catalog, token-aware claim validation, token-aware frontend, post-audit fixes.
- Phase 18: production-safe admin control subset.
- Phase 19: production hardening plus post-audit fixes for HTTP headers, request limits, shutdown, content type validation, nginx duplicate header prevention, and test timeout guidance.
- Phase 20: SQLite-backed admin claim/queue read+control, durable admin audit history, persisted admin blocklist, and persisted claim-path blocklist enforcement.

This document exists to prevent scope drift after Phase 19. The roadmap below must be completed in order before moving into broader feature-list expansion from `docs/scavium_faucet_public_features.md`.

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

## Missing features from `docs/scavium_faucet_public_phase-roadmap-post14.md`

The post-Phase-14 roadmap closes 15 through 20 and explicitly leaves the following items open or deferred:

### Admin/control-plane maturation

Still missing or intentionally limited:

- Dynamic budget/config editing.
- CSV/export workflows.
- Allowlist/campaign management.
- Runtime token mutation and database-backed token catalogs.

### Observability and operator feedback loops

Still missing or partial:

- Prometheus-style scrape surface or equivalent export format.
- Queue/blockchain/abuse metrics beyond current process-local JSON counters.
- Alerting guidance for balance, RPC, stuck queue, abnormal drain, and abuse spikes.
- Slow request logging and operational threshold documentation.
- Nginx/journald correlation guidance with request IDs.
- Log rotation and retention operational templates.

### Production/mainnet readiness

Still missing or partial:

- RPC failover.
- Wallet balance/nonce operator visibility beyond readiness probes.
- Hot/cold-wallet refill workflow.
- Wallet rotation procedure.
- Backup/restore verification scripts and runbook closure.
- Stuck transaction recovery/operator reconciliation surfaces.
- Final production checklist after observability, blockchain resilience, and operational runbooks are complete.

## Ordered phases to finish the post-14 roadmap

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

### Phase 22 — Blockchain and Runtime Resilience

Goal: harden the RPC/transaction path for long-running production operation.

Scope:

- RPC failover with conservative configuration.
- Wallet balance and nonce operator visibility.
- Stuck transaction reconciliation/admin trigger if safe.
- Replacement transaction policy only if supported and carefully bounded.
- Reorg/min-confirmation behavior documentation.
- No multi-instance/distributed lock yet.

Suggested subphases:

- 22.1: RPC failover foundation.
- 22.2: Wallet/nonce runtime visibility.
- 22.3: Stuck transaction reconciliation controls.
- 22.4: Blockchain resilience closure audit.

### Phase 23 — Operational Runbooks, Backup/Restore, and Wallet Procedures

Goal: make operations repeatable and auditable before broader feature expansion.

Scope:

- SQLite backup and restore scripts/guides.
- Config backup checklist.
- Wallet rotation runbook.
- Manual refill runbook with hard safety checks.
- Deployment rollback verification.
- Production checklist closure.

Suggested subphases:

- 23.1: Backup/restore scripts and dry-run verification.
- 23.2: Wallet refill and rotation runbooks.
- 23.3: Deployment/rollback operational closure.

### Phase 24 — Post-14 Roadmap Closure Audit

Goal: close the post-14 roadmap before starting broader backlog from the public features file.

Scope:

- Audit Phases 15 through 23 against code, tests, docs, and deployment templates.
- Confirm all deferred post-14 items are either implemented or explicitly moved to the broader feature backlog.
- Update `docs/scavium_faucet_public_phase-roadmap-post14.md` with final closure notes.
- Produce Copilot audit prompt and manual validation checklist.

## Broader feature phases after the post-14 roadmap is satisfied

After Phase 24 closes, continue with the remaining features from `docs/scavium_faucet_public_features.md`. Stage 4 remains deferred temporarily and partially as already agreed.

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

### Phase 28 — Config and Budget Control

- Dynamic budget/config editing with durable audit.
- Safe runtime policy changes.
- Optional allowlist groundwork.
- No campaign system yet unless explicitly scheduled.

### Phase 29 — Campaigns, Allowlists, and Invitation Codes

- Campaign tables and budgets.
- Allowlist scopes.
- Invitation codes.
- Admin CSV export if not already complete.

### Phase 30 — SCAVIUM Wallet Integration

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

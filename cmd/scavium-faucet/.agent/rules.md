# SCAVIUM Faucet — Agent Rules After Phase 19

## Current context

Project: SCAVIUM Faucet inside `scavium-netgen`.

This is the only active project context. Do not mix assumptions, plans, code, or documentation from any other project.

The software is in production. Treat every change as production-impacting.

The current source ZIP/worktree is the only source of truth. Do not assume structure, behavior, or status from memory. Read the real files before changing anything.

## Canonical planning documents

Read these before every new phase:

- `docs/scavium_faucet_public_features.md`
- `docs/scavium_faucet_public_phase-roadmap-post14.md`
- `docs/scavium-faucet/implementation-roadmap-after-phase19.md`
- relevant files under `docs/scavium-faucet/*.md`
- relevant deployment files under `docs/scavium-faucet/deployment/*`
- relevant scripts under `scripts/*`

## Phase ordering

The post-Phase-14 roadmap must be completed before broader feature expansion.

Closed baseline:

- Phase 15: closed.
- Phase 16: closed.
- Phase 17: closed.
- Phase 18: closed with documented in-memory admin limitations.
- Phase 19: closed with post-audit hardening fixes.

Next required phases:

1. Phase 20 — SQLite-backed Admin State and Enforcement.
2. Phase 21 — Operator Observability and Alerting Baseline.
3. Phase 22 — Blockchain and Runtime Resilience.
4. Phase 23 — Operational Runbooks, Backup/Restore, and Wallet Procedures.
5. Phase 24 — Post-14 Roadmap Closure Audit.

Only after Phase 24 closure may work proceed to the broader feature backlog in `docs/scavium_faucet_public_features.md`.

Stage/Phase 4 professional-scale features are deferred temporarily and partially unless explicitly reactivated.

## Hard constraints

- Preserve public API contracts.
- Preserve the main claim endpoint: `POST /api/v1/claim`.
- Preserve normalized error envelopes: `code`, `message`, `details`, `request_id` where applicable.
- Preserve known error codes such as `captcha_failed`, `claim_rejected`, `rate_limited`, and `daily_budget_exceeded`.
- Preserve request headers: `Idempotency-Key`, `X-Request-ID`, `X-Correlation-ID`.
- Do not introduce large refactors.
- Do not introduce heavy dependencies.
- Prefer standard library and current internal packages.
- Do not add secrets, private keys, production RPC credentials, or real domains.
- Do not log secrets, bearer tokens, captcha tokens, raw request bodies, idempotency keys, private keys, or raw sensitive values.
- Maintain backward compatibility for clients that omit optional fields.
- Keep admin endpoints protected.
- Keep deployment topology: nginx terminates TLS and proxies to localhost Go backend.

## Delivery contract

Every delivery must be a partial ZIP containing only files that were created or modified for that phase/step. Never include the full project ZIP.

Every response to a completed step must include:

1. Files read.
2. Short analysis.
3. ZIP partial link.
4. Validation commands and results.
5. Complete Git commands.

## Documentation contract

Project docs live under `docs/scavium-faucet/*.md` and must be maintained incrementally.

Roadmap/source docs:

- `docs/scavium_faucet_public_features.md` is the feature backlog.
- `docs/scavium_faucet_public_phase-roadmap-post14.md` is the post-14 roadmap and must be completed before broader backlog work.
- `docs/scavium-faucet/implementation-roadmap-after-phase19.md` is the execution guardrail after Phase 19.

When a phase changes behavior, update the relevant docs in the same phase. Do not leave code and docs misaligned.

## Deployment/scripts contract

- `docs/scavium-faucet/deployment/*` contains maintained templates/configuration for the software and third-party services.
- `scripts/*` contains deployment and operations scripts.
- Any deployment/script change must be safe, idempotent where reasonable, and documented.

## Renewed 24-hour coding budget

The previous 24-hour coding window was consumed. Every new phase must be split into small steps that fit safely into a renewed 24-hour coding budget.

If a step appears too large, stop before coding and split it into smaller sequential steps. Do not push through a risky oversized change.

## Required execution pattern per step

1. Read the real files from the ZIP/worktree.
2. Confirm files read.
3. Implement the smallest complete change.
4. Add or update tests for code changes.
5. Update docs incrementally.
6. Run `gofmt` for Go files.
7. Run validation.
8. Package only changed/created files into a partial ZIP.
9. Provide complete Git commands.

## Architecture guardrails

Main code stays under:

```text
cmd/scavium-faucet/
├─ main.go
├─ internal/
│  ├─ app/
│  ├─ config/
│  ├─ domain/
│  ├─ httpapi/
│  ├─ store/sqlite/
│  ├─ faucet/
│  ├─ chain/
│  ├─ abuse/
│  ├─ admin/
│  ├─ frontend/
│  ├─ observability/
│  └─ ready/
└─ migrations/
```

Avoid moving packages unless a step explicitly requires it and the risk is justified.

## Git policy

Every step must use a branch.

Start:

```bash
git checkout main
git pull --ff-only
git checkout -b phase-XX.Y-short-name
```

Close:

```bash
git status --short
git add <changed-files>
git commit -m "phase XX.Y short description"
git checkout main
git merge phase-XX.Y-short-name
git branch -d phase-XX.Y-short-name
```

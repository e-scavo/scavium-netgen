# Phase 25 — Public API Completion Closure Audit

## Scope closed

Phase 25 completes the scheduled public API work from `implementation-roadmap-after-phase19.md` without leaving the roadmap or starting frontend Phase 26.

Closed items:

- `GET /api/v1/address/{address}/history` and alias `GET /api/v1/faucet/address/{address}/history` return bounded, deterministic address claim history.
- Access logs redact both canonical and faucet-prefixed address status/history paths so public wallet addresses are not emitted as high-cardinality/raw path values.
- Address eligibility/status remains backward-compatible and now includes optional token-aware and daily-budget fields when durable runtime state can support them; exhausted configured daily budgets are reflected as public-safe `daily_budget_exceeded` ineligibility; the in-memory fallback reports the same optional shape for contract consistency.
- Pagination conventions are documented as bounded offset pagination with `limit`, `offset`, `count`, and `has_more`.
- `docs/scavium-faucet/openapi.yaml` records the lightweight manually maintained OpenAPI baseline for implemented public and admin-safe surfaces.

## Contract safety

The existing `POST /api/v1/claim` contract, normalized error envelope, `Idempotency-Key`, `X-Request-ID`, and `X-Correlation-ID` behavior are preserved. New history/status data is additive only. Public responses still exclude secrets, captcha tokens, fingerprints, raw abuse signals, idempotency keys in history, admin audit data, internal queue-control details, and private blocklist notes. Blocked addresses are reflected only through public-safe `eligible: false` and `reason: "blocked"`.

## Runtime validation notes

The implementation validates address input below route dispatch through the same domain address validation used by claim/status endpoints. History pagination is bounded in the service layer and rejected early for invalid query values. Address status also checks persisted address blocklist state below the HTTP handler instead of relying on route-layer assumptions. SQLite history ordering is deterministic by `created_at DESC, id DESC`, and the in-memory service applies the same public ordering model for tests and fallback usage.

## Deferred work

Frontend history/status UX remains Phase 26. Advanced anti-abuse, durable dynamic config, campaigns/allowlists, and wallet-integration challenge strategies remain later scheduled phases.

# Phase 28 — Config and Budget Control

Phase 28 adds a conservative runtime policy layer for operator-controlled budget and throttling edits. The implementation is deliberately limited to non-secret values that are already part of faucet enforcement: cooldown seconds, source-IP hourly limit, address daily limit, the aggregate UTC daily budget, and token-specific daily budgets.

## Runtime-editable subset

Editable at runtime through the admin API:

- `cooldown_seconds`
- `rate_limit_ip_per_hour`
- `rate_limit_addr_per_day`
- `daily_budget_wei`
- `token_daily_budget_wei` by configured token id

Not editable at runtime in this phase:

- private keys
- admin token
- RPC URLs
- captcha secret or verify URL
- token catalog metadata, token contract addresses, decimals, or claim amounts
- CORS origins, trusted proxy, worker/watcher topology, and chain id

Those values still require environment changes and restart because they affect startup validation, process security, or transfer semantics.

## Persistence and precedence

Runtime overrides are stored in SQLite table `runtime_policy`, created by migration `007_runtime_policy.sql`. Startup environment configuration remains the default source. When a persisted runtime override is present and valid, claim-time and public config reads use the persisted value. When an override is absent, cleared, unknown, or malformed, the service falls back to environment/default configuration instead of failing closed. Mutation paths validate non-negative values in the admin request/service layer and again at the SQLite persistence boundary, so direct store callers cannot persist negative or nameless token-budget overrides.

This precedence keeps rollback simple: `DELETE /api/v1/admin/policy` clears overrides and restores environment-backed behavior immediately.

## Admin API

`GET /api/v1/admin/policy` returns the current persisted runtime policy view.

`PUT /api/v1/admin/policy` replaces the full runtime policy override set. Values must be non-negative decimal integers. Omitted or zero throttling fields mean no persisted override for that field, so environment/default configuration applies.

`DELETE /api/v1/admin/policy` clears all runtime overrides.

All routes are protected by the existing bearer-token admin middleware. Every mutation writes a durable admin audit entry with a bounded before/after summary and no secrets.

## Runtime application

`PersistentReadService` reads the runtime policy from the same durable store used for claims. The policy is applied to:

- public `/api/v1/config` cooldown and rate-limit fields
- address eligibility cooldown and rate-limit metadata
- claim-time cooldown enforcement
- claim-time rate-limit enforcement
- aggregate and token-scoped daily budget checks

The public claim contract and normalized error envelopes are unchanged.

## Closure audit

Phase 28 intentionally does not add campaigns, invitation codes, allowlist behavior, runtime token registration, role-based admin accounts, external config services, or secret mutation. Phase 29 can build campaign and allowlist behavior on top of this durable policy/audit foundation.

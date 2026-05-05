# API reference

Base path: `/api/v1`

All API responses are JSON. Every request gets an `X-Request-ID` response header; if the client does not send one, the server generates one. The server also echoes `X-Correlation-ID`; when the client does not send one, the correlation ID falls back to the request ID.

Normalized error responses use this shape:

```json
{
  "code": "invalid_address",
  "message": "invalid address",
  "details": {
    "reason": "address must be a valid hex Ethereum address"
  },
  "request_id": "abc123"
}
```

## Public endpoints

### `GET /health`

Liveness endpoint.

```json
{
  "status": "ok",
  "time": "2026-05-03T12:00:00Z",
  "uptime_seconds": 3600,
  "build": {
    "version": "dev",
    "commit": "unknown",
    "build_date": "unknown"
  }
}
```

### `GET /ready`

Readiness endpoint. Probes are real infrastructure checks wired by `runtimeChecks()` in `app.New`:

- `db` — SQLite ping via `ready.DBCheck`
- `queue` — SQLite queue probe via `ready.QueueCheck`
- `rpc` — Ethereum JSON-RPC chain-ID check via `ready.RPCCheck` (non-dry-run only)
- `wallet` — on-chain balance check for the signer address via `ready.WalletCheck` (non-dry-run only)

In dry-run mode only `db` and `queue` checks are active. A degraded result means at least one probe failed.

Healthy example:

```json
{
  "status": "ok",
  "checks": [
    { "name": "db", "status": "ok", "duration_ms": 1 },
    { "name": "queue", "status": "ok", "duration_ms": 1 },
    { "name": "rpc", "status": "ok", "duration_ms": 4 },
    { "name": "wallet", "status": "ok", "duration_ms": 8 }
  ],
  "summary": {
    "total": 4,
    "ok": 4,
    "degraded": 0
  },
  "time": "2026-05-03T12:00:00Z"
}
```

### `GET /api/v1/status`
### `GET /api/v1/faucet/status`

Alias routes for the public faucet status payload.

```json
{
  "status": "active",
  "network_name": "scavium-test",
  "symbol": "tSCAV",
  "dry_run": false,
  "updated_at": "2026-05-03T12:00:00Z"
}
```

The value reported by `status` reflects the configured `SCAVIUM_FAUCET_MODE` value (`active`, `paused`, or `maintenance`).

### `GET /api/v1/config`
### `GET /api/v1/faucet/config`

Alias routes for public faucet configuration.

```json
{
  "network_name": "scavium-test",
  "chain_id": 123,
  "symbol": "tSCAV",
  "amount_wei": "42",
  "tokens": [
    {
      "id": "native",
      "symbol": "tSCAV",
      "type": "native",
      "decimals": 18,
      "amount_wei": "42",
      "daily_budget_wei": "4200"
    }
  ],
  "cooldown_seconds": 60,
  "explorer_tx_url": "https://explorer.example.test/tx/{txHash}",
  "dry_run": false,
  "rate_limit_ip_per_hour": 10,
  "rate_limit_addr_per_day": 3
}
```

### `GET /api/v1/tokens`
### `GET /api/v1/faucet/tokens`

Alias routes for the public faucet token catalog. These endpoints expose only claim-safe token metadata derived from runtime configuration. They do not expose private keys, admin tokens, RPC credentials, or any operational secret. They are the canonical post-restart validation point after registering testnet tokens through `SCAVIUM_FAUCET_TOKENS_JSON`.

```json
{
  "tokens": [
    {
      "id": "native",
      "symbol": "tSCAV",
      "type": "native",
      "decimals": 18,
      "amount_wei": "42",
      "daily_budget_wei": "4200"
    },
    {
      "id": "scav",
      "symbol": "SCAV",
      "type": "erc20",
      "address": "0x1111111111111111111111111111111111111111",
      "decimals": 18,
      "amount_wei": "100",
      "daily_budget_wei": "10000"
    }
  ]
}
```



Operational registration note:

- Token ids are configured server-side and discovered through this endpoint.
- Existing clients can continue omitting `token_id`; the configured default token is used.

Phase 17.2 closure note:

- These catalog endpoints are the stable discovery and validation surface for configured faucet tokens.
- They are read-only and do not mutate token configuration.
- `POST /api/v1/claim` remains backward-compatible: clients that omit `token_id` receive the configured default token.
- Token administration, frontend token selection, and database-backed token catalogs are not part of the Phase 17.2 API surface.

- ERC20 clients should first read this catalog and then submit the selected `id` as `token_id` to `POST /api/v1/claim`.
- See [token-registration.md](token-registration.md) for the testnet registration checklist.

Phase 17.3.1 validation note:

- Claim-time token validation is strict. Unknown or non-executable `token_id` values are rejected before the claim enters the durable queue.
- The public error code remains `claim_rejected`; the details reason is `invalid_token`.
- Omitted `token_id` remains backward-compatible and resolves to the configured default token.

Only `GET` is allowed. Unsupported methods return `405` with the existing `method_not_allowed` envelope.

### `GET /api/v1/address/{address}/status`
### `GET /api/v1/faucet/address/{address}/eligibility`

Alias routes for address eligibility.

```json
{
  "address": "0x52908400098527886E0F7030069857D2E4169EE7",
  "eligible": true,
  "reason": "eligible",
  "cooldown_seconds": 60,
  "cooldown_remaining_seconds": 0,
  "rate_limit_ip_per_hour": 10,
  "rate_limit_addr_per_day": 3
}
```

An invalid address returns `400` with `code: "invalid_address"`.

### `POST /api/v1/claim`
### `POST /api/v1/faucet/claim`

Alias routes for claim creation.

Request body:

```json
{
  "address": "0x52908400098527886E0F7030069857D2E4169EE7",
  "token_id": "native",
  "captcha_token": "<provider-token>",
  "fingerprint": "<client-fingerprint>"
}
```

`token_id` is optional; when omitted, the configured default token is used. All fields except `address` remain optional at the wire-contract level. `captcha_token` is required by policy when `SCAVIUM_FAUCET_CAPTCHA_PROVIDER` is not `disabled`; the public frontend obtains it from the configured provider widget using the public `captcha_site_key` exposed by `/api/v1/config`. `fingerprint` is used for fingerprint-scoped rate limiting when provided.

Optional request header:

```text
Idempotency-Key: same-key-for-retries
```

Accepted response:

```json
{
  "id": "claim_test",
  "address": "0x52908400098527886E0F7030069857D2E4169EE7",
  "amount_wei": "42",
  "token_id": "native",
  "token_symbol": "SCAV",
  "token_type": "native",
  "token_decimals": 18,
  "status": "queued",
  "idempotency_key": "same-key-for-retries",
  "created_at": "2026-05-03T12:00:00Z",
  "updated_at": "2026-05-03T12:00:00Z"
}
```

Current behavior:

- `address`, optional `token_id`, `captcha_token`, and `fingerprint` are decoded from the body
- the body is capped at `1 MiB`
- `RemoteIP` is extracted from the request (trusting `X-Forwarded-For` / `X-Real-IP` when `SCAVIUM_FAUCET_TRUSTED_PROXY` is set)
- `UserAgent` is forwarded from the request header
- the address cooldown is checked against the SQLite store
- persistent rate limits are enforced per IP (hourly), per address (daily), and per fingerprint (hourly when provided)
- the daily budget is enforced for the selected token when token-scoped configuration is available; legacy deployments keep the existing faucet-wide budget behavior
- captcha is verified when `SCAVIUM_FAUCET_CAPTCHA_PROVIDER` is not `disabled`; missing or failed verification returns `422 captcha_failed`
- risk evaluation runs when a risk engine is configured
- the accepted claim is persisted to SQLite with initial status `received`, then enqueued as `queued`
- repeated requests with the same `Idempotency-Key` return the same persisted claim without creating a duplicate

Common errors:

| HTTP | Code | Meaning |
|---|---|---|
| 400 | `invalid_json` | Malformed JSON body |
| 400 | `invalid_address` | Address failed validation |
| 422 | `captcha_failed` | Captcha verification failed |
| 403 | `claim_rejected` | Claim rejected by abuse or risk policy |
| 429 | `rate_limited` | IP, address, fingerprint, or cooldown limit exceeded |
| 429 | `daily_budget_exceeded` | Global daily faucet budget exceeded |
| 503 | `faucet_unavailable` | Faucet is paused, in maintenance, or unavailable |
| 500 | `claim_unavailable` | Unexpected claim service failure |

For `rate_limited`, `daily_budget_exceeded`, and other mapped claim errors, the `details` object may include:

```json
{
  "reason": "retry after 30 seconds",
  "retry_after_seconds": 30
}
```

`retry_after_seconds` is advisory and appears when the server can calculate a retry delay. A `Retry-After` header may be added in future versions.

### `GET /api/v1/claim/{id}`
### `GET /api/v1/faucet/claim/{id}`

Alias routes for claim lookup.

```json
{
  "id": "claim_test",
  "address": "0x52908400098527886E0F7030069857D2E4169EE7",
  "amount_wei": "42",
  "token_id": "native",
  "token_symbol": "SCAV",
  "token_type": "native",
  "token_decimals": 18,
  "status": "queued",
  "created_at": "2026-05-03T12:00:00Z",
  "updated_at": "2026-05-03T12:00:00Z"
}
```

If the claim is missing, the server returns `404` with `code: "claim_not_found"`.

### `GET /api/v1/version`

Returns build metadata from `internal/version`.

```json
{
  "version": "dev",
  "commit": "unknown",
  "build_date": "unknown"
}
```

## Admin endpoints

Admin routes are active when `SCAVIUM_FAUCET_ADMIN_TOKEN` is set. `app.New` passes the token into `httpapi.Dependencies.AdminToken`; when the token is empty the routes return `503`. Admin routes are excluded from CORS even when public API CORS is configured.

Every admin request must include:

```text
Authorization: Bearer <SCAVIUM_FAUCET_ADMIN_TOKEN>
```

Wrong or missing token returns `401`. Empty configured token returns `503`. The admin token is compared with constant-time comparison and is never written to structured logs.

### `GET /api/v1/admin/metrics`

Returns process-local runtime counters, build/runtime metadata, token-scoped claim counters, and Phase 18.1 process metrics. Counters reset when the faucet process restarts and are intended for immediate operational diagnostics, not durable accounting. The special `default` token id represents requests that omitted `token_id` at the HTTP boundary and therefore used the configured default-token path.

Representative shape:

```json
{
  "started_at": "2026-05-03T12:00:00Z",
  "uptime_seconds": 3600,
  "build": {
    "version": "dev",
    "commit": "unknown",
    "build_date": "unknown"
  },
  "claims": {
    "accepted": 12,
    "rejected": 3,
    "rejected_by_risk": 1,
    "faucet_unavailable": 0,
    "claim_unavailable": 0,
    "invalid_token": 1
  },
  "captcha": { "failed": 1 },
  "rate_limits": { "limited": 1 },
  "budgets": { "daily_exceeded": 1 },
  "tokens": [
    {
      "token_id": "default",
      "accepted": 8,
      "rejected": 1,
      "rate_limited": 0,
      "daily_exceeded": 0,
      "invalid_token": 0
    }
  ],
  "process": {
    "goroutines": 12,
    "cpu_count": 2,
    "memory_alloc_bytes": 123456,
    "memory_sys_bytes": 789012,
    "heap_alloc_bytes": 123456,
    "heap_objects": 100,
    "mallocs": 200,
    "frees": 100,
    "gc_cycles": 1
  }
}
```

Operational notes:

- `claims.*`, `captcha.*`, `rate_limits.*`, `budgets.*`, and `process.*` are process-local diagnostics.
- `tokens[*]` is diagnostic only and is not a durable accounting ledger.
- Token ids are configuration/public-catalog identifiers only; wallet addresses, raw fingerprints, request bodies, captcha tokens, admin tokens, and idempotency keys are not exposed.

### `GET /api/v1/admin/runtime`

Returns a composite operator view containing dashboard, readiness, metrics, and a server timestamp. This endpoint is a convenience aggregation over already-protected admin/runtime data; it does not mutate state.

Representative shape:

```json
{
  "dashboard": {
    "mode": "active",
    "claim_counts": {},
    "blocklist_size": 0,
    "updated_at": "2026-05-03T12:00:00Z"
  },
  "readiness": {
    "status": "ready",
    "checks": []
  },
  "metrics": {},
  "time": "2026-05-03T12:00:00Z"
}
```

### `GET /api/v1/admin/dashboard`

Returns the admin summary used by the runtime endpoint:

```json
{
  "mode": "active",
  "claim_counts": {},
  "blocklist_size": 0,
  "updated_at": "2026-05-03T12:00:00Z"
}
```

### `GET /api/v1/admin/queue?limit=50`

Returns bounded queue visibility for operators. The `limit` parameter defaults to the handler default and is capped at `500` to avoid accidental large admin responses.

Representative shape:

```json
{
  "summary": {
    "counts": {
      "queued": 3,
      "sending": 1,
      "failed": 2
    },
    "ready": 3,
    "delayed": 0,
    "in_flight": 1,
    "pending_tx": 0,
    "terminal": 2,
    "total": 6,
    "updated_at": "2026-05-03T12:00:00Z"
  },
  "items": [
    {
      "id": "claim-id",
      "status": "queued",
      "token_id": "default",
      "tx_hash": "",
      "attempts": 0,
      "next_attempt_at": "2026-05-03T12:00:00Z",
      "updated_at": "2026-05-03T12:00:00Z"
    }
  ]
}
```

Wallet addresses are intentionally omitted from queue items.

### `POST /api/v1/admin/queue/retry`

Request body:

```json
{ "id": "claim-id" }
```

Retries an eligible queued/failed claim through the admin service.

Success response:

```json
{ "status": "retried" }
```

### `POST /api/v1/admin/queue/cancel`

Request body:

```json
{ "id": "claim-id" }
```

Cancels an eligible queued claim through the admin service.

Success response:

```json
{ "status": "cancelled" }
```

### `GET /api/v1/admin/claims?limit=50&offset=0`

Lists admin-visible claims for operator review.

```json
{
  "claims": []
}
```

### `GET /api/v1/admin/claim/{id}`

Returns one claim or `404`.

### `POST /api/v1/admin/claim/{id}/retry`

Moves a `failed` or `rejected` claim back to `queued` when the claim is eligible.

Success response:

```json
{ "status": "retried" }
```

Conflict response:

- `409` / `not_retryable`

### `POST /api/v1/admin/claim/{id}/cancel`

Rejects a claim that has not been sent yet.

Success response:

```json
{ "status": "cancelled" }
```

Conflict response:

- `409` / `not_cancellable`

### `POST /api/v1/admin/faucet/mode`

Request body:

```json
{ "mode": "paused" }
```

Accepted values are:

- `active`
- `paused`
- `maintenance`

Success response:

```json
{ "mode": "paused" }
```

Invalid mode response:

- `400` / `invalid_mode`

As of Phase 18.7, accepted mode changes are propagated into the live faucet runtime, so public claim handling observes the selected mode. The action is also captured in the in-process audit log and in structured admin-action logs.

### `GET /api/v1/admin/blocklist`

```json
{
  "entries": []
}
```

### `POST /api/v1/admin/blocklist`

Request body:

```json
{
  "key_type": "ip",
  "value": "1.2.3.4",
  "reason": "test"
}
```

Success response:

```json
{ "status": "blocked" }
```

Structured logs for this action do not include the raw blocklist value or reason text.

### `DELETE /api/v1/admin/blocklist?key_type=ip&value=1.2.3.4`

Success response:

```json
{ "status": "unblocked" }
```

### `GET /api/v1/admin/audit?limit=100`

Returns recent in-process audit entries. The audit trail is useful for immediate operator review and is not a durable compliance ledger. Structured admin-action logs are also emitted for sensitive admin operations.

```json
{
  "entries": [
    {
      "action": "set_mode",
      "actor": "203.0.113.10",
      "target": "faucet",
      "detail": "paused",
      "created_at": "2026-05-03T12:00:00Z"
    }
  ]
}
```

## Token-aware claim enforcement

`POST /api/v1/claim` continues to accept an optional `token_id`. After token validation, cooldown and rate-limit enforcement are applied to the resolved token scope. The public response and error contracts do not change: invalid tokens still use `claim_rejected` with reason `invalid_token`, cooldown uses `cooldown_active`, rate limiting uses `rate_limited`, and daily budget exhaustion uses `daily_budget_exceeded`.


Phase 17.3 closure note:

- Token validation, token-scoped enforcement, and token-scoped metrics are now part of the stable claim-intake baseline.
- `token_id` remains optional; omitted values continue to use the configured default token.
- Invalid token selections continue to use the existing `claim_rejected` public contract with `invalid_token` as the details reason.
- No response body shape, public endpoint path, admin authentication model, or claim error envelope is changed by the closure.

Phase 17.4.1 frontend consumption note:

- The embedded public faucet UI consumes `GET /api/v1/tokens` as its token-selection source.
- Browser clients submit the selected catalog `id` as optional `token_id` in `POST /api/v1/claim`.
- If catalog discovery fails, the frontend omits `token_id` and preserves the configured default-token path.
- The catalog remains read-only and claim-safe; it does not expose private keys, admin tokens, RPC credentials, request bodies, fingerprints, or idempotency keys.

Phase 17.4.2 frontend UX note:

- The selector now presents explicit loading and fallback states for catalog discovery.
- Selected-token details are rendered from claim-safe catalog metadata only.
- Fallback UX remains contract-preserving: when no catalog token is selected, the browser omits `token_id` and the backend resolves the configured default token.

Phase 17.4.3 claim-result UX note:

- Claim creation and claim polling responses keep the same response body shape.
- The frontend now formats returned `amount_wei` with token decimals when token metadata is available, while preserving the raw base-unit value in the result details.
- Explorer links continue to use the configured explorer URL and `tx_hash`; only the user-facing copy is clarified as a transaction action.
- Native/default claims remain supported even when `token_id`, `token_symbol`, or `token_type` are absent from older/default responses.

Phase 17.4 closure note:

- The embedded frontend now treats `GET /api/v1/tokens` as the stable browser-side catalog source.
- `token_id` remains optional in `POST /api/v1/claim`; omitted values continue to use the configured default token.
- Catalog discovery failures are handled on the client by omitting `token_id`, not by changing the claim API contract.
- Claim-result rendering may display token-aware summaries, but the API response shape and existing error envelope remain unchanged.


Phase 17 closure note:

- Multi-token support is now part of the stable faucet API baseline for the current public testnet scope.
- `GET /api/v1/tokens` and `GET /api/v1/faucet/tokens` are the public, claim-safe discovery endpoints for configured assets.
- `POST /api/v1/claim` remains backward-compatible: `token_id` is optional and omitted values continue to use the configured default token.
- Token validation, token-scoped enforcement, token-aware metrics, and frontend token selection do not change the public error envelope or response body contract.
- Runtime token administration, database-backed token catalogs, and admin mutation endpoints are not part of Phase 17.

Phase 17.5 post-audit closure note:

- The frontend status banner consumes the `status` field returned by `/api/v1/status`; no API response shape change is introduced.
- Cooldown presentation uses `retry_after_seconds` from the existing error `details` object when available.
- Runtime metrics now keep accepted and rejected requests that omit `token_id` in the same `default` token bucket, preserving legacy-client observability consistency.
- Rejection logging and token-scoped metrics defensively sanitize user-supplied token ids before recording them.
- Public claim, token catalog, status, and metrics endpoint contracts remain unchanged.

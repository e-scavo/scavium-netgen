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
  "rate_limit_addr_per_day": 3,
  "default_token_id": "native",
  "daily_budget": {
    "budget_wei": "100000000000000000000",
    "used_wei": "0",
    "remaining_wei": "100000000000000000000"
  },
  "tokens": [
    {
      "token_id": "native",
      "symbol": "SCAV",
      "type": "native",
      "eligible": true,
      "reason": "eligible",
      "amount_wei": "42000000000000000000",
      "cooldown_remaining_seconds": 0,
      "daily_budget": {
        "budget_wei": "100000000000000000000",
        "used_wei": "0",
        "remaining_wei": "100000000000000000000"
      }
    }
  ]
}
```

Phase 25 extends this response only with backward-compatible optional fields. Older clients can ignore `default_token_id`, `daily_budget`, and `tokens`. The endpoint remains public and intentionally excludes raw abuse signals, fingerprints, captcha material, internal risk scores, blocklist reasons, and admin-only decisions. Per-token eligibility is derived from the configured token catalog and token-scoped cooldown/budget state when durable storage supports it; the in-memory fallback also emits the optional token/default-token fields for contract consistency. Persisted address blocklist state is reflected as `eligible: false` with public reason `blocked` only; private operator blocklist notes are never exposed. When configured daily budget visibility is available and the remaining public budget is exhausted, address and token status report `eligible: false` with public reason `daily_budget_exceeded`, matching claim-time enforcement without exposing internal accounting beyond the documented budget totals.

An invalid address returns `400` with `code: "invalid_address"`.

### `GET /api/v1/address/{address}/history`
### `GET /api/v1/faucet/address/{address}/history`

Returns a public, address-scoped claim history with deterministic reverse-creation ordering. The endpoint is intentionally bounded and exposes only the same public-safe claim fields used by claim creation and lookup. It never returns idempotency keys, captcha tokens, fingerprints, raw abuse signals, admin audit data, or internal queue control details.

Query parameters:

| Parameter | Default | Maximum | Description |
|---|---:|---:|---|
| `limit` | `25` | `100` | Number of claims to return. Values above the maximum are capped. |
| `offset` | `0` | n/a | Zero-based offset for pagination. Negative or non-integer values are rejected. |

Example response:

```json
{
  "address": "0x52908400098527886E0F7030069857D2E4169EE7",
  "claims": [
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
  ],
  "pagination": {
    "limit": 25,
    "offset": 0,
    "count": 1,
    "has_more": false
  }
}
```

Invalid addresses return `400 invalid_address`. Invalid pagination values return `400 invalid_pagination`. Empty history is a successful `200` with an empty `claims` array and `count: 0`.

### `POST /api/v1/claim`
### `POST /api/v1/faucet/claim`

Alias routes for claim creation.

Request body:

```json
{
  "address": "0x52908400098527886E0F7030069857D2E4169EE7",
  "token_id": "native",
  "campaign_id": "launch-campaign",
  "invitation_code": "INVITE-001",
  "captcha_token": "<provider-token>",
  "fingerprint": "<client-fingerprint>",
  "website": ""
}
```

`token_id` is optional; when omitted, the configured default token is used. `campaign_id` and `invitation_code` are optional Phase 29 campaign controls; legacy clients can omit both and continue through the normal faucet policy path. Invite-scoped campaigns require a valid invitation code, allowlist-scoped campaigns require the claimant address to be present in the durable allowlist, and private campaign failures return the generic `invalid_campaign` rejection reason. All fields except `address` remain optional at the wire-contract level. `captcha_token` is required by policy when `SCAVIUM_FAUCET_CAPTCHA_PROVIDER` is not `disabled`; the public frontend obtains it from the configured provider widget using the public `captcha_site_key` exposed by `/api/v1/config`. `fingerprint` is used for fingerprint-scoped rate limiting and Phase 27 clustering when provided. `website` is an optional compatibility-safe honeypot field: legitimate clients should omit it or send an empty string, and it is evaluated only when `SCAVIUM_FAUCET_ABUSE_HONEYPOT_ENABLED=true`.

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
  "campaign_id": "launch-campaign",
  "created_at": "2026-05-03T12:00:00Z",
  "updated_at": "2026-05-03T12:00:00Z"
}
```

Current behavior:

- `address`, optional `token_id`, optional `campaign_id`, optional `invitation_code`, `captcha_token`, `fingerprint`, and disabled-by-default honeypot field `website` are decoded from the body
- the body is capped at `1 MiB`; requests declaring a larger `Content-Length` are rejected before claim handling
- `RemoteIP` is extracted from the request (trusting `X-Forwarded-For` / `X-Real-IP` when `SCAVIUM_FAUCET_TRUSTED_PROXY` is set)
- `UserAgent` is forwarded from the request header
- the address cooldown is checked against the SQLite store
- persistent rate limits are enforced per IP (hourly), per address (daily), and per fingerprint (hourly when provided); empty scope values are skipped, fingerprint values are trimmed/lowercased for keying, and denial reasons remain generic rather than exposing raw limiter keys
- campaign validation runs after token/blocklist validation and before captcha/risk/cooldown/rate-limit work; campaign budgets are enforced against durable claim usage and invite codes are consumed only after successful durable claim creation
- the daily budget is enforced for the selected token when token-scoped configuration is available; legacy deployments keep the existing faucet-wide budget behavior
- captcha is verified when `SCAVIUM_FAUCET_CAPTCHA_PROVIDER` is not `disabled`; missing or failed verification returns `422 captcha_failed`
- risk evaluation runs when a risk engine is configured; Phase 27 composes persisted negative-signal counts, same-IP bursts across successful and failed intake records, rotating-IP fingerprint behavior, address clustering, optional honeypot state, and bounded manual-review hints into deterministic operator diagnostics
- the accepted claim is persisted to SQLite with initial status `received`, then enqueued as `queued`
- repeated requests with the same `Idempotency-Key` return the same persisted claim without creating a duplicate

Common errors:

| HTTP | Code | Meaning |
|---|---|---|
| 400 | `invalid_json` | Malformed JSON body |
| 400 | `invalid_address` | Address failed validation |
| 413 | `request_body_too_large` | Declared JSON request body exceeds `1 MiB` |
| 422 | `captcha_failed` | Captcha verification failed |
| 403 | `claim_rejected` | Claim rejected by abuse, risk, or invalid campaign policy |
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

## Pagination conventions

Phase 25 standardizes bounded offset pagination for public list-style endpoints. Public pagination uses `limit`, `offset`, `count`, and `has_more`; returned item order is deterministic and documented per endpoint. Current public usage is `GET /api/v1/address/{address}/history`. Admin list endpoints keep their existing bounded `limit`/`offset` behavior.

## Admin endpoints

Admin routes are active when `SCAVIUM_FAUCET_ADMIN_TOKEN` is set. `app.New` passes the token into `httpapi.Dependencies.AdminToken`; when the token is empty the routes return `503`.

When the handler is constructed with an admin token, every admin request must include:

```text
Authorization: Bearer <SCAVIUM_FAUCET_ADMIN_TOKEN>
```

Wrong or missing token returns `401`. Empty configured token returns `503`.

### Phase 29 campaign administration endpoints

The following endpoints are protected by the existing admin bearer-token middleware and use the same JSON error envelope as the rest of the admin plane.

- `GET /api/v1/admin/campaigns` lists durable campaigns. Query parameters: `limit` (bounded to 500) and `offset`.
- `POST /api/v1/admin/campaigns` creates a campaign with `id`, `name`, `scope` (`public`, `invite`, or `allowlist`), optional `token_id`, optional `budget_wei`, optional RFC3339 `starts_at` / `ends_at`, and `enabled`.
- `PUT /api/v1/admin/campaigns/{id}` updates the same mutable campaign fields and preserves historical claim attribution. The path id must match an optional body `id` when one is supplied.
- `POST /api/v1/admin/campaigns/{id}/disable` disables a campaign without deleting historical attribution.
- `GET /api/v1/admin/campaigns/export.csv` exports a bounded CSV view of campaigns. User-controlled strings are hardened against spreadsheet formula injection.
- `POST /api/v1/admin/invitations` creates an invitation code for an existing campaign with `code`, `campaign_id`, `max_uses`, and `enabled`.
- `POST /api/v1/admin/allowlist` adds a durable allowlist entry with `campaign_id`, `address`, and optional `note`; duplicate entries are idempotent and preserve the original metadata.

Campaign mutations append durable admin audit entries when SQLite-backed admin storage is available. If durable audit append fails after a campaign create/update/disable, invitation create, or allowlist insert mutation, the admin service attempts to roll the mutation back so unaudited campaign changes do not remain active. The in-memory fallback preserves the same visible campaign/list/disable behavior for tests and standalone partial-store wiring.

### `GET /api/v1/admin/metrics`

Returns lightweight in-process runtime counters and build/runtime metadata. The endpoint is protected by the same bearer-token middleware as the rest of `/api/v1/admin/*`. Counters reset when the faucet process restarts and are intended for immediate operational diagnostics, not durable accounting. Post Phase 17.3.3, the response also includes token-scoped counters under `tokens`; the special `default` token id represents requests that omitted `token_id` at the HTTP boundary and therefore used the configured default-token path.

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
  "captcha": {
    "failed": 1
  },
  "rate_limits": {
    "limited": 1
  },
  "budgets": {
    "daily_exceeded": 1
  },
  "tokens": [
    {
      "token_id": "default",
      "accepted": 8,
      "rejected": 1,
      "rate_limited": 0,
      "daily_exceeded": 0,
      "invalid_token": 0
    },
    {
      "token_id": "scav",
      "accepted": 4,
      "rejected": 2,
      "rate_limited": 1,
      "daily_exceeded": 1,
      "invalid_token": 0
    }
  ]
}
```

Operational notes:

- `claims.*`, `captcha.*`, `rate_limits.*`, and `budgets.*` remain aggregate process-local counters.
- `tokens[*]` is diagnostic only and is not a durable accounting ledger.
- Token ids are configuration/public-catalog identifiers only; wallet addresses, raw fingerprints, request bodies, captcha tokens, and idempotency keys are not exposed.
- Invalid token attempts are counted both in aggregate `claims.invalid_token` and in the bounded token-scoped `invalid` bucket; the supplied untrusted `token_id` is not exported as a label.

### `GET /api/v1/admin/wallet`

Returns operator-safe wallet runtime visibility. The route is protected by the existing admin bearer-token middleware and is disabled with `503` when `SCAVIUM_FAUCET_ADMIN_TOKEN` is empty. The response never exposes private keys, admin tokens, authorization headers, RPC credentials, request bodies, captcha tokens, fingerprints, or idempotency keys.

```json
{
  "enabled": true,
  "status": "ok",
  "address": "0x52908400098527886E0F7030069857D2E4169EE7",
  "native_balance_wei": "1000000000000000000",
  "pending_nonce": 42,
  "tokens": [
    {
      "token_id": "native",
      "symbol": "SCAV",
      "type": "native",
      "balance_wei": "1000000000000000000",
      "status": "ok"
    },
    {
      "token_id": "erc20-demo",
      "symbol": "DEMO",
      "type": "erc20",
      "address": "0x0000000000000000000000000000000000000001",
      "balance_wei": "250000000000000000000",
      "status": "ok"
    }
  ]
}
```

`pending_nonce` uses the selected RPC client's pending nonce API when available, so it includes mempool-visible transactions rather than only confirmed chain state. If a balance or token read fails, the endpoint remains available with `status: "degraded"` and a safe error string. In dry-run mode, or when no real signer/client exists, it returns `enabled:false` and `status:"disabled"`.

### `GET /api/v1/admin/runtime`

Returns dashboard, readiness, runtime metrics, and the same wallet object returned by `/api/v1/admin/wallet` under `wallet`. This keeps Phase 22 visibility available from the existing runtime surface without changing public API contracts.

### `GET /api/v1/admin/dashboard`

Returns the admin summary. The faucet `mode` is runtime-effective for claim intake after the Phase 18.7 post-audit fix. In Phase 20, `claim_counts` and `blocklist_size` are derived from persisted SQLite state.

```json
{
  "mode": "active",
  "claim_counts": {},
  "blocklist_size": 0,
  "updated_at": "2026-05-03T12:00:00Z"
}
```

### `GET /api/v1/admin/queue?limit=50`

Returns an operator-safe queue snapshot from persisted SQLite claim/queue state. `limit` controls only the returned `items` slice and is capped at `500`.

```json
{
  "counts": {
    "queued": 2,
    "sending": 1,
    "failed": 1
  },
  "ready": 2,
  "delayed": 0,
  "in_flight": 1,
  "pending_tx": 0,
  "terminal": 1,
  "items": [
    {
      "id": "claim_123",
      "status": "queued",
      "token_id": "default",
      "token_symbol": "SCAV",
      "retry_count": 0,
      "created_at": "2026-05-03T12:00:00Z",
      "updated_at": "2026-05-03T12:00:00Z"
    }
  ],
  "updated_at": "2026-05-03T12:00:00Z"
}
```

Queue item responses intentionally omit wallet addresses, idempotency keys, request bodies, captcha tokens, and transaction internals. Phase 20.1 moves this read surface to persisted SQLite state.

### `POST /api/v1/admin/queue/retry`

Request body:

```json
{ "id": "claim_123" }
```

Success response:

```json
{ "status": "retried", "id": "claim_123" }
```

Phase 20.2 applies this transition to persisted SQLite claim state. Eligible `failed` and `rejected` claims are moved to `queued` and `next_attempt_at` is cleared so worker pickup can resume immediately.

Conflict response:

- `409` / `not_retryable`

### `POST /api/v1/admin/queue/cancel`

Request body:

```json
{ "id": "claim_123" }
```

Success response:

```json
{ "status": "cancelled", "id": "claim_123" }
```

Phase 20.2 applies this transition to persisted SQLite claim state for eligible not-yet-sent claims.

Conflict response:

- `409` / `not_cancellable`

### `GET /api/v1/admin/claims?limit=50&offset=0`

Lists persisted claims from the SQLite-backed admin read model. `limit` is capped at `500`.

```json
{
  "claims": []
}
```

### `GET /api/v1/admin/claim/{id}`

Returns one claim or `404`.

### `POST /api/v1/admin/claim/{id}/retry`

Moves a `failed` or `rejected` claim back to `queued`.

Phase 20.2 persists this transition in SQLite and clears `next_attempt_at`.

Success response:

```json
{ "status": "retried" }
```

Conflict response:

- `409` / `not_retryable`

### `POST /api/v1/admin/claim/{id}/cancel`

Rejects a claim that has not been sent yet.

Phase 20.2 persists this transition in SQLite.

Success response:

```json
{ "status": "cancelled" }
```

Conflict response:

- `409` / `not_cancellable`

### `POST /api/v1/admin/faucet/mode`

Accepted modes are `active`, `paused`, and `maintenance`. Invalid modes return `400 invalid_mode`. The selected mode is propagated to the live faucet runtime.

Request body:

```json
{ "mode": "paused" }
```

Success response:

```json
{ "mode": "paused" }
```

The admin service records an audit entry and, after Phase 18.7, propagates the accepted mode to the live claim path.

### `GET /api/v1/admin/blocklist`

Returns persisted admin blocklist entries from SQLite.

```json
{
  "entries": []
}
```

### `POST /api/v1/admin/blocklist`

`key_type` must be one of `ip`, `address`, or `fingerprint`; invalid values return `400 invalid_key_type`.

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

Notes:

- Blocklist values are canonicalized before persistence using the existing abuse key normalization rules.
- Adding the same `(key_type, value)` pair updates its reason/timestamp.

### `DELETE /api/v1/admin/blocklist?key_type=ip&value=1.2.3.4`

`key_type` must be one of `ip`, `address`, or `fingerprint`; invalid values return `400 invalid_key_type`.

Success response:

```json
{ "status": "unblocked" }
```

### `GET /api/v1/admin/policy`

Returns the persisted runtime policy override set for the minimal non-secret Phase 28 subset. The route is admin-protected and never returns secrets.

### `PUT /api/v1/admin/policy`

Replaces the runtime policy override set. The body may include `cooldown_seconds`, `rate_limit_ip_per_hour`, `rate_limit_addr_per_day`, `daily_budget_wei`, and `token_daily_budget_wei`. Values must be non-negative decimal integers. The mutation writes a durable admin audit entry.

### `DELETE /api/v1/admin/policy`

Clears all persisted runtime policy overrides and immediately restores environment/default behavior for the editable subset. The mutation writes a durable admin audit entry.

### `GET /api/v1/admin/audit?limit=100`

Returns recent persisted admin audit entries from SQLite. `limit` is capped at `500`.

Sensitive material is excluded from durable audit rows. In particular, bearer tokens,
raw blocklist values, captcha tokens, request bodies, idempotency keys, and secrets
are not stored.

```json
{
  "entries": [
    {
      "action": "set_mode",
      "actor": "127.0.0.1",
      "target": "faucet",
      "detail": "paused",
      "created_at": "2026-05-03T12:00:00Z"
    }
  ]
}
```

## Token-aware claim enforcement

`POST /api/v1/claim` continues to accept an optional `token_id`. After token validation, persisted blocklist enforcement runs before captcha/risk/cooldown/rate-limit work where applicable. Blocklisted requests keep the existing `claim_rejected` contract and return a safe generic reason. Cooldown and rate-limit enforcement are then applied to the resolved token scope. Invalid tokens still use `claim_rejected` with reason `invalid_token`, cooldown uses `cooldown_active`, rate limiting uses `rate_limited`, and daily budget exhaustion uses `daily_budget_exceeded`.


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


## Phase 25 — Public API completion and OpenAPI

Phase 25 adds the public address history endpoint, extends wallet/address eligibility status with optional token and daily-budget visibility, and introduces a manually maintained lightweight OpenAPI contract at `openapi.yaml`. All changes are backward-compatible: `POST /api/v1/claim`, normalized error envelopes, request/correlation headers, admin bearer-token protection, and existing alias routes remain intact.

## Phase 26 — Public frontend consumption of Phase 25 APIs

The embedded public frontend now consumes the Phase 25 address status and history endpoints directly from the browser. The address typed into the claim form can be used to check `GET /api/v1/address/{address}/status` and to render the first page of `GET /api/v1/address/{address}/history?limit=10&offset=0`.

This is a client-side UX addition only. The backend API contracts remain unchanged: claim creation, claim lookup, status, token catalog, address status, address history, normalized error envelopes, request IDs, admin authentication, and alias routes keep their documented behavior. The frontend renders only the public-safe fields returned by those endpoints and never displays idempotency keys, captcha tokens, fingerprints, raw abuse signals, private blocklist notes, or admin-only queue/audit metadata.

Explorer links in the claim result and history panels are optional. The browser renders them only when `explorer_tx_url` is an absolute HTTP(S) template with a `{txHash}` placeholder and the claim contains a valid 32-byte EVM transaction hash. Missing, relative, non-HTTP(S), or malformed values result in no link.

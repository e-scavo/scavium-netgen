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
- ERC20 clients should first read this catalog and then submit the selected `id` as `token_id` to `POST /api/v1/claim`.
- See [token-registration.md](token-registration.md) for the testnet registration checklist.

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

Admin routes are active when `SCAVIUM_FAUCET_ADMIN_TOKEN` is set. `app.New` passes the token into `httpapi.Dependencies.AdminToken`; when the token is empty the routes return `503`.

When the handler is constructed with an admin token, every admin request must include:

```text
Authorization: Bearer <SCAVIUM_FAUCET_ADMIN_TOKEN>
```

Wrong or missing token returns `401`. Empty configured token returns `503`.

### `GET /api/v1/admin/metrics`

Returns lightweight in-process runtime counters and build/runtime metadata. The endpoint is protected by the same bearer-token middleware as the rest of `/api/v1/admin/*`. Counters reset when the faucet process restarts and are intended for immediate operational diagnostics, not durable accounting.

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
    "claim_unavailable": 0
  },
  "captcha": {
    "failed": 1
  },
  "rate_limits": {
    "limited": 1
  },
  "budgets": {
    "daily_exceeded": 1
  }
}
```

### `GET /api/v1/admin/dashboard`

Returns the in-memory admin summary:

```json
{
  "mode": "active",
  "claim_counts": {},
  "blocklist_size": 0,
  "updated_at": "2026-05-03T12:00:00Z"
}
```

### `GET /api/v1/admin/claims?limit=50&offset=0`

Lists in-memory claims.

```json
{
  "claims": []
}
```

### `GET /api/v1/admin/claim/{id}`

Returns one claim or `404`.

### `POST /api/v1/admin/claim/{id}/retry`

Moves a `failed` or `rejected` claim back to `queued`.

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

Success response:

```json
{ "mode": "paused" }
```

The in-memory admin service accepts the new mode string and records an audit entry.

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

### `DELETE /api/v1/admin/blocklist?key_type=ip&value=1.2.3.4`

Success response:

```json
{ "status": "unblocked" }
```

### `GET /api/v1/admin/audit?limit=100`

Returns recent in-memory audit entries.

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

# API reference

Base path: `/api/v1`

All API responses are JSON. Every request gets an `X-Request-ID` response header; if the client does not send one, the server generates one.

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
  "time": "2026-05-03T12:00:00Z"
}
```

### `GET /ready`

Readiness endpoint backed by `ready.DefaultChecks()`.

Today those checks are stubs, so the shipped binary reports an aggregate of named checks such as `db`, `queue`, `rpc`, and `wallet`, but it is still a shallow readiness signal.

Healthy example:

```json
{
  "status": "ok",
  "checks": [
    { "name": "db", "status": "ok" },
    { "name": "queue", "status": "ok" },
    { "name": "rpc", "status": "ok" },
    { "name": "wallet", "status": "ok" }
  ],
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

The current in-memory read service always reports `status: "active"`.

### `GET /api/v1/config`
### `GET /api/v1/faucet/config`

Alias routes for public faucet configuration.

```json
{
  "network_name": "scavium-test",
  "chain_id": 123,
  "symbol": "tSCAV",
  "amount_wei": "42",
  "cooldown_seconds": 60,
  "explorer_tx_url": "https://explorer.example.test/tx/{txHash}",
  "dry_run": false,
  "rate_limit_ip_per_hour": 10,
  "rate_limit_addr_per_day": 3
}
```

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
  "address": "0x52908400098527886E0F7030069857D2E4169EE7"
}
```

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
  "status": "queued",
  "idempotency_key": "same-key-for-retries",
  "created_at": "2026-05-03T12:00:00Z",
  "updated_at": "2026-05-03T12:00:00Z"
}
```

Current behavior:

- only `address` is decoded from the body
- the body is capped at `1 MiB`
- the claim is stored in memory with initial status `queued`
- repeated requests with the same `Idempotency-Key` return the same claim

Common errors:

| HTTP | Code | Meaning |
|---|---|---|
| 400 | `invalid_json` | Malformed JSON body |
| 400 | `invalid_address` | Address failed validation |
| 500 | `claim_unavailable` | Claim service failure |

### `GET /api/v1/claim/{id}`
### `GET /api/v1/faucet/claim/{id}`

Alias routes for claim lookup.

```json
{
  "id": "claim_test",
  "address": "0x52908400098527886E0F7030069857D2E4169EE7",
  "amount_wei": "42",
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

The handler package implements admin routes under `/api/v1/admin/*`, but the shipped binary currently does not pass `AdminToken` into `httpapi.NewHandler`, so these routes return `503` when accessed through `main.go`/`app.New`.

When the handler is constructed with an admin token, every admin request must include:

```text
Authorization: Bearer <SCAVIUM_FAUCET_ADMIN_TOKEN>
```

Wrong or missing token returns `401`. Empty configured token returns `503`.

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

# API Reference

Base path: `/api/v1`

All responses are `application/json`. Error envelopes follow the shape:

```json
{
  "code": "RATE_LIMIT_EXCEEDED",
  "message": "Too many requests from this IP.",
  "details": {},
  "request_id": "abc123"
}
```

Every request receives an `X-Request-ID` response header for tracing.

---

## Public endpoints

### `GET /health`

Liveness probe. Returns `200 OK` while the process is running.

**Response**

```json
{ "status": "ok", "time": "2024-01-01T00:00:00Z" }
```

---

### `GET /ready`

Readiness probe. Checks DB connectivity, RPC reachability, wallet balance, and queue health.

**Response — healthy**

```
200 OK
{ "status": "ready" }
```

**Response — degraded**

```
503 Service Unavailable
{ "status": "not ready", "checks": { "rpc": "timeout" } }
```

---

### `GET /api/v1/faucet/status`

Returns the current operational status of the faucet.

**Response**

```json
{
  "mode": "active",
  "paused": false,
  "maintenance": false,
  "network": "scavium-testnet",
  "symbol": "SCAV"
}
```

`mode` is one of `active`, `paused`, `maintenance`.

---

### `GET /api/v1/faucet/config`

Returns public parameters visible to clients and the SCAVIUM Wallet.

**Response**

```json
{
  "amount_wei": "1000000000000000000",
  "cooldown_seconds": 86400,
  "network_name": "scavium-testnet",
  "chain_id": 1337,
  "symbol": "SCAV",
  "explorer_tx_url": "https://explorer.scavium.io/tx/"
}
```

---

### `POST /api/v1/faucet/claim`

Submit a fund request.

**Request body**

```json
{
  "address": "0xAbCd…",
  "captcha_token": "<provider-token>"
}
```

**Response — accepted**

```
202 Accepted
{
  "claim_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "pending"
}
```

**Error codes**

| HTTP | Code | Meaning |
|---|---|---|
| 400 | `INVALID_ADDRESS` | Address format or checksum invalid |
| 400 | `CAPTCHA_FAILED` | Captcha verification failed |
| 429 | `RATE_LIMIT_IP` | IP hourly limit reached |
| 429 | `RATE_LIMIT_ADDRESS` | Address daily limit reached |
| 429 | `COOLDOWN_ACTIVE` | Address is in cooldown period |
| 503 | `FAUCET_PAUSED` | Faucet is paused or in maintenance |
| 507 | `BUDGET_EXHAUSTED` | Daily budget fully disbursed |

---

### `GET /api/v1/faucet/claim/{id}`

Poll the status of a previously submitted claim.

**Response**

```json
{
  "claim_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "sent",
  "tx_hash": "0xabc…",
  "amount_wei": "1000000000000000000",
  "created_at": "2024-01-01T12:00:00Z"
}
```

`status` values: `pending`, `sent`, `confirmed`, `failed`.

---

### `GET /api/v1/faucet/address/{address}/eligibility`

Returns whether an address can request funds, and how long until the next claim is allowed.

**Response**

```json
{
  "eligible": false,
  "cooldown_remaining_seconds": 72340,
  "reason": "cooldown"
}
```

`reason` when `eligible=false`: `cooldown`, `rate_limit`, `blocked`, `faucet_paused`.

---

### `GET /api/v1/version`

Returns build metadata.

**Response**

```json
{
  "version": "v1.2.0",
  "commit": "abc1234",
  "built_at": "2024-01-01T00:00:00Z"
}
```

---

## Admin endpoints

All admin endpoints require the header:

```
Authorization: Bearer <SCAVIUM_FAUCET_ADMIN_TOKEN>
```

If `SCAVIUM_FAUCET_ADMIN_TOKEN` is not set, all admin endpoints return `503`.

---

### `GET /api/v1/admin/dashboard`

Returns aggregate stats: total claims today, amount disbursed, wallet balance.

---

### `GET /api/v1/admin/claims`

List claims with optional filters (`?status=pending&page=1`).

---

### `GET /api/v1/admin/claim/{id}`

Retrieve full claim detail including internal fields.

---

### `PUT /api/v1/admin/claim/{id}`

Update a claim (approve/reject manual review cases).

---

### `POST /api/v1/admin/faucet/mode`

Change operational mode.

**Request body**

```json
{ "mode": "paused" }
```

---

### `GET /api/v1/admin/blocklist`

List blocked IPs and addresses.

### `POST /api/v1/admin/blocklist`

Add an entry to the blocklist.

**Request body**

```json
{ "type": "ip", "value": "1.2.3.4", "reason": "abuse" }
```

### `DELETE /api/v1/admin/blocklist/{id}`

Remove a blocklist entry.

---

### `GET /api/v1/admin/audit`

Retrieve the admin audit log (paginated).

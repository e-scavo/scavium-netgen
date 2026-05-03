# Architecture

## Overview

```
                      ┌──────────────────────────────────────────────────┐
                      │                     nginx (TLS)                  │
                      │  :443 → 127.0.0.1:18080 (proxy_pass)            │
                      └──────────────────┬───────────────────────────────┘
                                         │
                      ┌──────────────────▼───────────────────────────────┐
                      │           scavium-faucet (Go binary)             │
                      │                                                  │
                      │  ┌────────────┐  ┌──────────────┐               │
                      │  │  frontend  │  │   httpapi    │               │
                      │  │  (web UI)  │  │  (REST API)  │               │
                      │  └────────────┘  └──────┬───────┘               │
                      │                         │                        │
                      │  ┌──────────────────────▼─────────────────────┐ │
                      │  │               Internal services             │ │
                      │  │  config │ faucet │ worker │ chain │ store  │ │
                      │  │  abuse  │ captcha│ ready  │ admin │ iputil │ │
                      │  └────────────────────────────────────────────┘ │
                      └──────────────────────────────────────────────────┘
                               │                        │
                    ┌──────────▼──────────┐  ┌──────────▼──────────┐
                    │   Besu RPC node     │  │   SQLite / Postgres  │
                    │  (EVM JSON-RPC)     │  │   (claim store)      │
                    └─────────────────────┘  └─────────────────────-┘
```

The binary listens on `127.0.0.1:18080` by default; nginx terminates TLS and proxies to it.

---

## Package layout

| Package | Responsibility |
|---|---|
| `cmd/scavium-faucet` | `main.go` — wires config, DB, services, and HTTP server |
| `internal/config` | Load and validate runtime configuration from environment |
| `internal/httpapi` | HTTP router, public endpoints, admin endpoints, middleware |
| `internal/frontend` | Embedded static web UI served at `/` |
| `internal/faucet` | Domain logic: claim validation, read/write service interfaces |
| `internal/worker` | Background queue that signs and broadcasts transactions |
| `internal/chain` | Ethereum client wrapper (nonce manager, tx signer) |
| `internal/store` | Persistence layer (claim storage, audit log) |
| `internal/abuse` | Rate-limit, blocklist, cooldown enforcement |
| `internal/captcha` | Captcha provider abstraction (Turnstile, hCaptcha, reCAPTCHA, dev) |
| `internal/ready` | Readiness checks (DB, RPC, wallet, queue, balance) |
| `internal/admin` | Admin service (mode changes, blocklist, audit access) |
| `internal/domain` | Shared value types (`Claim`, `Address`, `Mode`, …) |
| `internal/iputil` | Trusted-proxy-aware real-IP extraction |
| `internal/observability` | Structured logging helpers |
| `internal/version` | Build metadata embedded at compile time |

---

## Request lifecycle — public claim

```
Client
  │
  ▼
POST /api/v1/faucet/claim  {address, captcha_token}
  │
  ├─ middleware: request-ID, real-IP, trusted-proxy check
  ├─ abuse: rate-limit by IP (per-hour)
  ├─ abuse: rate-limit by address (per-day)
  ├─ captcha: verify token with provider
  ├─ faucet: validate address format + chain-ID
  ├─ abuse: cooldown check for address
  ├─ faucet: global daily budget guard
  ├─ store: persist claim (status=pending)
  ├─ worker: enqueue for signing
  └─ response: {claim_id, status="pending"}

Worker (background goroutine)
  │
  ├─ chain: acquire nonce lock
  ├─ chain: sign and broadcast transaction
  ├─ store: update claim (status=sent, txHash)
  └─ chain: release nonce lock
```

---

## Operational modes

| Mode | Behaviour |
|---|---|
| `active` | Normal operation; claims are accepted and processed |
| `paused` | Claim endpoint returns 503; read-only endpoints remain up |
| `maintenance` | All endpoints return maintenance notice; admin still accessible |

Mode is set via `SCAVIUM_FAUCET_MODE` or dynamically via the admin API.

---

## Data stores

### Claim table (SQLite / Postgres)

| Column | Type | Description |
|---|---|---|
| `id` | UUID | Public claim identifier |
| `address` | TEXT | Requester EVM address |
| `ip` | TEXT | Source IP (hashed for privacy) |
| `status` | TEXT | `pending`, `sent`, `confirmed`, `failed` |
| `tx_hash` | TEXT | On-chain tx hash once broadcast |
| `amount_wei` | TEXT | Amount delivered |
| `created_at` | TIMESTAMP | Request timestamp |
| `updated_at` | TIMESTAMP | Last status change |

### Audit log table

Append-only log of admin actions: mode changes, blocklist mutations, manual reviews.

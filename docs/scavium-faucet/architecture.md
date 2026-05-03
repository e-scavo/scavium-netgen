# Architecture

## Runtime overview

The current binary is much smaller than the long-term faucet design. `main.go` loads config, builds `app.New(cfg)`, and starts one `http.Server` with fixed timeouts. `app.New` then wires a single handler tree with:

- `ready.DefaultChecks()` for `/ready`
- `faucet.NewInMemoryReadService(cfg)` for public read and claim routes
- `frontend.Handler()` for the embedded web UI
- default `version.Current()` for `/api/v1/version`

The shipped app does **not** wire the admin token or a persistent store into `httpapi.NewHandler`.

```text
client
  |
  v
scavium-faucet http.Server
  |
  +-- /health, /ready, /api/v1/*
  |     |
  |     +-- RequestIDMiddleware
  |     +-- httpapi public handlers
  |     +-- ready.DefaultChecks()         -> stub "ok" checks
  |     +-- faucet.InMemoryReadService    -> in-memory claims/idempotency
  |     `-- version.Current()
  |
  `-- / and non-/api paths
        `-- frontend.Handler() -> embedded static files
```

## Request flow

### Public claim creation

```text
POST /api/v1/claim
  |
  +-- max 1 MiB JSON body
  +-- decode {"address": "..."}
  +-- domain.ValidateAddress(...)
  +-- optional Idempotency-Key header lookup
  +-- create in-memory claim with status "queued"
  `-- 202 Accepted with claim payload
```

There is no RPC send, worker pickup, captcha verification, or rate-limit enforcement in the shipped binary yet. Claims are accepted into memory only.

### Claim lookup

`GET /api/v1/claim/{id}` returns the stored in-memory claim record or a JSON `404`.

### Frontend routing

`frontend.Handler()` serves embedded files directly and falls back to `index.html` for unmatched non-API paths so the single-page UI can own client-side routing.

## State model

The live binary currently keeps state in memory:

- `claimsByID map[string]domain.Claim`
- `claimIDsByIdem map[string]string`
- default admin service state if the handler is constructed with it

That has two immediate consequences:

1. Restarting the process drops claims, blocklist entries, and audit history.
2. There is no queue recovery or durable audit trail in the shipped app yet.

## Package roles

| Package | Current role |
|---|---|
| `cmd/scavium-faucet` | Binary entrypoint and server startup |
| `internal/config` | Loads and validates environment configuration |
| `internal/httpapi` | Route registration, JSON helpers, request IDs, admin middleware |
| `internal/frontend` | Embedded UI served at `/` |
| `internal/faucet` | In-memory public read/claim service |
| `internal/ready` | Stub readiness checks and aggregate result shaping |
| `internal/admin` | In-memory admin service and bearer-token middleware; not wired by `app.New` |
| `internal/domain` | Shared claim and status types plus address validation |
| `internal/observability` | Structured JSON logger |
| `internal/version` | Build metadata payload for `/api/v1/version` |
| `internal/iputil` | Trusted-proxy IP helper present in repo but not wired into HTTP handlers yet |
| `internal/abuse`, `internal/captcha`, `internal/chain`, `internal/store`, `internal/worker` | Supporting packages and future-facing implementation work not active in the shipped runtime |

## Startup and shutdown

- The server listens on `SCAVIUM_FAUCET_BIND_ADDR` and defaults to `127.0.0.1:18080`.
- Timeouts are set in `main.go`: `ReadHeaderTimeout=5s`, `ReadTimeout=10s`, `WriteTimeout=10s`, `IdleTimeout=60s`.
- SIGINT and SIGTERM trigger graceful shutdown with a `10s` timeout.

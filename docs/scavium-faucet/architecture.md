# Architecture

## Runtime overview

`main.go` loads config, builds `app.New(cfg)`, and starts one `http.Server` with fixed timeouts. `app.New` wires the full persistent runtime:

- opens SQLite (WAL mode, 5 s busy timeout) from `SCAVIUM_FAUCET_DATABASE_PATH` and runs embedded migrations automatically
- `faucet.NewPersistentReadService(cfg, store, store, store)` for public read and claim routes
- SQLite-backed abuse signal recorder attached to the persistent read service
- captcha verifier selected by `SCAVIUM_FAUCET_CAPTCHA_PROVIDER` (`disabled`, `dev`, `hcaptcha`, `recaptcha`, `turnstile`)
- `runtimeChecks()` — real DB and queue probes; RPC and wallet probes when not in dry-run mode
- `AdminToken: cfg.AdminToken` passed into `httpapi.Dependencies`, enabling the admin API when the token is set
- `TrustedProxy` wired for trusted-proxy IP extraction, fingerprint, and user-agent forwarding
- background worker (enabled by default) processing the SQLite-backed claim queue
- background watcher polling on-chain confirmations (auto-enabled in non-dry-run mode)
- `frontend.Handler()` for the embedded web UI
- `version.Current()` for `/api/v1/version`

```text
client
  |
  v
scavium-faucet http.Server
  |
  +-- /health, /ready, /api/v1/*
  |     |
  |     +-- CORSHandler                  -> exact-origin CORS for public paths
  |     +-- RequestIDMiddleware
  |     +-- RequestLoggingMiddleware      -> structured JSON access log per request
  |     +-- httpapi public handlers
  |     +-- runtimeChecks()               -> real DB/queue (+ RPC/wallet) probes
  |     +-- faucet.PersistentReadService  -> SQLite-backed claims/idempotency
  |     +-- captcha verifier              -> configured provider or nil
  |     +-- abuse signal recorder         -> SQLite-backed non-blocking telemetry
  |     +-- rate limiter                  -> SQLite-backed IP/address limits
  |     +-- admin middleware              -> bearer token when AdminToken set
  |     `-- version.Current()
  |
  +-- background: worker                  -> polls SQLite queue, dispatches sender
  +-- background: watcher (non-dry-run)   -> polls chain confirmations
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
  +-- extract RemoteIP (TrustedProxy-aware), UserAgent, CaptchaToken, Fingerprint
  +-- decode {"address": "..."}
  +-- domain.ValidateAddress(...)
  +-- optional Idempotency-Key header lookup
  +-- captcha verification (if provider configured)
  +-- risk evaluation
  +-- record captcha/risk/cooldown/rate-limit/budget/accepted-claim signals
  +-- persistent rate-limit check (IP per hour, address per day, fingerprint)
  +-- daily budget check (if DAILY_BUDGET_WEI set)
  +-- persist claim to SQLite with status "received"
  `-- 202 Accepted with claim payload
```

The background worker picks up queued claims, calls the configured sender (real `EthSender` or `DryRunSender`), and advances claim status through `sending` → `sent`. The watcher (non-dry-run only) polls for on-chain confirmations and advances status to `confirmed` or `failed`.

### Claim lookup

`GET /api/v1/claim/{id}` returns the persisted SQLite claim record or a JSON `404`.

### Frontend routing

`frontend.Handler()` serves embedded files directly and falls back to `index.html` for unmatched non-API paths so the single-page UI can own client-side routing.

## State model

The live binary persists state in SQLite (WAL journal mode, 5 s busy timeout):

- claim records with full lifecycle status (`received` → `queued` → `sending` → `sent` → `confirmed` / `failed`)
- idempotency key index for duplicate-safe claim submission
- rate-limit counters per IP (hourly) and per address (daily)
- queue metadata (`next_attempt_at`, retry count) used by the background worker
- abuse signals with kind, address, remote IP, fingerprint, user-agent, claim ID, reason, score, and timestamp

Migrations (`001_initial.sql`, `002_queue.sql`, `003_abuse_signals.sql`) run automatically on startup inside `sqlite.Open()`. Restarting the process does not lose queued or in-flight claims or recorded abuse observations.

## Package roles

| Package | Current role |
|---|---|
| `cmd/scavium-faucet` | Binary entrypoint and server startup |
| `internal/config` | Loads and validates environment configuration |
| `internal/httpapi` | Route registration, JSON helpers, request IDs, CORS middleware, request-logging middleware, admin middleware |
| `internal/frontend` | Embedded UI served at `/` |
| `internal/faucet` | SQLite-backed persistent read/claim service (`PersistentReadService`) |
| `internal/ready` | Real DB/queue probes, optional RPC/wallet probes, and aggregate result shaping |
| `internal/admin` | In-memory admin service and bearer-token middleware; enabled when `AdminToken` is set |
| `internal/domain` | Shared claim, abuse signal, status, and validation contracts |
| `internal/observability` | Structured JSON logger |
| `internal/version` | Build metadata payload for `/api/v1/version` |
| `internal/iputil` | Trusted-proxy IP extraction; wired into claim handler |
| `internal/captcha` | Captcha verifier selection and HTTP-based verification; wired into `PersistentReadService` |
| `internal/abuse` | Risk evaluation helpers used during claim creation |
| `internal/chain` | RPC client, signer, `EthSender`, `DryRunSender`, and `Watcher`; wired at startup based on `DryRun` |
| `internal/store/sqlite` | SQLite persistence layer: claims, queue, rate limits, abuse signals, migrations |
| `internal/worker` | Background worker polling SQLite queue and dispatching the configured sender |

## Startup and shutdown

- The server listens on `SCAVIUM_FAUCET_BIND_ADDR` and defaults to `127.0.0.1:18080`.
- Timeouts are set in `main.go`: `ReadHeaderTimeout=5s`, `ReadTimeout=10s`, `WriteTimeout=10s`, `IdleTimeout=60s`.
- SIGINT and SIGTERM trigger graceful shutdown with a `10s` timeout.

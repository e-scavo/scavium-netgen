# Architecture

## Runtime overview

`main.go` loads config, builds `app.New(cfg)`, and starts one `http.Server` with fixed timeouts. `app.New` wires the full persistent runtime:

- opens SQLite (WAL mode, 5 s busy timeout) from `SCAVIUM_FAUCET_DATABASE_PATH` and runs embedded migrations automatically
- `faucet.NewPersistentReadService(cfg, store, store, store)` for public read and claim routes
- SQLite-backed abuse signal recorder attached to the persistent read service
- captcha verifier selected by `SCAVIUM_FAUCET_CAPTCHA_PROVIDER` (`disabled`, `dev`, `hcaptcha`, `recaptcha`, `turnstile`)
- `runtimeChecks()` — real DB and queue probes; RPC and wallet probes when not in dry-run mode
- `observability.NewRuntimeMetrics(version.Current())` — process-local counters and build/runtime metadata for health and admin metrics
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
  |     +-- RequestIDMiddleware            -> request ID + correlation ID
  |     +-- RequestLoggingMiddleware      -> structured JSON access log per request
  |     +-- httpapi public handlers
  |     +-- runtimeChecks()               -> real DB/queue (+ RPC/wallet) probes
  |     +-- RuntimeMetrics                -> process-local counters + uptime/build metadata
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
| `internal/httpapi` | Route registration, JSON helpers, request IDs, correlation IDs, CORS middleware, request-logging middleware, admin middleware, health/ready/metrics handlers |
| `internal/frontend` | Embedded UI served at `/` |
| `internal/faucet` | SQLite-backed persistent read/claim service (`PersistentReadService`) |
| `internal/ready` | Real DB/queue probes, optional RPC/wallet probes, and aggregate result shaping |
| `internal/admin` | In-memory admin service and bearer-token middleware; enabled when `AdminToken` is set |
| `internal/domain` | Shared claim, abuse signal, status, and validation contracts |
| `internal/observability` | Structured JSON logger and lightweight process-local runtime metrics |
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


## Phase 15.3 — Progressive Abuse Enforcement

The claim path now wires `internal/abuse.ProgressiveEnforcer` as the read service risk engine. The enforcer does not persist state itself; it queries aggregate counts through `domain.AbuseSignalCounter`, implemented by the SQLite store over the `abuse_signals` table introduced in Phase 15.2.

The design keeps enforcement inside the existing claim intake pipeline:

1. Captcha verification still runs first when configured.
2. Progressive enforcement counts recent negative signals by IP, address, and fingerprint.
3. If a configured threshold is reached, the existing risk rejection path returns `claim_rejected` and records a new `risk_rejected` signal.
4. Cooldown, rate limits, budget checks, claim persistence, queueing, and worker processing remain unchanged.

This preserves the public API surface while allowing production operators to activate a measured anti-abuse brake before moving to explicit blocklists and adaptive throttling.


## Phase 15.4 — Abuse Operations & Retention

The abuse signal ledger is now treated as an operational dataset with a bounded retention policy. `internal/config` owns the retention knob, `internal/abuse` owns the retention helper, and `internal/store/sqlite` owns the database-level delete and aggregate summary queries.

Startup wiring remains conservative: after migrations complete and before sender/worker/watchers are started, `app.NewWithLogger` prunes expired abuse signals using the configured retention window. A pruning error fails startup because it indicates the same persistence layer used by enforcement and claim intake is unhealthy.

No public API route was added. The new summary contract is internal-only and exists to make Phase 16 metrics and later admin surfaces depend on a stable domain boundary instead of ad-hoc SQL.

## Phase 15.close — Abuse Protection Closure

The Phase 15 architecture is closed around a conservative claim-intake control loop: captcha verification, abuse signal recording, progressive signal-based enforcement, persistent rate limits, budget checks, claim persistence, and background dispatch remain ordered inside the existing runtime.

No new external service boundary was added beyond the already-configured captcha provider, and no public API route was introduced for abuse operations. The internal contracts added during Phase 15 (`AbuseSignalRecorder`, `AbuseSignalCounter`, `AbuseSignalPruner`, and `AbuseSignalReporter`) now form the stable bridge into Phase 16 metrics and later Phase 18 admin surfaces.

## Phase 16 — Observability & Operations Architecture

Phase 16 adds observability without changing the service topology. The faucet still runs as one loopback-bound Go process behind nginx, with SQLite persistence and the existing background worker/watcher model. The added observability surfaces are internal to the same binary and avoid new external dependencies.

### Request correlation

`RequestIDMiddleware` now manages both `X-Request-ID` and `X-Correlation-ID`. A caller-supplied request ID is preserved; otherwise the backend generates one. A caller-supplied correlation ID is preserved; otherwise it falls back to the request ID. Both values are attached to the request context and echoed as response headers.

Structured access logs include both IDs so nginx, backend logs, claim-flow events, and client-side troubleshooting can be correlated without changing public response contracts.

### Claim-flow logging

The claim handler logs safe structured events for accepted and rejected claims. The fields intentionally avoid sensitive or high-cardinality raw data: no request bodies, wallet addresses, captcha tokens, raw fingerprints, secrets, or idempotency-key values are emitted. Instead, the handler records operational booleans such as whether a fingerprint, captcha token, or idempotency key was present, along with the rejection code and retry metadata when applicable.

### Runtime metrics

`internal/observability.RuntimeMetrics` is constructed during `app.New` and passed into `httpapi.Dependencies`. It uses atomic counters and process-local state only. The snapshot includes:

- process start time and uptime
- build metadata from `internal/version`
- accepted and rejected claim counts
- classified rejection counters for captcha failures, rate-limit hits, daily-budget exceedances, faucet-unavailable, claim-unavailable, and risk rejections

`GET /api/v1/admin/metrics` exposes that snapshot behind the existing admin bearer-token middleware. Counters reset on process restart and are diagnostic rather than durable accounting; durable claim and abuse data remains in SQLite.

### Health and readiness enrichment

`/health` remains a liveness endpoint and now includes uptime plus build metadata. `/ready` keeps the existing real probes and enriches each check with elapsed duration and a response-level summary. Dry-run deployments still probe DB and queue only; non-dry-run deployments also probe RPC and wallet readiness.

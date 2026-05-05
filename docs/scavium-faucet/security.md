# Security

This document describes the security posture of the faucet **as currently implemented**. It also calls out important gaps so the docs do not overstate protections that are only present in roadmap packages.

## Security properties in the current binary

### HTTP server defaults

- Default bind address is loopback: `127.0.0.1:18080`
- `ReadHeaderTimeout=5s`
- `ReadTimeout=10s`
- `WriteTimeout=10s`
- `IdleTimeout=60s`
- JSON request bodies for write endpoints are capped at `1 MiB`

These are useful baseline protections against accidental exposure and slow or oversized requests.

### Request tracing

Every request carries an `X-Request-ID` response header. If the caller does not provide one, the server generates a random request ID. This makes log correlation and error tracing easier.

### Address validation

Claim creation and address-status lookups validate the supplied EVM address and reject malformed input with a normalized `400 invalid_address` response.

### Admin token comparison

`internal/admin.TokenAuthMiddleware` uses constant-time comparison for bearer token checks. If the handler is instantiated with an admin token, the comparison itself avoids basic timing side channels.

### Structured logging and request logging

The binary logs structured JSON events through `internal/observability`. Startup events (listen address, configuration errors) and per-request access entries are written to stdout as JSON lines. Each access log entry contains: `request_id`, `method`, `path`, `status`, `duration`, and `remote_ip`. Request bodies, captcha tokens, browser fingerprints, and sensitive configuration values (`PRIVATE_KEY`, `ADMIN_TOKEN`, `CAPTCHA_SECRET`) are never included in log output.

### Persistent rate limiting

Claim creation enforces rate limits via the SQLite-backed `RateLimiter`:

- per source IP: `SCAVIUM_FAUCET_RATE_LIMIT_IP_PER_HOUR` requests per hour
- per Ethereum address: `SCAVIUM_FAUCET_RATE_LIMIT_ADDR_PER_DAY` requests per day
- per fingerprint: same hourly limit as IP when a `fingerprint` field is supplied in the claim body

Set `SCAVIUM_FAUCET_TRUSTED_PROXY` to your reverse-proxy address so that IP extraction uses the real client IP rather than `127.0.0.1`.

### Captcha verification

When `SCAVIUM_FAUCET_CAPTCHA_PROVIDER` is set to `hcaptcha`, `recaptcha`, or `turnstile`, the frontend renders the configured public provider widget from `SCAVIUM_FAUCET_CAPTCHA_SITE_KEY` and claim creation verifies the submitted `captcha_token` against the provider endpoint. A failed or missing token causes the claim to be rejected. Provider `dev` always passes (for testing only). Default is `disabled`. `SCAVIUM_FAUCET_CAPTCHA_SECRET` remains server-side only; `SCAVIUM_FAUCET_CAPTCHA_SITE_KEY` is public by design.

### Admin token and persistent claim storage

`app.New` passes `cfg.AdminToken` into `httpapi.Dependencies.AdminToken`, enabling the `/api/v1/admin/*` routes when the token is non-empty. Claims are persisted in SQLite (WAL mode); restarting the process does not lose state.

### Trusted proxy and real IP extraction

`SCAVIUM_FAUCET_TRUSTED_PROXY` controls whether the handler trusts `X-Forwarded-For` / `X-Real-IP` headers. Set to the loopback or proxy address to ensure rate limiting operates on real client IPs.

### Risk engine blocking

When a `domain.RiskEngine` is wired via `SetRiskEngine`, each claim request is evaluated against the engine before insertion. A rejected evaluation returns `403 claim_rejected` with the rejection reason in `details.reason`. The engine receives the request IP, Ethereum address, browser fingerprint, user-agent, and request timestamp. The default `app.New` wiring does not attach a risk engine; operators can inject one for production deployments that require additional abuse signals.

### Durable abuse signals

Claim intake now records production-safe abuse signals in SQLite through `domain.AbuseSignalRecorder`, wired by default in `app.New` using the same persistent store. The signal layer is observational and deliberately non-blocking: a storage failure while recording a signal does not change the public claim response or break existing API contracts.

Recorded events include successful and failed captcha verification, risk-engine allow/reject decisions, cooldown denials, rate-limit denials, daily-budget denials, and accepted claims. Each signal stores the signal kind, Ethereum address when available, trusted-proxy-derived client IP, browser fingerprint when supplied, user-agent, optional claim ID, reason, score, and UTC timestamp. Captcha tokens, private keys, admin tokens, and captcha secrets are not stored.

This gives operators a durable audit base for Phase 15.3+ blocklisting, adaptive throttling, and later admin review without introducing new hard-blocking behavior in 15.2.

### Progressive abuse enforcement

Phase 15.3 turns the Phase 15.2 signal ledger into a conservative runtime control. The faucet now evaluates recent negative signals (`captcha_failed`, `risk_rejected`, `rate_limited`, `cooldown_active`, and `daily_budget_exceeded`) before claim creation proceeds past risk evaluation. Enforcement is scoped independently by source IP, wallet address, and browser fingerprint.

The default posture is intentionally gradual: enforcement is enabled, the lookback window is one hour, and thresholds are high enough to avoid interfering with ordinary users while still giving operators a production-safe brake during abuse bursts. A rejected request uses the existing `claim_rejected` error contract and records a `risk_rejected` abuse signal with the observed score, preserving API compatibility and extending the audit trail.

Operators can disable the full enforcement layer with `SCAVIUM_FAUCET_ABUSE_ENFORCEMENT_ENABLED=false`, or disable one scope by setting its threshold to `0`. This keeps Phase 15.3 deployable without schema changes, new external services, or direct backend exposure.


### Daily budget enforcement

`SCAVIUM_FAUCET_DAILY_BUDGET_WEI` limits the total amount distributed per UTC calendar day. The check is enforced atomically inside a SQLite `BEGIN IMMEDIATE` transaction: the current day's sum across all active claim statuses (`received`, `validated`, `queued`, `sending`, `sent`, `confirmed`) is read, and the insert is rejected if adding the new claim would exceed the configured budget. A rejected claim returns `429 daily_budget_exceeded` with the used, requested, and maximum amounts in `details.reason`. The budget resets automatically at UTC midnight. An unset or zero budget means unlimited.

### CORS policy

`CORSHandler` applies exact-origin CORS to public API routes. Behaviour:

- **Empty `SCAVIUM_FAUCET_CORS_ALLOWED_ORIGINS`**: no CORS headers are emitted. This is the safe default.
- **Configured origins**: each request `Origin` is matched exactly against the allowed set. Only matching origins receive `Access-Control-Allow-Origin`.
- `Vary: Origin` is set when CORS is active to prevent cache poisoning.
- OPTIONS preflight requests return `204 No Content` for allowed origins.
- Admin paths (`/api/v1/admin/*`) are excluded from CORS regardless of configuration.
- Wildcard `*` in the allowed origins list is rejected at startup by `Config.Validate()`.

### Admin API isolation

Admin endpoints (`/api/v1/admin/*`) are protected by bearer-token authentication and are explicitly excluded from CORS. Browser-based clients cannot reach admin endpoints cross-origin even when CORS is configured for public routes.

## Remaining gaps

The following limitations remain in the current binary:

- **CORS wildcard not supported.** `SCAVIUM_FAUCET_CORS_ALLOWED_ORIGINS` does not accept `*`. Operators must supply an explicit origin list. This is by design to prevent overly permissive cross-origin access.
- **`Retry-After` header not set.** Rate-limited and budget-exceeded responses (429) include `details.retry_after_seconds` in the JSON body but do not set the standard `Retry-After` HTTP response header.
- **Single-node daily budget.** `SCAVIUM_FAUCET_DAILY_BUDGET_WEI` is enforced atomically within a single SQLite instance. Multi-replica deployments sharing one database are not a supported configuration; each instance would enforce the budget independently.

## Deployment guidance

### Reverse proxy and network placement

Run the Go binary on loopback and terminate TLS in a reverse proxy such as nginx.

Recommended external exposure:

1. allow inbound HTTPS to the proxy
2. keep the Go server off the public interface
3. keep the RPC endpoint off the public interface

### Secret handling

These values must be managed as secrets and must not appear in the repository:

- `SCAVIUM_FAUCET_PRIVATE_KEY`
- `SCAVIUM_FAUCET_ADMIN_TOKEN`
- `SCAVIUM_FAUCET_CAPTCHA_SECRET`

None of these are logged by the binary. Recommended generation:

```bash
openssl rand -hex 32
```

Use a dedicated environment file or service manager secret store, not the repository.

### Wallet hygiene

Use a dedicated faucet hot wallet with limited balance. Do not reuse treasury, validator, or deployer keys.

## Practical hardening checklist

- [ ] keep `SCAVIUM_FAUCET_BIND_ADDR` on loopback
- [ ] put TLS and external access control in a reverse proxy
- [ ] firewall off direct access to backend and RPC ports
- [ ] keep environment files readable only by the service owner
- [ ] leave `SCAVIUM_FAUCET_DRY_RUN=true` in development
- [ ] set `SCAVIUM_FAUCET_TRUSTED_PROXY` to the reverse proxy address
- [ ] set `SCAVIUM_FAUCET_CAPTCHA_PROVIDER` for public deployments
- [ ] set `SCAVIUM_FAUCET_CORS_ALLOWED_ORIGINS` to the exact frontend origin when serving browser clients
- [ ] set `SCAVIUM_FAUCET_DAILY_BUDGET_WEI` to limit total daily distribution
- [ ] rotate the admin token if it is ever exposed

## Recommended operator stance

The binary provides persistent claim storage, real readiness probes, persistent rate limiting, captcha verification, durable abuse signal capture, risk engine support, daily budget enforcement, configurable exact-origin CORS, trusted-proxy IP extraction, structured per-request logging, and an active admin API. It is suitable for a testnet faucet deployment behind a reverse proxy with TLS. With captcha enabled it is appropriate for a public faucet deployment.


## Phase 15.4 — Abuse operations and retention

Phase 15.4 keeps abuse protection operationally safe by bounding the durability window of `abuse_signals`. The SQLite store now implements explicit pruning through `domain.AbuseSignalPruner`, and application startup invokes the pruning helper with `SCAVIUM_FAUCET_ABUSE_SIGNAL_RETENTION_DAYS`.

The default retention is 30 days. Setting the value to `0` disables pruning for incident investigation or temporary forensic capture, while negative values are rejected during config validation. Pruning touches only `abuse_signals`; claims, transactions, queue state, rate-limit counters, and idempotency records are not affected.

The same store also exposes internal aggregate summaries by signal kind through `domain.AbuseSignalReporter`. These summaries are intentionally not wired to public HTTP endpoints in Phase 15.4; they prepare the data contract needed for Phase 16 observability and later admin review without exposing raw abuse metadata.

## Phase 15.close — Abuse Protection Closure

Phase 15 is closed with a layered abuse-protection posture active for the public faucet. Captcha validation acts as the public entry barrier, `abuse_signals` records claim-intake behavior, progressive enforcement can reject requests through the existing `claim_rejected` contract, and retention keeps the signal ledger bounded for long-running SQLite operation.

This closure does not introduce hard bans, new public endpoints, direct backend exposure, or additional third-party dependencies. It establishes the security baseline that Phase 16 observability will measure: captcha outcomes, risk decisions, rate-limit pressure, cooldown activity, budget exhaustion, accepted claims, and enforcement rejections.

## Phase 18 admin-control security closure

The Phase 18 admin API remains a private operator surface. All `/api/v1/admin/*` routes require the configured bearer token, are excluded from public CORS handling, and preserve the public claim API contracts.

Security-relevant closure decisions:

- Admin bearer tokens are not emitted in structured logs or audit entries.
- Admin actor attribution uses trusted-proxy-aware real IP extraction when `SCAVIUM_FAUCET_TRUSTED_PROXY` is configured.
- Mode changes are validated to `active`, `paused`, or `maintenance` before mutation and are propagated to the live faucet runtime only after validation succeeds.
- Queue, claim, and audit list endpoints enforce a `500` item cap even when larger `limit` query values are supplied.
- Blocklist `key_type` values are restricted to `ip`, `address`, or `fingerprint`; invalid values return `400 invalid_key_type`.
- Queue item responses and structured audit logs avoid wallet addresses, idempotency keys, request bodies, captcha tokens, raw fingerprints, private keys, and admin tokens.

The current in-memory admin blocklist is an operator-control surface, not a replacement for persisted abuse enforcement. Persisted abuse signals, progressive enforcement, rate limits, cooldown, and daily-budget checks remain the production claim-protection layers until a SQLite-backed admin blocklist/enforcement integration is explicitly implemented.

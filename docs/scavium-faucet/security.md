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

The binary provides persistent claim storage, real readiness probes, persistent rate limiting, captcha verification, risk engine support, daily budget enforcement, configurable exact-origin CORS, trusted-proxy IP extraction, structured per-request logging, and an active admin API. It is suitable for a testnet faucet deployment behind a reverse proxy with TLS. With captcha enabled it is appropriate for a public faucet deployment.

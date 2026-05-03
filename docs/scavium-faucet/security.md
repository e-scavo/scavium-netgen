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

### Structured logging

The binary logs structured JSON events through `internal/observability`. The current startup path logs listen and error events and does not print configuration values by default.

### Persistent rate limiting

Claim creation enforces rate limits via the SQLite-backed `RateLimiter`:

- per source IP: `SCAVIUM_FAUCET_RATE_LIMIT_IP_PER_HOUR` requests per hour
- per Ethereum address: `SCAVIUM_FAUCET_RATE_LIMIT_ADDR_PER_DAY` requests per day
- per fingerprint: same hourly limit as IP when a `fingerprint` field is supplied in the claim body

Set `SCAVIUM_FAUCET_TRUSTED_PROXY` to your reverse-proxy address so that IP extraction uses the real client IP rather than `127.0.0.1`.

### Captcha verification

When `SCAVIUM_FAUCET_CAPTCHA_PROVIDER` is set to `hcaptcha`, `recaptcha`, or `turnstile`, claim creation verifies the `captcha_token` field against the configured provider endpoint. A failed or missing token causes the claim to be rejected. Provider `dev` always passes (for testing only). Default is `disabled`.

### Admin token and persistent claim storage

`app.New` passes `cfg.AdminToken` into `httpapi.Dependencies.AdminToken`, enabling the `/api/v1/admin/*` routes when the token is non-empty. Claims are persisted in SQLite (WAL mode); restarting the process does not lose state.

### Trusted proxy and real IP extraction

`SCAVIUM_FAUCET_TRUSTED_PROXY` controls whether the handler trusts `X-Forwarded-For` / `X-Real-IP` headers. Set to the loopback or proxy address to ensure rate limiting operates on real client IPs.

## Remaining gaps

The following protections are not yet implemented:

- CORS policy
- daily budget enforcement (`SCAVIUM_FAUCET_DAILY_BUDGET_WEI` is loaded but not checked)
- service-level error differentiation for rate-limit and captcha failures (currently both return `500 claim_unavailable`; a future improvement would return `429` or `403` with precise error codes)

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
- [ ] rotate the admin token if it is ever exposed

## Recommended operator stance

The binary provides persistent claim storage, real readiness probes, persistent rate limiting, captcha support, trusted-proxy IP extraction, and an active admin API. It is suitable for a testnet faucet deployment behind a reverse proxy with TLS. Remaining gaps are CORS policy and daily budget enforcement.

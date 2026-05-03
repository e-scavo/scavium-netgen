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

## Important gaps in the current runtime

The repository already contains packages and roadmap items for richer protections, but the shipped binary does **not** enforce all of them yet.

Not wired today:

- captcha verification
- IP or address rate-limit enforcement
- trusted-proxy real IP extraction
- CORS policy
- wallet signing / on-chain payout flow
- persistent audit log or persistent claim storage
- admin API enablement through `app.New`
- deep readiness checks against real DB/RPC dependencies

Treat those as future work, not active defenses.

## Deployment guidance

### Reverse proxy and network placement

Run the Go binary on loopback and terminate TLS in a reverse proxy such as nginx.

Recommended external exposure:

1. allow inbound HTTPS to the proxy
2. keep the Go server off the public interface
3. keep the RPC endpoint off the public interface

### Secret handling

Even though the current binary does not use every secret yet, these values should still be managed as secrets:

- `SCAVIUM_FAUCET_PRIVATE_KEY`
- `SCAVIUM_FAUCET_ADMIN_TOKEN`
- `SCAVIUM_FAUCET_CAPTCHA_SECRET`

Recommended practice:

```bash
openssl rand -hex 32
```

Use a dedicated environment file or service manager secret store, not the repository.

### Wallet hygiene

When the signing path is wired later, use a dedicated faucet hot wallet with limited balance. Do not reuse treasury, validator, or deployer keys.

## Practical hardening checklist

- [ ] keep `SCAVIUM_FAUCET_BIND_ADDR` on loopback
- [ ] put TLS and external access control in a reverse proxy
- [ ] firewall off direct access to backend and RPC ports
- [ ] keep environment files readable only by the service owner
- [ ] leave `SCAVIUM_FAUCET_DRY_RUN=true` in development
- [ ] rotate the admin token if it is ever exposed
- [ ] do not assume captcha, rate limits, or budget guards are active until they are wired into the runtime

## Recommended operator stance

Because the current binary lacks durable storage and several planned abuse controls, the safest interpretation is:

- good for local development, tests, and documentation-backed MVP exploration
- not yet a hardened public internet faucet without additional outer controls and future application wiring

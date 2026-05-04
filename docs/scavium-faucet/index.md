# scavium-faucet

`scavium-faucet` is the faucet service that currently ships from this repository. The binary is a single Go HTTP server with an embedded web UI, public JSON endpoints, request IDs, structured logs, SQLite-backed persistent claim state, a background worker, and real readiness probes.

This directory documents the **implemented project surface**, not the full roadmap. The broader feature backlog remains in [`docs/scavium_faucet_public_features.md`](../scavium_faucet_public_features.md), which stays untouched and should be treated as the source roadmap document.

## Current state

`scavium-faucet` is deployed and operational on Debian 13 at `https://faucet.testnet.scavium.network` behind nginx with certbot-managed TLS and a systemd-managed backend process.

Phase 14 deployment work is COMPLETED for the testnet public faucet target.

The service is production-ready for the current testnet scope, including validated TLS auto-renewal, active firewall policy, and loopback-isolated backend exposure.

## Documentation

| Document | Description |
|---|---|
| [architecture.md](architecture.md) | Actual runtime wiring, package roles, and state model |
| [api.md](api.md) | Public API reference plus the handler-level admin contract |
| [configuration.md](configuration.md) | Environment variables, defaults, and what is wired today |
| [deployment.md](deployment.md) | Review-first VPS deployment package with systemd, nginx, env, certbot, firewall, and rollback assets |
| [deployment-certbot.md](deployment-certbot.md) | Manual ACME and certbot guide for TLS issuance and renewal |
| [deployment-firewall.md](deployment-firewall.md) | Public exposure and firewall policy for VPS and cloud edge |
| [deployment-rollback.md](deployment-rollback.md) | Rollback procedure for release symlinks and service recovery |
| [runbook.md](runbook.md) | Build, run, health checks, and operational caveats |
| [security.md](security.md) | Current security properties, gaps, and deployment guidance |

## Current implementation snapshot

- The binary loads environment config and listens on `127.0.0.1:18080` by default.
- Non-API paths serve the embedded frontend; `/api/*` paths return JSON.
- Public endpoints support health, readiness, status, config, claim creation, claim lookup, address eligibility, and version.
- Claim data and abuse-signal observations are persisted in SQLite (WAL mode). Restarting the process does not lose queued or in-flight claims or recorded claim-intake signals.
- The background worker processes the SQLite claim queue and dispatches the configured sender (dry-run or real).
- Readiness checks are real probes against the database and queue; RPC and wallet checks activate when not in dry-run mode.
- `AdminToken` is wired from config into the HTTP handler; setting `SCAVIUM_FAUCET_ADMIN_TOKEN` enables the `/api/v1/admin/*` endpoints.
- Captcha verification, durable abuse signal capture, trusted-proxy IP extraction, user-agent forwarding, and persistent rate limits (IP per hour, address per day) are active in claim creation.
- CORS is configurable via `SCAVIUM_FAUCET_CORS_ALLOWED_ORIGINS`; exact-origin matching only, wildcard `*` rejected at startup, admin paths always excluded. Empty (default) disables CORS headers entirely.
- Daily distribution is optionally capped by `SCAVIUM_FAUCET_DAILY_BUDGET_WEI`; the limit is enforced atomically in SQLite and resets at UTC midnight.
- Each request produces a structured JSON access log line on stdout containing `request_id`, `method`, `path`, `status`, `duration`, and `remote_ip`; no secrets or request bodies are logged.

## Quick start

```bash
go build ./cmd/scavium-faucet

SCAVIUM_FAUCET_DRY_RUN=true \
SCAVIUM_FAUCET_DATABASE_PATH=/tmp/scavium-faucet-dev.db \
SCAVIUM_FAUCET_RPC_URL=http://127.0.0.1:18545 \
go run ./cmd/scavium-faucet
```

See [configuration.md](configuration.md) for the environment reference and [runbook.md](runbook.md) for operational notes.

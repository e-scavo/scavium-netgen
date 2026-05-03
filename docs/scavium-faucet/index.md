# scavium-faucet

`scavium-faucet` is the faucet service that currently ships from this repository. The binary is a single Go HTTP server with an embedded web UI, public JSON endpoints, request IDs, structured logs, SQLite-backed persistent claim state, a background worker, and real readiness probes.

This directory documents the **implemented project surface**, not the full roadmap. The broader feature backlog remains in [`docs/scavium_faucet_public_features.md`](../scavium_faucet_public_features.md), which stays untouched and should be treated as the source roadmap document.

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
- Claim data is persisted in SQLite (WAL mode). Restarting the process does not lose queued or in-flight claims.
- The background worker processes the SQLite claim queue and dispatches the configured sender (dry-run or real).
- Readiness checks are real probes against the database and queue; RPC and wallet checks activate when not in dry-run mode.
- `AdminToken` is wired from config into the HTTP handler; setting `SCAVIUM_FAUCET_ADMIN_TOKEN` enables the `/api/v1/admin/*` endpoints.
- Captcha verification, trusted-proxy IP extraction, user-agent forwarding, and persistent rate limits (IP per hour, address per day) are active in claim creation.

## Quick start

```bash
go build ./cmd/scavium-faucet

SCAVIUM_FAUCET_DRY_RUN=true \
SCAVIUM_FAUCET_DATABASE_PATH=/tmp/scavium-faucet-dev.db \
SCAVIUM_FAUCET_RPC_URL=http://127.0.0.1:18545 \
go run ./cmd/scavium-faucet
```

See [configuration.md](configuration.md) for the environment reference and [runbook.md](runbook.md) for operational notes.

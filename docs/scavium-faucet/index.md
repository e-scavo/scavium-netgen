# scavium-faucet

`scavium-faucet` is the faucet MVP that currently ships from this repository. The shipped binary is a single Go HTTP server with an embedded web UI, public JSON endpoints, request IDs, structured logs, stub readiness checks, and in-memory claim state.

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
- Claim data is stored in memory only; restarting the process loses queued claims and admin state.
- The handler package includes admin routes, but the shipped app does not pass `AdminToken` into the handler, so `/api/v1/admin/*` is disabled in the binary today.

## Quick start

```bash
go build ./cmd/scavium-faucet

SCAVIUM_FAUCET_DRY_RUN=true \
SCAVIUM_FAUCET_RPC_URL=http://127.0.0.1:18545 \
go run ./cmd/scavium-faucet
```

See [configuration.md](configuration.md) for the environment reference and [runbook.md](runbook.md) for operational notes.

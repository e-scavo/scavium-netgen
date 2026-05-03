# scavium-faucet

`scavium-faucet` is the public token-distribution service for SCAVIUM EVM-compatible networks.

It provides a web UI and REST API that lets users request test funds from a wallet controlled by the operator, with integrated rate-limiting, captcha validation, per-address cooldowns, and an admin API.

## Documentation

| Document | Description |
|---|---|
| [architecture.md](architecture.md) | Component diagram, data-flow, and module overview |
| [api.md](api.md) | Public and admin REST API reference |
| [configuration.md](configuration.md) | Environment variable reference |
| [runbook.md](runbook.md) | Operational procedures, health checks, and incident response |
| [security.md](security.md) | Security model, hardening checklist, and secret management |

## Source documents

| Document | Description |
|---|---|
| [scavium_faucet_public_features.md](../scavium_faucet_public_features.md) | Canonical feature list and implementation roadmap |
| [OPERATIONS.md](../OPERATIONS.md) | Network-level operational procedures |

## Quick start

```bash
# Build
go build ./cmd/scavium-faucet

# Run in dry-run mode (no real transactions)
SCAVIUM_FAUCET_DRY_RUN=true \
SCAVIUM_FAUCET_RPC_URL=http://127.0.0.1:18545 \
./scavium-faucet
```

See [configuration.md](configuration.md) for the full list of environment variables.

## Design principles

- **Single binary** — no external runtime dependencies beyond a Besu RPC endpoint and a SQLite/Postgres DB.
- **Config from environment** — all secrets and tuning parameters are injected at runtime; nothing sensitive is committed.
- **Defense in depth** — IP rate-limits, address cooldowns, captcha, and circuit-breaker all operate independently.
- **Observable** — structured JSON logs, `/health`, `/ready`, and metrics endpoints.
- **Testable** — every package exposes interfaces so the HTTP layer, wallet signer, and DB can be exercised in unit tests without real infrastructure.

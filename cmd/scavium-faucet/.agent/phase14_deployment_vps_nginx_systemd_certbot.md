# Phase 14 — Deployment VPS / nginx / systemd / certbot

## Target environment

- OS: Debian GNU/Linux 13 (trixie)
- Public testnet domain: `faucet.testnet.scavium.network`
- Future mainnet domain: `faucet.scavium.network` (out of scope for this phase)
- Firewall: not configured yet
- Deployment mode: full hardening
- Backend: `cmd/scavium-faucet`
- Backend should listen only on localhost.
- nginx exposes HTTPS publicly.

## Objective

Prepare production deployment assets and operational steps for the SCAVIUM public faucet.

This phase focuses on:

- Linux user and directory layout
- production environment file
- systemd service hardening
- nginx reverse proxy
- Certbot TLS
- firewall hardening
- deploy script / rollback helper
- post-deploy smoke tests

## Explicit non-goals

- Do not change faucet Go business logic.
- Do not implement mainnet deployment yet.
- Do not expose backend Go port publicly.
- Do not store private keys in the repository.
- Do not hardcode secrets in scripts or docs.
- Do not enable wildcard CORS.

## Required final layout

```text
/opt/scavium-faucet/bin/scavium-faucet
/etc/scavium-faucet/scavium-faucet.env
/var/lib/scavium-faucet/scavium-faucet.db
/var/log/scavium-faucet/              # optional if journald only is not enough
```

## Runtime constraints

The backend must use:

```text
SCAVIUM_FAUCET_HOST=127.0.0.1
SCAVIUM_FAUCET_PORT=18080
SCAVIUM_FAUCET_TRUSTED_PROXY=127.0.0.1
SCAVIUM_FAUCET_PUBLIC_BASE_URL=https://faucet.testnet.scavium.network
SCAVIUM_FAUCET_DATABASE_PATH=/var/lib/scavium-faucet/scavium-faucet.db
```

## Security constraints

- SSH must remain accessible during firewall setup.
- Public ports should be limited to 22, 80, 443.
- nginx must pass real IP headers correctly.
- nginx must include basic request/body/connection limits.
- systemd must sandbox service while preserving access to `/etc/scavium-faucet`, `/var/lib/scavium-faucet`, and binary path.
- Certbot renewal must be testable.

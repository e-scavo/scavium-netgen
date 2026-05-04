# Step 14.1.0 — Deployment asset audit

## Recommended executor

Copilot Chat in VSCode.

## Goal

Read current repository deployment-related assets and produce an audit plan. Do not modify files.

## Read

- `cmd/scavium-faucet/`
- `cmd/scavium-faucet/.agent/phase14_deployment_vps_nginx_systemd_certbot.md`
- `docs/scavium-faucet/`
- any existing deployment templates under `docs/scavium-faucet/deployment/`
- `Makefile`
- `go.mod`

## Confirm

1. Existing systemd unit template, if any.
2. Existing nginx template, if any.
3. Existing env example, if any.
4. Existing deploy/rollback scripts, if any.
5. Whether templates already reflect:
   - Debian 13
   - `faucet.testnet.scavium.network`
   - backend `127.0.0.1:18080`
   - `SCAVIUM_FAUCET_TRUSTED_PROXY=127.0.0.1`
   - `/var/lib/scavium-faucet/scavium-faucet.db`
   - CORS allowed origins
   - daily budget
   - captcha provider placeholders
   - systemd hardening
   - nginx real IP headers
   - Certbot
   - firewall

## Output

- files read
- assets already present
- assets missing or stale
- exact files to create/update in 14.1.1
- risks before touching VPS

## Hard constraints

- Do not modify files.
- Do not read or use `cmd/scavium-faucet-v0`.

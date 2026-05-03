# Step 12.1.3 — Align runbook, operations and deployment documentation

## Recommended executor

Copilot Chat in VSCode.

## Goal

Update runbook/deployment/operations docs to match the now-wired runtime while keeping VPS/nginx/certbot as deployment-stage work.

## Files to consider

- `docs/scavium-faucet/runbook.md`
- `docs/scavium-faucet/deployment.md`
- `docs/scavium-faucet/operations.md`
- `docs/scavium-faucet/testing.md`
- Any related file under `docs/scavium-faucet/`

## Required corrections

Document accurately:

- How to run locally in dry-run with SQLite.
- Expected `/health` and `/ready` behavior.
- DB path and persistence expectations.
- WAL/busy timeout behavior.
- Queue/worker operational behavior.
- Watcher only in non-dry when configured.
- What logs/metrics are available now vs planned.
- What remains deployment-stage: nginx, certbot, systemd, VPS hardening, production TLS.

## Suggested local smoke command

If the docs contain a smoke section, align it with the actual command style:

```bash
SCAVIUM_FAUCET_DRY_RUN=true \
SCAVIUM_FAUCET_DATABASE_PATH=/tmp/scavium-faucet-smoke.db \
go run ./cmd/scavium-faucet
```

Then:

```bash
curl -s http://127.0.0.1:18080/health
curl -s http://127.0.0.1:18080/ready
curl -s -X POST http://127.0.0.1:18080/api/v1/claim \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: smoke-1' \
  -d '{"address":"0x0000000000000000000000000000000000000001"}'
```

## Hard constraints

- Do not modify Go code.
- Do not claim production deployment has been completed.
- Do not read or use `cmd/scavium-faucet-v0`.

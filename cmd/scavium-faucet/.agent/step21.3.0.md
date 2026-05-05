# Step 21.3.0 — Alerting Runbook and Smoke Tests

## Goal

Document and script safe operator smoke tests and alert thresholds.

## Scope

- Add runbook sections for low balance, RPC unavailable, stuck queue, failed tx spike, captcha spike, blocklist spike, and high rejection rate.
- Add scripts only if they are local, safe, no-secret, and useful.
- Do not add external services.
- Update deployment docs if nginx/journald correlation is involved.

## Validation

```bash
bash -n scripts/*.sh
go test ./... -timeout 300s
make build -B
```

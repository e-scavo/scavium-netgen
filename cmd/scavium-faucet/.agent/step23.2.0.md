# Step 23.2.0 — Wallet Refill and Rotation Runbooks

## Goal

Document safe manual wallet refill and wallet rotation procedures.

## Scope

- Update runbook/security/configuration docs.
- Add scripts only if they do not touch real funds without explicit operator action.
- Include rollback and verification steps.
- Do not implement automatic treasury refill.

## Validation

```bash
bash -n scripts/*.sh
go test ./... -timeout 300s
make build -B
```

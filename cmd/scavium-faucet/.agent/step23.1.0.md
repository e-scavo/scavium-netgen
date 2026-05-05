# Step 23.1.0 — Backup Restore Scripts and Verification

## Goal

Add safe SQLite/config backup and restore guidance with optional scripts.

## Scope

- Add or update scripts under `scripts/` only if safe and generic.
- Add dry-run/check modes where possible.
- Do not embed paths that conflict with deployment docs; use env vars/defaults.
- Update runbook/deployment docs.

## Validation

```bash
bash -n scripts/*.sh
go test ./... -timeout 300s
make build -B
```

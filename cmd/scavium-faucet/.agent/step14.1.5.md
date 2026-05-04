# Step 14.1.5 — Deployment documentation alignment

## Recommended executor

Codex or Copilot if only small documentation edits are needed.

## Goal

After real VPS deployment, update documentation with the actual tested deployment flow.

## Files to consider

- `docs/scavium-faucet/deployment.md`
- `docs/scavium-faucet/runbook.md`
- `docs/scavium-faucet/deployment/README.md`

## Include

- Debian 13 confirmation
- domain `faucet.testnet.scavium.network`
- actual service paths
- actual smoke commands
- known post-deploy status
- any corrected deployment caveats

## Hard constraints

- Documentation-only.
- Do not store secrets.
- Do not modify Go code.
- Do not read or use `cmd/scavium-faucet-v0`.

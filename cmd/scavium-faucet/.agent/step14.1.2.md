# Step 14.1.2 — Review generated deployment assets

## Recommended executor

Copilot Chat in VSCode.

## Goal

Review Phase 14 deployment assets before they are used on the VPS. Do not modify files.

## Verify

- Debian 13 package commands are reasonable.
- nginx listens for `faucet.testnet.scavium.network`.
- backend proxies to `127.0.0.1:18080`.
- nginx passes real IP headers.
- env example sets `SCAVIUM_FAUCET_TRUSTED_PROXY=127.0.0.1`.
- systemd hardening does not block:
  - reading `/etc/scavium-faucet/scavium-faucet.env`
  - writing `/var/lib/scavium-faucet/scavium-faucet.db`
  - outbound RPC connections
  - localhost HTTP bind
- install script does not overwrite secrets.
- firewall commands do not lock out SSH.
- certbot flow is correct.
- smoke test is safe.

## Output

- pass/fail summary
- risks found
- whether a fix step is needed before VPS execution

## Hard constraints

- Do not modify files.
- Do not read or use `cmd/scavium-faucet-v0`.

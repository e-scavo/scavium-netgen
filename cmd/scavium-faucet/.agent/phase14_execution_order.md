# Phase 14 — Recommended Execution Order

## Low-risk order

```text
14.1.0 → Copilot Chat → read existing deployment assets and audit
14.1.1 → Codex → generate/update deployment templates/scripts only
14.1.2 → Copilot Chat → review generated assets for Debian 13 + domain
14.1.3 → Manual VPS execution → install packages, create user/dirs, install binary/env/systemd/nginx/certbot/firewall
14.1.4 → Manual smoke validation → health/ready/claim/logs/renewal
14.1.5 → Codex or Copilot → documentation alignment only, if needed
```

## Codex wrapper

```text
Execute cmd/scavium-faucet/.agent/step14.1.X.md following cmd/scavium-faucet/.agent/rules.md and cmd/scavium-faucet/.agent/commands.md.

Use the current repository as the only source of truth.
Target VPS is Debian 13 trixie.
Target domain is faucet.testnet.scavium.network.
Do not change Go business logic.
Do not hardcode secrets.
Do not expose the backend Go port publicly.
Do not read, copy, or derive implementation from cmd/scavium-faucet-v0.
Before editing, report files read, files to modify, and plan.
After editing, report files modified, validation commands run, results, and git commands executed.
```

## Copilot wrapper

```text
Execute cmd/scavium-faucet/.agent/step14.1.X.md.

Use the current repository as the only source of truth.
Target VPS is Debian 13 trixie.
Target domain is faucet.testnet.scavium.network.
Do not modify files unless the step explicitly says so.
Do not read or use cmd/scavium-faucet-v0.
Return files read, findings, risks, and next actions.
```

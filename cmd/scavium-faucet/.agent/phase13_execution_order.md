# Phase 13 — Recommended Execution Order

## Low-cost flow

```text
13.1.0 → Copilot Chat → audit only
13.1.1 → Codex → precise claim error mapping
13.1.2 → Codex → safe CORS
13.1.3 → Codex → daily budget enforcement
13.1.4 → Codex → minimal observability/logging
13.1.5 → Copilot Chat → final audit
13.1.6 → Codex only if needed
```

## Standard Codex wrapper

```text
Execute cmd/scavium-faucet/.agent/step13.1.X.md following cmd/scavium-faucet/.agent/rules.md and cmd/scavium-faucet/.agent/commands.md.

Use the current repository as the only source of truth.
Do not modify documentation.
Do not touch deployment/VPS/nginx/systemd/certbot files.
Do not read, copy, or derive implementation from cmd/scavium-faucet-v0.
Before editing, report files read, files to modify, and plan.
After editing, report files modified, tests run, results, and git commands executed.
```

## Standard Copilot wrapper

```text
Execute cmd/scavium-faucet/.agent/step13.1.X.md.

Use the current repository as the only source of truth.
Do not modify files unless the step explicitly allows a tiny comment typo fix.
Do not read or use cmd/scavium-faucet-v0.
Return files read, findings, commands run, and whether Codex is needed.
```

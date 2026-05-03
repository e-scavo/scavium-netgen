# Phase 12 — Recommended Execution Order

## Preferred low-cost flow

Use Copilot Chat for steps 12.1.0 to 12.1.4.

```text
12.1.0 → Copilot Chat → audit plan only, no edits
12.1.1 → Copilot Chat → architecture/index docs
12.1.2 → Copilot Chat → configuration/API/security docs
12.1.3 → Copilot Chat → runbook/ops/deployment/testing docs
12.1.4 → Copilot Chat → final documentation consistency pass
```

## Optional Codex step

Use only if Copilot leaves inconsistencies or the final diff is too broad:

```text
12.1.5 → Codex → strict final documentation review/fix
```

## Standard Copilot instruction wrapper

Use this before each step:

```text
Execute cmd/scavium-faucet/.agent/step12.1.X.md.
Use the current repository as the only source of truth.
Do not modify Go code.
Do not modify deployment scripts unless they are Markdown documentation.
Do not read or use cmd/scavium-faucet-v0.
Before editing, report files read, files to modify, and plan.
After editing, report files modified and validation commands run.
```

## Standard Codex instruction wrapper, if step12.1.5 is used

```text
Execute cmd/scavium-faucet/.agent/step12.1.5.md following cmd/scavium-faucet/.agent/rules.md and cmd/scavium-faucet/.agent/commands.md.

Use the current repository as the only source of truth.
Documentation-only.
Do not modify Go code.
Do not read, copy, or derive implementation from cmd/scavium-faucet-v0.
Before editing, report files read, files to modify, and plan.
After editing, report files modified, validation commands run, results, and git commands executed.
```

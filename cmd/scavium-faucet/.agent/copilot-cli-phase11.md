# Copilot CLI Usage Guide — Phase 11

Use this file as a lightweight prompt source for Copilot CLI to reduce Codex usage.

## Good Copilot CLI tasks

Use Copilot CLI for:

- grep/search audits.
- listing references.
- summarizing existing interfaces.
- finding compile/test failures.
- small mechanical edits after Codex has defined the design.
- final regression checks.

## Tasks that should remain Codex/VSCode

Use Codex in VSCode for:

- adding persistent service architecture.
- modifying runtime composition.
- changing app lifecycle.
- wiring worker/watcher/sender.
- adding integration tests.
- fixing multi-file compile failures.

## Copilot CLI prompt for Step 11.1.0

```text
Inspect the current Go repository only. Do not edit files.
Focus on cmd/scavium-faucet.
Confirm whether app.New still uses faucet.NewInMemoryReadService.
List all runtime wiring gaps for SQLite store, queue, worker, sender, watcher, readiness, admin token, captcha/risk and rate limits.
Return exact file paths and symbol names.
Do not read or use cmd/scavium-faucet-v0.
```

## Copilot CLI prompt for Step 11.1.6

```text
Inspect the current Go repository only unless a test failure requires a minimal fix.
Focus on cmd/scavium-faucet.
Verify that the production runtime no longer uses in-memory faucet service by default.
Verify SQLite, migrations, persistent claim creation, queue, worker, readiness and admin token are wired.
Run or suggest go test ./... from repo root.
Do not modify documentation.
Do not read or use cmd/scavium-faucet-v0.
```

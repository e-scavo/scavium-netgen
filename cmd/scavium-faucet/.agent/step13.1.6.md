# Step 13.1.6 — Optional final fix after hardening audit

## Recommended executor

Codex in VSCode only if step13.1.5 finds a small concrete gap.

## Goal

Apply a minimal final fix discovered by step13.1.5.

## Instructions

Before editing, state:

- The exact failing check from step13.1.5.
- The minimal files to modify.
- The test that will prove the fix.

Then implement only that fix.

## Validation

Run:

```bash
gofmt -w <go files changed>
go test ./cmd/scavium-faucet/...
go test ./...
go build ./cmd/scavium-faucet
```

## Hard constraints

- Do not broaden scope.
- Do not modify docs.
- Do not touch deployment files.
- Do not read or use `cmd/scavium-faucet-v0`.

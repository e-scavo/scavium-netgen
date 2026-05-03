# Step 12.1.5 — Optional Codex packaging review

## Recommended executor

Codex in VSCode only if needed.

## Goal

Use this only if Copilot's documentation edits need a stricter final review before packaging.

This step should not perform broad rewrites. It is a final audit/fix step.

## Instructions

Read:

- `docs/scavium-faucet/*.md`
- `docs/scavium_faucet_public_features.md`
- `cmd/scavium-faucet/internal/app/app.go`
- `cmd/scavium-faucet/internal/config/config.go`
- `cmd/scavium-faucet/internal/store/sqlite/store.go`
- `cmd/scavium-faucet/internal/httpapi/handler.go`
- `cmd/scavium-faucet/internal/ready/ready.go`

Fix only documentation inconsistencies that directly contradict current code.

## Required validation

Run:

```bash
git diff --name-only
grep -Rni "in-memory\|stub readiness\|not wired\|unwired\|SQLite.*pending\|persistence.*pending" docs/scavium-faucet docs/scavium_faucet_public_features.md || true
go test ./...
```

## Hard constraints

- Documentation-only.
- Do not modify Go code.
- Do not read or use `cmd/scavium-faucet-v0`.
- Do not perform style-only rewrites.

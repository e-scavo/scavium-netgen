# SCAVIUM Faucet Copilot Step Runner

This runner is an operator-controlled helper for executing the `.agent/step*.md` prompts in sequence.

It intentionally does **not** call Copilot automatically. Instead, it prepares the exact prompt, writes it to `current-prompt.md`, optionally copies it to the clipboard when a clipboard tool is available, pauses for the operator to run the prompt in Copilot Chat, and then runs validation before advancing.

## Why it is interactive

SCAVIUM Faucet is production software. Each phase must remain:

- minimal
- auditable
- backward-compatible
- documented
- committed before the next phase

A fully unattended runner would encourage broad, unreviewed changes. This script keeps automation useful without removing operator checkpoints.

## Basic usage

From the repository root:

```bash
cmd/scavium-faucet/.agent/run-copilot-steps.sh --from step20.1.0.md --to step20.1.0.md
```

Dry-run the selected steps:

```bash
cmd/scavium-faucet/.agent/run-copilot-steps.sh --from step20.1.0.md --to step20.5.0.md --dry-run
```

Run a broader sequence:

```bash
cmd/scavium-faucet/.agent/run-copilot-steps.sh --from step20.1.0.md --to step24.1.0.md
```

Use explicit validation commands:

```bash
cmd/scavium-faucet/.agent/run-copilot-steps.sh \
  --from step20.1.0.md \
  --to step20.1.0.md \
  --validate 'go test ./... -timeout 300s' \
  --validate 'make build -B'
```

## Operating rules

Before each step, the script requires a clean git working tree.

After Copilot finishes a step:

1. review the diff
2. run/confirm validation
3. commit and merge using the git block produced by Copilot
4. continue to the next step only after the repository is clean again

## Generated files

- `current-prompt.md`: last prepared prompt for Copilot Chat
- `run-logs/*.log`: local run metadata for each operator-confirmed step

These files are local operator artifacts. Commit them only if you intentionally want to preserve run evidence in the repository.

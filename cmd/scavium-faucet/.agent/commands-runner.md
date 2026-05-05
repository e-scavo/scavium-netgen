# Copilot Runner Commands Addendum

Use this addendum together with `commands.md`.

## Run one step

```bash
cmd/scavium-faucet/.agent/run-copilot-steps.sh --from step20.1.0.md --to step20.1.0.md
```

## Dry-run selected sequence

```bash
cmd/scavium-faucet/.agent/run-copilot-steps.sh --from step20.1.0.md --to step20.5.0.md --dry-run
```

## Run with production validation

```bash
cmd/scavium-faucet/.agent/run-copilot-steps.sh \
  --from step20.1.0.md \
  --to step20.1.0.md \
  --validate 'go test ./... -timeout 300s' \
  --validate 'make build -B'
```

## Safety checkpoint

The runner must stop on a dirty working tree. Commit, stash, or revert before continuing.

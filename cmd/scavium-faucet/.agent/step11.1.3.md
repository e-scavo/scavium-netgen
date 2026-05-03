# Step 11.1.3 — Runtime Composition: SQLite Store, Worker, Sender, Watcher

Recommended executor: Codex in VSCode.

## Goal

Wire the runtime so `cmd/scavium-faucet/main.go` starts a persistent faucet, not an in-memory mock.

## Required implementation

1. Update `cmd/scavium-faucet/internal/app/app.go` so `New(cfg)`:

   - opens SQLite using `cfg.DatabasePath`.
   - applies migrations through `sqlite.Open`.
   - creates the persistent faucet read service from Step 11.1.2.
   - passes that service to `httpapi.NewHandler`.
   - passes `cfg.AdminToken` to `httpapi.NewHandler`.
   - creates worker dependencies.

2. Sender wiring:

   - when `cfg.DryRun == true`, use `chain.NewDryRunSender(...)`.
   - when `cfg.DryRun == false`:
     - create RPC client using `chain.NewClient`.
     - validate chain ID with `chain.ValidateChainID`.
     - create signer from `cfg.PrivateKeyHex`.
     - create `chain.NewEthSender`.
   - Never log private key.

3. Worker wiring:

   - create `worker.New(store, sender, worker.Config{...}, logger)`.
   - start it in a goroutine with an app-owned context.
   - make app shutdown cancel the context and close DB/RPC.

4. Watcher wiring:

   - if there is a real chain client and watcher enabled, create and start `chain.NewWatcher`.
   - skip watcher in dry-run unless implementation intentionally supports dry-run confirmation.
   - do not let watcher startup break dry-run local development.

5. `main.go`:

   - ensure `app.Close` runs on server shutdown where possible.
   - preserve existing server behavior.

6. Add tests:

   - `app.New(config.Defaults())` uses persistent store when DB path points to temp file.
   - public claim survives app close/reopen.
   - admin token is passed through enough to avoid disabled admin when token set.
   - dry-run starts without RPC/private key.

## Important boundaries

- Do not modify documentation.
- Do not implement VPS/nginx/systemd here.
- Do not use `cmd/scavium-faucet-v0`.

## Commands

```bash
git checkout -b faucet/step11.1.3-runtime-composition
git status --short
go test ./cmd/scavium-faucet/internal/app ./cmd/scavium-faucet/internal/httpapi ./cmd/scavium-faucet/internal/worker ./cmd/scavium-faucet/internal/chain
go test ./...
```

## Git finalization

```bash
git status --short
git add cmd/scavium-faucet/internal/app cmd/scavium-faucet/main.go cmd/scavium-faucet/internal/chain cmd/scavium-faucet/internal/worker cmd/scavium-faucet/internal/httpapi
git commit -m "Wire faucet runtime to persistent store and workers"
```

## Expected output

- Files read.
- Files modified.
- Tests run and results.
- Commit hash.

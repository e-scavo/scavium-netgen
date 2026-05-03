# Runbook

This runbook covers the faucet binary that exists today: one Go process, embedded frontend, public JSON API, SQLite-backed persistent claim state, a background worker, and real readiness probes.

## Build and run

```bash
go build ./cmd/scavium-faucet

SCAVIUM_FAUCET_BIND_ADDR=127.0.0.1:18080 \
SCAVIUM_FAUCET_RPC_URL=http://127.0.0.1:18545 \
SCAVIUM_FAUCET_DATABASE_PATH=/tmp/scavium-faucet-dev.db \
SCAVIUM_FAUCET_DRY_RUN=true \
./scavium-faucet
```

The binary logs structured JSON to stdout. The database file and its parent directory are created automatically if they do not exist. Migrations run on every startup.

## Health checks

```bash
curl -s http://127.0.0.1:18080/health
curl -s http://127.0.0.1:18080/ready
curl -s http://127.0.0.1:18080/api/v1/status
curl -s http://127.0.0.1:18080/api/v1/config
curl -s http://127.0.0.1:18080/api/v1/version
```

What to expect:

- `/health` returns `{"status":"ok",...}`
- `/ready` returns `status: "ok"` with real DB and queue probes; in non-dry-run mode also includes RPC and wallet probes
- `/api/v1/status` returns the configured network name, symbol, and `dry_run` flag

## Manual API smoke test

Start the service in a separate terminal:

```bash
SCAVIUM_FAUCET_DRY_RUN=true \
SCAVIUM_FAUCET_DATABASE_PATH=/tmp/scavium-faucet-smoke.db \
go run ./cmd/scavium-faucet
```

Then in another terminal:

```bash
curl -s http://127.0.0.1:18080/health
curl -s http://127.0.0.1:18080/ready
curl -s -X POST http://127.0.0.1:18080/api/v1/claim \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: smoke-1' \
  -d '{"address":"0x0000000000000000000000000000000000000001"}'
```

The claim response will include an `id`. Fetch the persisted claim:

```bash
curl -s http://127.0.0.1:18080/api/v1/claim/<claim-id>
```

In dry-run mode the background worker picks up the queued claim and advances its status to `sent` within a few seconds.

## Operating assumptions

- Claim state is persisted in SQLite (WAL mode, 5 s busy timeout). Restarting the process does not lose queued or in-flight claims.
- The background worker processes queued claims automatically (enabled by default, polls every 5 s).
- `/ready` runs real probes: DB and queue always; RPC and wallet when `DRY_RUN=false`.
- The admin API (`/api/v1/admin/*`) is active when `SCAVIUM_FAUCET_ADMIN_TOKEN` is set.
- In dry-run mode no on-chain transactions are submitted; the `DryRunSender` simulates success.
- The watcher (on-chain confirmation poller) is only active when `DRY_RUN=false`.

## Service manager example

If you wrap the binary with `systemd`, the operational loop is standard:

```bash
systemctl status scavium-faucet
journalctl -u scavium-faucet -n 200 --no-pager
systemctl restart scavium-faucet
```

## Deployment notes

1. Bind the service to loopback.
2. Put nginx or another reverse proxy in front of it for TLS.
3. Keep configuration outside the repository.
4. Set `SCAVIUM_FAUCET_DATABASE_PATH` to a durable path outside the release directory so the database survives deployments.
5. Set `SCAVIUM_FAUCET_DRY_RUN=false` and provide `SCAVIUM_FAUCET_PRIVATE_KEY` when ready for live transactions.

## Troubleshooting

### Process exits immediately

Most startup failures come from config validation. Check stdout/stderr for the structured `load config failed` log entry and verify required environment variables such as bind address, public base URL, RPC URL, chain ID, network name, symbol, and amount.

### Claim returns `400 invalid_address`

The API validates checksum-compatible EVM addresses. Re-submit with a proper `0x...` hex address.

### Claims disappear after restart

This should not happen with the current binary. Claims are persisted in SQLite. If claims are missing after restart, verify that `SCAVIUM_FAUCET_DATABASE_PATH` points to the same durable file across restarts and that the file was not deleted.

### `/api/v1/admin/*` returns `503`

`503` means `SCAVIUM_FAUCET_ADMIN_TOKEN` is empty or not set. Set the token in the environment file and restart the service.

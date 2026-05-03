# Runbook

This runbook covers the faucet binary that exists today: one Go process, embedded frontend, public JSON API, and in-memory claim state.

## Build and run

```bash
go build ./cmd/scavium-faucet

SCAVIUM_FAUCET_BIND_ADDR=127.0.0.1:18080 \
SCAVIUM_FAUCET_RPC_URL=http://127.0.0.1:18545 \
SCAVIUM_FAUCET_DRY_RUN=true \
./scavium-faucet
```

The binary logs structured JSON to stdout.

## Health checks

```bash
curl -s http://127.0.0.1:18080/health
curl -s http://127.0.0.1:18080/ready
curl -s http://127.0.0.1:18080/api/v1/status
curl -s http://127.0.0.1:18080/api/v1/config
curl -s http://127.0.0.1:18080/api/v1/version
```

What to expect:

- `/health` should return `{"status":"ok",...}`
- `/ready` should usually return `status: "ok"` with stub checks
- `/api/v1/status` should return the configured network name, symbol, and `dry_run`

## Manual API smoke test

Create a claim:

```bash
curl -s -X POST http://127.0.0.1:18080/api/v1/claim \
  -H 'Content-Type: application/json' \
  -d '{"address":"0x52908400098527886E0F7030069857D2E4169EE7"}'
```

Then fetch it:

```bash
curl -s http://127.0.0.1:18080/api/v1/claim/<claim-id>
```

## Operating assumptions

- Claim state is in memory only.
- Restarting the process clears claims and any admin-service state.
- `/ready` is not a deep infrastructure probe yet.
- The shipped binary does not currently enable the admin API because `AdminToken` is not passed through `app.New`.

Because of that, do not treat the current binary as a durable production service yet.

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
4. Set `SCAVIUM_FAUCET_DRY_RUN=false` only when you are ready to fund and use a real faucet wallet in a later wired-up deployment path.

## Troubleshooting

### Process exits immediately

Most startup failures come from config validation. Check stdout/stderr for the structured `load config failed` log entry and verify required environment variables such as bind address, public base URL, RPC URL, chain ID, network name, symbol, and amount.

### Claim returns `400 invalid_address`

The API validates checksum-compatible EVM addresses. Re-submit with a proper `0x...` hex address.

### Claims disappear after restart

That is expected today because the shipped service uses in-memory claim storage only.

### `/api/v1/admin/*` returns `503`

That is expected in the shipped app. The handler supports admin routes, but `app.New` does not wire `SCAVIUM_FAUCET_ADMIN_TOKEN` into the runtime handler yet.

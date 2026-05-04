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

## CORS, daily budget, and logging

### CORS

CORS is disabled by default. No `Access-Control-Allow-Origin` headers are emitted unless `SCAVIUM_FAUCET_CORS_ALLOWED_ORIGINS` is set. To allow a browser frontend at a specific origin:

```bash
export SCAVIUM_FAUCET_CORS_ALLOWED_ORIGINS=https://faucet.example.com
```

Multiple origins are comma-separated. Wildcard `*` is rejected at startup. Admin paths (`/api/v1/admin/*`) are never covered by CORS regardless of this setting.

### Daily budget

`SCAVIUM_FAUCET_DAILY_BUDGET_WEI` caps the total amount distributed within a UTC calendar day. When the cap is reached, new claim requests return `429 daily_budget_exceeded` until midnight UTC. The budget is enforced atomically in SQLite; it resets automatically with the next day's window. Leave the variable unset for unlimited distribution.

Example — limit to 100 ether per day on a network with 18-decimal tokens:

```bash
export SCAVIUM_FAUCET_DAILY_BUDGET_WEI=100000000000000000000
```

### Request logging

The binary writes one JSON log line per request to stdout. Each line contains:

```json
{"time":"2026-05-04T12:00:00Z","level":"info","message":"http request","request_id":"abc123","method":"POST","path":"/api/v1/claim","status":202,"duration":"3ms","remote_ip":"1.2.3.4"}
```

Request bodies, captcha tokens, fingerprints, and secret configuration values are never logged. Collect stdout with `journalctl` or forward to a log aggregator.

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

### Claim returns `429 rate_limited`

The source IP, Ethereum address, or browser fingerprint has exceeded its configured rate limit or cooldown window. Check `SCAVIUM_FAUCET_RATE_LIMIT_IP_PER_HOUR`, `SCAVIUM_FAUCET_RATE_LIMIT_ADDR_PER_DAY`, and `SCAVIUM_FAUCET_COOLDOWN_SECONDS`. The response body `details.retry_after_seconds` indicates the remaining wait time. If the rate is lower than expected, verify `SCAVIUM_FAUCET_TRUSTED_PROXY` is set so IP extraction uses the real client address.

### Claim returns `429 daily_budget_exceeded`

`SCAVIUM_FAUCET_DAILY_BUDGET_WEI` has been reached for the current UTC day. No new claims will be accepted until midnight UTC. To raise the cap, increase `SCAVIUM_FAUCET_DAILY_BUDGET_WEI` and restart the service. The response body `details.reason` shows the used, requested, and maximum amounts.

### Claim returns `422 captcha_failed`

Captcha verification failed. Check that `SCAVIUM_FAUCET_CAPTCHA_PROVIDER`, `SCAVIUM_FAUCET_CAPTCHA_SECRET`, and `SCAVIUM_FAUCET_CAPTCHA_VERIFY_URL` are correct and that the captcha provider is reachable from the server. The response `details.reason` contains the provider's error codes. Use `SCAVIUM_FAUCET_CAPTCHA_PROVIDER=dev` with token `dev-bypass` for local testing.

### Claim returns `403 claim_rejected`

The request was blocked by the risk engine or an operator blocklist. The response `details.reason` contains the rejection reason. If this is unexpected for a legitimate address, review any blocklist entries via the admin API (`GET /api/v1/admin/blocklist`) and remove incorrect entries with `DELETE /api/v1/admin/blocklist`.

### Claims disappear after restart

This should not happen with the current binary. Claims are persisted in SQLite. If claims are missing after restart, verify that `SCAVIUM_FAUCET_DATABASE_PATH` points to the same durable file across restarts and that the file was not deleted.

### `/api/v1/admin/*` returns `503`

`503` means `SCAVIUM_FAUCET_ADMIN_TOKEN` is empty or not set. Set the token in the environment file and restart the service.

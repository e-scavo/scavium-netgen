# Runbook

This runbook covers the faucet binary that exists today: one Go process, embedded frontend, public JSON API, SQLite-backed persistent claim state, a background worker, and real readiness probes.

## Production status (2026-05-04)

The faucet is live at `https://faucet.testnet.scavium.network`.

Validated production topology:

- Debian 13 (trixie) VPS
- nginx reverse proxy on public HTTPS
- systemd-managed backend service
- certbot-managed TLS certificates
- backend loopback bind (`127.0.0.1:18080`)
- SQLite persistence and background worker active

Validated production outcomes:

- `/health` and `/ready` successful
- `/health` exposes uptime and build metadata
- `/ready` exposes real probe status, per-check duration, and readiness summary
- claim flow successful through `queued` -> `sending` -> `confirmed`
- rate limiting observed (`429`)
- CORS configuration active and verified
- request and correlation logging active in journald
- admin-protected runtime metrics available when `SCAVIUM_FAUCET_ADMIN_TOKEN` is set
- RPC connectivity and transaction sending verified

Operational confirmations (Phase 14 final validation):

- TLS renewal validated (`certbot renew --dry-run` successful)
- `certbot.timer` active
- deploy hook present at `/etc/letsencrypt/renewal-hooks/deploy/reload-nginx.sh`
- host firewall active (UFW default deny incoming; allow `22/tcp`, `80/tcp`, `443/tcp`)

Manual renewal verification command:

```bash
sudo certbot renew --dry-run
```

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

- `/health` returns `status: "ok"` plus `uptime_seconds` and build metadata
- `/ready` returns `status: "ok"` with real DB and queue probes; in non-dry-run mode also includes RPC and wallet probes; each check includes `duration_ms` and the response includes a `summary` object
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
- `/api/v1/admin/metrics` reports process-local runtime counters and resets on process restart.
- In dry-run mode no on-chain transactions are submitted; the `DryRunSender` simulates success.
- The watcher (on-chain confirmation poller) is only active when `DRY_RUN=false`.

## CORS, daily budget, metrics, and logging

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


### Runtime metrics

`GET /api/v1/admin/metrics` is available when `SCAVIUM_FAUCET_ADMIN_TOKEN` is configured. It uses the same bearer-token contract as the rest of the admin API:

```bash
curl -s http://127.0.0.1:18080/api/v1/admin/metrics \
  -H "Authorization: Bearer $SCAVIUM_FAUCET_ADMIN_TOKEN"
```

The response includes process-local counters for accepted claims, rejected claims, captcha failures, rate-limit hits, daily-budget exceedances, faucet-unavailable responses, claim-unavailable responses, and risk rejections. It also includes build metadata and uptime. These counters are operational diagnostics only and reset when the process restarts; durable claim history remains in SQLite.

### Request logging

The binary writes one JSON log line per request to stdout. Each line contains:

```json
{"time":"2026-05-04T12:00:00Z","level":"info","message":"http request","request_id":"abc123","correlation_id":"abc123","method":"POST","path":"/api/v1/claim","status":202,"duration":"3ms","remote_ip":"1.2.3.4"}
```

Request bodies, captcha tokens, raw fingerprints, wallet addresses, idempotency-key values, and secret configuration values are never logged. Claim-flow events log only safe booleans such as `has_fingerprint`, `captcha_token_present`, and `has_idempotency_key`, plus rejection code/retry metadata when present. Collect stdout with `journalctl` or forward to a log aggregator.

## Service manager example

If you wrap the binary with `systemd`, the operational loop is standard:

```bash
systemctl status scavium-faucet
journalctl -u scavium-faucet -n 200 --no-pager
systemctl restart scavium-faucet
```

## Production operation commands (executed)

The following command set was used during successful production rollout and validation:

- `systemctl daemon-reload`
- `systemctl enable scavium-faucet.service`
- `systemctl start scavium-faucet.service`
- `systemctl status scavium-faucet.service --no-pager`
- `nginx -t`
- `systemctl reload nginx`
- `certbot --nginx -d faucet.testnet.scavium.network --agree-tos --email OPS_EMAIL --redirect --dry-run`
- `certbot --nginx -d faucet.testnet.scavium.network --agree-tos --email OPS_EMAIL --redirect`
- `journalctl -u scavium-faucet.service -f`
- `curl -fsS http://127.0.0.1:18080/health`
- `curl -fsS http://127.0.0.1:18080/ready`
- `curl -fsS http://127.0.0.1:18080/api/v1/admin/metrics -H "Authorization: Bearer $SCAVIUM_FAUCET_ADMIN_TOKEN"`
- `curl -fsS https://faucet.testnet.scavium.network/health`


## Token registration operations (Phase 17.2 closed)

Token registration is operated through environment configuration and service restart. This is the stable Phase 17.2 model: the faucet does not mutate token definitions at runtime and does not store the token catalog in SQLite.

Operator sequence:

1. Add or update the desired native/ERC20 entries in `SCAVIUM_FAUCET_TOKENS_JSON`.
2. Keep `SCAVIUM_FAUCET_DEFAULT_TOKEN_ID` pointed at a configured token so legacy claim clients continue to work.
3. Restart the systemd service after changing token configuration.
4. Validate the public catalog with `GET /api/v1/tokens` or `GET /api/v1/faucet/tokens`.
5. Submit a bounded test claim with explicit `token_id` for each ERC20 token before announcing it to users.

Do not publish private keys, captcha secrets, admin bearer tokens, or raw environment files while documenting registered tokens. Publicly safe token fields are the fields returned by the token catalog endpoints.

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

The request was blocked by the risk engine. In the current Phase 15 path this usually means progressive abuse enforcement found too many recent negative signals for the source IP, wallet address, or browser fingerprint. The response `details.reason` contains the rejection reason. Review `SCAVIUM_FAUCET_ABUSE_ENFORCEMENT_*` thresholds and the recent `abuse_signals` ledger before loosening controls.

### Claims disappear after restart

This should not happen with the current binary. Claims are persisted in SQLite. If claims are missing after restart, verify that `SCAVIUM_FAUCET_DATABASE_PATH` points to the same durable file across restarts and that the file was not deleted.

### `/api/v1/admin/*` returns `503`

`503` means `SCAVIUM_FAUCET_ADMIN_TOKEN` is empty or not set. Set the token in the environment file and restart the service.

### `/api/v1/admin/metrics` returns `401`

`401` means the metrics endpoint is active but the request is missing `Authorization: Bearer <token>` or the supplied token does not match `SCAVIUM_FAUCET_ADMIN_TOKEN`. Use the same token contract as the rest of the admin API.


## Abuse signal retention

`SCAVIUM_FAUCET_ABUSE_SIGNAL_RETENTION_DAYS` controls how long abuse observations are kept in SQLite. The default is 30 days. Expired rows are pruned during startup after migrations run and before background processing starts.

Operational guidance:

- Keep the default enabled for public testnet operation.
- Set the value to `0` only during a bounded investigation where retaining all abuse signals is intentional.
- Restart the service after changing the value so startup pruning runs with the new window.
- Pruning affects only `abuse_signals`; claim history and transaction records are preserved.

Example manual inspection from the VPS:

```bash
sqlite3 /var/lib/scavium-faucet/scavium-faucet.db   "SELECT kind, COUNT(*) FROM abuse_signals WHERE created_at >= datetime('now','-24 hours') GROUP BY kind ORDER BY COUNT(*) DESC;"
```

## Phase 16 observability closure

Phase 16 closes with the following operational baseline:

- request and correlation IDs are available as response headers and structured log fields
- access logs and claim-flow logs are JSON and safe for journald/log aggregation
- `/api/v1/admin/metrics` provides lightweight process-local counters behind the admin bearer token
- `/health` carries liveness, uptime, and build identity
- `/ready` carries real dependency probes, per-check duration, and aggregate summary counts

No reverse-proxy exposure change is required. Keep the backend bound to loopback and expose only through nginx/TLS.


## Token validation operations (Phase 17.3.1)

Phase 17.3.1 validates token selection at claim time before captcha, risk, cooldown, rate-limit, daily-budget, persistence, and queue processing. Operators should use this flow when validating a new token registration:

1. Restart the service after changing `SCAVIUM_FAUCET_TOKENS_JSON`.
2. Confirm the token appears in `GET /api/v1/tokens` or `GET /api/v1/faucet/tokens`.
3. Submit a claim with the selected `token_id`.
4. Submit a negative claim with an unknown `token_id` and confirm the response uses `claim_rejected` with `invalid_token` as the reason.
5. Review `/api/v1/admin/metrics` and confirm `claims.invalid_token` increments for rejected token selections.

An invalid token rejection is expected to happen before the claim is persisted or enqueued. It does not indicate RPC, wallet, queue, or SQLite failure.

## Token-aware enforcement operations (Phase 17.3.2)

Cooldown and rate-limit checks now use the resolved token id as part of their enforcement scope during claim creation. This means a claim for one configured token no longer blocks an otherwise valid claim for another configured token solely because the same address, IP, or fingerprint was used.

Operational notes:

- Validate the token exists first with `GET /api/v1/tokens`.
- Submit one claim with the default token and one claim with a non-default token from the same test wallet to verify independent cooldown behavior.
- Submit repeated claims for the same token, source IP, and wallet to verify the existing cooldown/rate-limit protections still apply per token.
- Existing rejection contracts are unchanged: cooldown still returns `cooldown_active`, rate limiting still returns `rate_limited`, and daily budget exhaustion still returns `daily_budget_exceeded`.

## Token-aware observability operations (Phase 17.3.3)

`GET /api/v1/admin/metrics` now exposes token-scoped counters alongside the existing aggregate counters. Use this view when validating multi-token behavior after adding or changing `SCAVIUM_FAUCET_TOKENS_JSON`.

Operational notes:

- `tokens[*].token_id` matches the public catalog token id when a token was explicitly selected or resolved by the claim response.
- The `default` bucket represents claims that omitted `token_id` at the HTTP boundary.
- `tokens[*].invalid_token` helps identify typo, stale frontend, or hostile token selection attempts.
- Token-scoped counters reset on process restart and are not a replacement for SQLite claim/abuse-signal inspection.
- Structured logs may include safe token ids and event markers, but must not include wallet addresses, raw fingerprints, captcha tokens, request bodies, secrets, or idempotency-key values.

Quick check:

```bash
curl -sS -H "Authorization: Bearer $SCAVIUM_FAUCET_ADMIN_TOKEN" \
  http://127.0.0.1:18080/api/v1/admin/metrics
```


## Claim validation closure operations (Phase 17.3 closed)

Phase 17.3 is the active operator baseline for token-aware claim intake. When validating a deployment after token configuration changes, operators should check the full chain rather than only the catalog response:

1. Confirm configured tokens appear in `GET /api/v1/tokens` or `GET /api/v1/faucet/tokens`.
2. Submit a claim without `token_id` and confirm it follows the configured default-token path.
3. Submit a claim with a configured non-default token and confirm enforcement is scoped to that token.
4. Submit a claim with an unknown `token_id` and confirm it is rejected through `claim_rejected` with `invalid_token` as the reason.
5. Review `/api/v1/admin/metrics` and confirm aggregate and token-scoped counters move as expected.

This closure does not introduce runtime token mutation, frontend token selection, database-backed catalogs, durable per-token analytics, or external metrics. Those remain later-phase concerns.

## Frontend Token Selection Operations (Phase 17.4.1)

The public embedded faucet UI now discovers configured faucet assets through `GET /api/v1/tokens` and renders them as a browser-side selector. Operators should validate the catalog after any `SCAVIUM_FAUCET_TOKENS_JSON` change and service restart before testing claims from the UI.

Recommended validation flow:

```bash
curl -sS https://faucet.testnet.scavium.network/api/v1/tokens
```

Then open the public faucet page and confirm that each configured token appears with its symbol, token id, type, and base-unit amount. If the catalog endpoint is unavailable from the browser, the UI intentionally falls back to the legacy/default claim behavior and does not send `token_id`.

Troubleshooting notes:

- If the selector is hidden, validate `/api/v1/tokens` first.
- If a token is missing, review `SCAVIUM_FAUCET_TOKENS_JSON` and restart the service.
- If a selected token claim returns `claim_rejected` with `invalid_token`, refresh the page and re-check the server-side catalog.

## Frontend token selector UX checks (Phase 17.4.2)

After deploying the frontend bundle, validate token selector behavior from a browser session:

1. Load the faucet page and confirm the token row reports catalog loading before the catalog response is rendered.
2. Confirm configured tokens appear in the selector using public catalog metadata only.
3. Select a non-default token and confirm the detail cards show amount, type, and decimals.
4. Temporarily block or fail `/api/v1/tokens` in a local/browser test and confirm the UI reports catalog fallback while preserving default-token claim behavior.
5. Submit a claim and confirm the browser request includes `token_id` only when a catalog token is selected.

The fallback state is intentional: catalog discovery failure must not make legacy/default-token claims impossible. Operators should still inspect `/api/v1/tokens`, `/api/v1/status`, and `/api/v1/admin/metrics` when diagnosing token selector issues.

## Frontend claim-result UX checks (Phase 17.4.3)

After deploying the frontend bundle, validate the post-claim result panel with both default and non-default token claims:

1. Submit a default/native claim and confirm the result panel still renders even when no explicit `token_id` is present.
2. Submit a configured ERC20 claim and confirm the summary shows the selected token symbol/id, resolved amount, token type, and status.
3. Confirm the detailed rows still include claim id, status, address, amount, transaction hash, and timestamps when returned by the backend.
4. Confirm the explorer action appears only when both `tx_hash` and the configured explorer transaction URL are available.
5. Confirm no address masking, token metadata, or explorer-copy changes alter the backend claim response contract.

This is a presentation-layer alignment only. API validation, token-scoped enforcement, and token-aware metrics remain owned by the backend layers closed in Phase 17.3.

## Frontend token-aware closure checks (Phase 17.4 closed)

Phase 17.4 is the active operator baseline for the embedded token-aware faucet UI. After deploying a frontend bundle or changing token configuration, operators should validate the complete browser-facing flow rather than only the backend catalog:

1. Confirm `GET /api/v1/tokens` returns the expected configured tokens after service restart.
2. Open the public faucet page and confirm the selector loads from the catalog.
3. Confirm catalog failure preserves default-token claim behavior by omitting `token_id`.
4. Submit a selected-token claim and confirm the request includes only the selected public token id.
5. Confirm accepted and polled claim results render token-aware summaries while preserving the raw claim details.

This closure does not introduce runtime token administration, token icons, balance display, database-backed catalogs, or any new frontend configuration source.


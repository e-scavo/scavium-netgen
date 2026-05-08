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
- On `SIGINT` or `SIGTERM`, the process stops accepting HTTP traffic with a bounded HTTP shutdown context, then uses a separate bounded application-cleanup context to cancel background runtime work and close owned resources once.

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


### Phase 22 RPC failover and wallet runtime visibility

RPC failover is intentionally startup-only. Configure the primary endpoint in `SCAVIUM_FAUCET_RPC_URL` and optional fallback candidates in `SCAVIUM_FAUCET_RPC_SECONDARY_URLS`. On startup the app dials candidates in order and validates the configured chain ID before keeping one. After startup, one selected RPC client is shared by sender, watcher, readiness, and wallet visibility. Do not treat this as load balancing or high availability; if the selected endpoint degrades later, alerts and operator restart/failover remain the safe response.

Admin-only wallet visibility is available at:

```bash
curl -sS http://127.0.0.1:18080/api/v1/admin/wallet \
  -H "Authorization: Bearer $SCAVIUM_FAUCET_ADMIN_TOKEN"
```

The same object is embedded under `wallet` in `/api/v1/admin/runtime`. Operators should use `native_balance_wei`, `pending_nonce`, and token balance statuses to detect refill needs, nonce stalls, and ERC20 funding gaps. The response deliberately excludes private keys, secrets, raw RPC credentials, and request material. If `status` is `degraded`, inspect `/ready`, RPC logs, Besu health, and ERC20 contract availability before resuming distribution.

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

## Phase 30 binary migration

Use `scripts/migrate-scavium-faucet-phase30.sh` when replacing the current production binary with the Phase 30 build. The helper is plan-first and expects an already-reviewed local binary and supports both the preferred release layout (`APP_PATH/current -> APP_PATH/releases/<release>`) and the current legacy/direct binary layout (`APP_PATH/bin/scavium-faucet`). It supports normal non-root VPS users by copying artifacts to `REMOTE_STAGE_DIR` first and executing privileged release/backup/systemd work through `REMOTE_SUDO` (`sudo` by default). With `CONFIG_CANDIDATES=yes` it stages repository env/nginx references on the VPS, generates `/etc/scavium-faucet/scavium-faucet.env.phase30-candidate` by carrying forward active values for still-supported keys and commenting legacy keys, and stages `${REMOTE_NGINX_SITE}.phase30-candidate` for diff review. It applies those candidates only with `APPLY_ENV_CANDIDATE=yes` or `APPLY_NGINX_CANDIDATE=yes`. It creates and verifies a remote SQLite/config backup before activation, including live config, generated candidates, and the previous direct binary inside the backup bundle when the VPS uses `/opt/scavium-faucet/bin/scavium-faucet`, restarts systemd, validates `/health`, `/ready`, `/api/v1/status`, `/api/v1/tokens`, and admin runtime/wallet endpoints when the token is readable from the VPS against `SMOKE_BASE_URL` (default `https://DEPLOY_HOST`), using separate public/admin smoke timeouts so slow admin wallet reads do not cause false rollbacks, then restores applied config plus the previous symlink or direct binary if smoke validation fails.

Detailed operator steps are documented in [deployment-phase30-migration.md](deployment-phase30-migration.md). Keep `SCAVIUM_FAUCET_WALLET_ALLOWED_ORIGINS` unset for legacy-only rollout, or set it to exact browser application origins before enabling browser wallet challenge/proof flows. Missing `Origin` remains valid for native, desktop, mobile, CLI, and server-to-server clients. The systemd file under `docs/scavium-faucet/deployment/` remains a first-install/reference template; nginx is staged as a candidate and installed only with explicit apply mode.

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

The request was blocked by the risk engine. In the Phase 27 path this can mean progressive negative-signal scoring, same-IP burst detection across successful or failed intake signals, same-fingerprint rotating IP behavior, address clustering, or an enabled honeypot challenge. The response `details.reason` contains only a bounded category. Review `SCAVIUM_FAUCET_ABUSE_*` thresholds and recent `abuse_signals` rows, including bounded `manual_review` hints, before loosening controls; do not copy raw IPs, addresses, fingerprints, or user agents into metric labels or public diagnostics.

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

## Token support closure checks (Phase 17 closed)

Phase 17 is the active operator baseline for multi-token faucet operation. After deploying a Phase 17 build or changing token configuration, validate the whole token-aware surface end to end:

1. Confirm the service starts with the intended `SCAVIUM_FAUCET_TOKENS_JSON` and `SCAVIUM_FAUCET_DEFAULT_TOKEN_ID` values.
2. Confirm `GET /api/v1/tokens` and `GET /api/v1/faucet/tokens` expose only claim-safe public token metadata.
3. Submit a default claim without `token_id` and confirm the configured default token path still works.
4. Submit a configured non-default token claim and confirm validation, cooldown/rate-limit scope, daily budget accounting, persistence, queue processing, and claim-result rendering all stay scoped to that token.
5. Check `/api/v1/admin/metrics` for aggregate and token-scoped counters without relying on them as durable accounting.

This closure does not introduce runtime token mutation, database-backed token catalogs, durable per-token analytics, token icons, balance display, or a new admin-control surface. Those remain Phase 18+ concerns.

## Post-audit token support checks (Phase 17.5 closed)

Phase 17.5 is the post-audit fix baseline for the completed token-support layer. After deploying a Phase 17.5 build, operators should validate the specific audit corrections in addition to the broader Phase 17 checks:

1. Confirm `/api/v1/status` drives the public frontend status banner from the `status` field, including paused, maintenance, and no-funds states.
2. Trigger or simulate a cooldown rejection and confirm the frontend displays `retry_after_seconds` when returned in the error details.
3. Submit accepted and rejected claims without `token_id` and confirm `/api/v1/admin/metrics` reports both paths under the same `default` token bucket.
4. Submit a rejected claim with a malformed or control-character token id in a non-production test and confirm logs/metrics do not retain raw control characters.
5. Re-run `go test ./...` before merging or deploying additional admin-control work.

This closure does not introduce new operational endpoints, runtime token mutation, database-backed token catalogs, or durable analytics. It only records the post-audit corrections that keep the Phase 17 token-aware baseline production-safe before Phase 18.

## Admin control closure checks (Phase 18.8 closed)

Phase 18.8 is the final closure pass for the Phase 18 admin-control surface. It keeps the public faucet contracts unchanged and records the production scope of the admin plane before Phase 19 hardening.

After deploying a Phase 18.8 build, validate the admin surface with the configured bearer token:

```bash
curl -sS http://127.0.0.1:18080/api/v1/admin/runtime \
  -H "Authorization: Bearer $SCAVIUM_FAUCET_ADMIN_TOKEN"

curl -sS http://127.0.0.1:18080/api/v1/admin/queue?limit=50 \
  -H "Authorization: Bearer $SCAVIUM_FAUCET_ADMIN_TOKEN"

curl -sS http://127.0.0.1:18080/api/v1/admin/audit?limit=100 \
  -H "Authorization: Bearer $SCAVIUM_FAUCET_ADMIN_TOKEN"
```

Operational scope notes:

- `POST /api/v1/admin/faucet/mode` is runtime-effective. Accepted values are `active`, `paused`, and `maintenance`; invalid modes return `400 invalid_mode`.
- `GET /api/v1/admin/queue` and `GET /api/v1/admin/claims` read persisted SQLite claim/queue state (Phase 20.1).
- Queue and claim retry/cancel commands update persisted SQLite claim state (Phase 20.2). Retry clears `next_attempt_at` and re-queues eligible `failed`/`rejected` claims for worker pickup; cancel rejects eligible not-yet-sent claims.
- `GET /api/v1/admin/audit` reads persisted SQLite admin audit state (Phase 20.3).
- Admin blocklist values are visible through the admin blocklist surface, while admin audit rows store only safe metadata (for example action, actor, key type, and timestamp) and do not persist raw blocklist values.
- Admin blocklist entries are persisted in SQLite and enforced during public claim intake for `ip`, `address`, and `fingerprint` scopes.
- `key_type` for blocklist add/remove is restricted to `ip`, `address`, or `fingerprint`.
- Admin list limits for queue, claims, and audit are capped at `500` even if a larger `limit` query value is supplied.

This closure does not introduce dynamic budget editing, CSV exports, allowlist/campaign management, or runtime token mutation. Those remain explicitly deferred from the current production-safe admin-control baseline.


## Phase 19 hardening validation checklist

Use this checklist after deploying the Phase 19 hardened binary behind nginx:

1. Confirm the backend still binds to the intended loopback address, usually `127.0.0.1:18080`.
2. Confirm nginx terminates TLS and owns `Strict-Transport-Security` policy; the Go backend intentionally does not emit HSTS.
3. Verify representative responses include the backend security headers: `/health`, `/ready`, `/api/v1/config`, `/api/v1/tokens`, `/api/v1/admin/runtime` when admin auth is enabled, and `/`.
4. Submit an oversized write request and confirm it is rejected as `413 request_body_too_large` without changing the claim contract for normal requests.
5. Re-run `go test ./...` before merge and after applying the partial ZIP.
6. Exercise service restart or `systemctl restart scavium-faucet` and confirm graceful shutdown logs complete without duplicate resource-close errors.
7. Confirm the public nginx response contains one copy of each security header after the template's `proxy_hide_header` directives are active.
8. Submit a JSON write request with `Content-Type: text/plain` and confirm it is rejected as `415 unsupported_media_type`.

Phase 19.6 closes the post-audit fixes for duplicate proxy headers, shutdown budgets, JSON content-type enforcement, and test timeout hygiene. If validation finds another runtime defect, fix it in a later implementation phase rather than widening this closure pass.

## Phase 21 operator observability and alerting baseline

Phase 21 keeps observability dependency-free and admin-protected. The JSON endpoint remains:

```bash
curl -s http://127.0.0.1:18080/api/v1/admin/metrics \
  -H "Authorization: Bearer $SCAVIUM_FAUCET_ADMIN_TOKEN"
```

A Prometheus-compatible text export is now available at the protected endpoint below. It is intentionally served by the same admin bearer-token middleware as the JSON metrics endpoint and must not be exposed directly from nginx as a public route.

```bash
curl -s http://127.0.0.1:18080/api/v1/admin/metrics/prometheus \
  -H "Authorization: Bearer $SCAVIUM_FAUCET_ADMIN_TOKEN"
```

The export uses only bounded labels; token-scoped metrics have an in-process overflow bucket to prevent unbounded series growth. Token metrics are labeled by configured/requested token id after the existing sanitization path when the token is trusted by the claim flow; invalid token rejections use `invalid`, early unavailable errors use `default`, and overflow uses `other`; wallet addresses, IP addresses, fingerprints, captcha tokens, idempotency keys, request bodies, private keys, admin tokens, and RPC credentials are never exported.

Additional Phase 21 counters cover worker dequeue/send/ack outcomes, watcher pending/confirmed/reverted/stuck/RPC outcomes, and an aggregate blocklist-rejection signal for blocklist-spike alerting. These counters reset on process restart and are operational signals, not durable accounting. Durable claim, queue, abuse, admin audit, blocklist, and mode state remain SQLite-backed.

### Local smoke test

Run the local smoke script after deployment or rollback from the repository root on the VPS:

```bash
SCAVIUM_FAUCET_BASE_URL=http://127.0.0.1:18080 \
SCAVIUM_FAUCET_ADMIN_TOKEN="$SCAVIUM_FAUCET_ADMIN_TOKEN" \
bash scripts/scavium-faucet-operator-smoke.sh
```

Without `SCAVIUM_FAUCET_ADMIN_TOKEN`, the script still verifies public health, readiness, status, and token catalog endpoints and skips admin checks. It does not send claims, mutate queue state, touch wallets, or require secrets beyond the already-configured admin token for protected checks.

### Alert threshold guidance

Use these as initial operator thresholds and tune after observing real baseline traffic:

- **Low balance:** alert when `/ready` reports the chain balance check degraded or when wallet balance is below the documented refill floor for the configured token set. Treat low native balance as urgent even for ERC20 payouts because gas is still required.
- **RPC unavailable:** alert on `/ready` degraded for the chain check, repeated `scavium_faucet_watcher_rpc_failed_total` increases, or sustained worker send failures with RPC-related log messages.
- **Stuck queue:** inspect `/api/v1/admin/queue` when queued ready items remain non-zero while `scavium_faucet_queue_dequeued_total` is flat for more than two worker poll intervals, or when `sending` claims repeatedly reappear through watcher stuck reconciliation.
- **Failed transaction spike:** alert when `scavium_faucet_queue_send_failed_total` or `scavium_faucet_watcher_reverted_total` increases sharply compared with accepted claims. Correlate with Besu txpool, gas policy, nonce behavior, and wallet balance before retrying claims.
- **Captcha spike:** alert when `scavium_faucet_captcha_failed_total` rises rapidly or dominates rejected claims. Check captcha provider status, frontend integration, and possible bot traffic before relaxing enforcement.
- **Blocklist spike:** alert when `scavium_faucet_abuse_blocklist_rejected_total` rises quickly. Use `/api/v1/admin/blocklist` and audit logs to review recent operator blocklist changes. A sudden increase in this counter with matching abuse logs may indicate targeted abuse or an overly broad manual block.
- **High rejection rate:** compare `scavium_faucet_claims_rejected_total` to `scavium_faucet_claims_accepted_total`. Sustained rejection dominance should trigger review of rate limits, cooldowns, captcha provider health, token configuration, and abuse signals.

### nginx and journald correlation

Keep nginx access logs enabled at the reverse proxy and correlate them with application JSON logs using `X-Request-ID`/`X-Correlation-ID`. For protected metrics collection, scrape through a private network path or localhost tunnel that supplies `Authorization: Bearer <admin-token>`; do not add a public unauthenticated nginx location for `/api/v1/admin/metrics/prometheus`.

### Phase 21 implementation verification note

The Phase 21 metrics instrumentation is expected to remain backward compatible with existing internal constructors used by tests and embedding code. `worker.New(...)` and `chain.NewWatcher(...)` still support nil metrics instrumentation; production wiring uses `NewWithMetrics(...)`, but legacy constructor paths must not panic when metrics are omitted.

## Phase 23 backup, restore, refill, and rotation operations

Phase 23 makes the manual production operations repeatable without adding automatic treasury movement or unsafe background restore behavior. The scripts are review-first and local-only; operators must still decide when to stop/start services, where to store encrypted backups, and when to move real funds.

### SQLite and configuration backup

Use the backup helper in plan mode first:

```bash
SCAVIUM_FAUCET_DATABASE_PATH=/var/lib/scavium-faucet/scavium-faucet.db \
SCAVIUM_FAUCET_ENV_FILE=/etc/scavium-faucet/scavium-faucet.env \
scripts/scavium-faucet-backup.sh --plan
```

Create a restricted local bundle only after reviewing the paths:

```bash
SCAVIUM_FAUCET_DATABASE_PATH=/var/lib/scavium-faucet/scavium-faucet.db \
SCAVIUM_FAUCET_ENV_FILE=/etc/scavium-faucet/scavium-faucet.env \
SCAVIUM_FAUCET_BACKUP_DIR=/secure/offline/scavium-faucet-backups \
scripts/scavium-faucet-backup.sh --execute
```

Verify the bundle before relying on it. Verification checks archive readability, rejects unsafe archive paths and link entries, requires `db/scavium-faucet.db` and `SHA256SUMS`, and validates the recorded checksums before printing the bundle entries:

```bash
SCAVIUM_FAUCET_BACKUP_FILE=/secure/offline/scavium-faucet-backups/scavium-faucet-backup-YYYYMMDDTHHMMSSZ.tar.gz \
scripts/scavium-faucet-backup.sh --verify
```

The bundle can include the reviewed environment file, which may contain `SCAVIUM_FAUCET_PRIVATE_KEY`, `SCAVIUM_FAUCET_ADMIN_TOKEN`, captcha secrets, and RPC credentials. Treat backup archives as secret material: store them encrypted, restrict filesystem permissions, and never attach them to tickets, chats, or public artifacts.

### Restore drill and production restore

Always perform a dry-run restore plan first. The restore helper validates the tar listing, rejects absolute paths, parent-directory paths, symlinks, and hardlinks, and verifies `SHA256SUMS` before any execute-mode write:

```bash
SCAVIUM_FAUCET_RESTORE_BUNDLE=/secure/offline/scavium-faucet-backups/scavium-faucet-backup-YYYYMMDDTHHMMSSZ.tar.gz \
SCAVIUM_FAUCET_DATABASE_PATH=/var/lib/scavium-faucet/scavium-faucet.db \
scripts/scavium-faucet-restore.sh --plan
```

For production restore, use an explicit maintenance window:

1. Put the faucet in maintenance mode through the admin API when available.
2. Stop the service: `sudo systemctl stop scavium-faucet`.
3. Snapshot the current DB/env paths out-of-band if possible.
4. Execute restore with an explicit confirmation flag:

```bash
SCAVIUM_FAUCET_RESTORE_BUNDLE=/secure/offline/scavium-faucet-backups/scavium-faucet-backup-YYYYMMDDTHHMMSSZ.tar.gz \
SCAVIUM_FAUCET_DATABASE_PATH=/var/lib/scavium-faucet/scavium-faucet.db \
SCAVIUM_FAUCET_RESTORE_CONFIRM=yes \
scripts/scavium-faucet-restore.sh --execute
```

5. Restore configuration only when the env file itself is part of the recovery scope:

```bash
SCAVIUM_FAUCET_RESTORE_BUNDLE=/secure/offline/scavium-faucet-backups/scavium-faucet-backup-YYYYMMDDTHHMMSSZ.tar.gz \
SCAVIUM_FAUCET_DATABASE_PATH=/var/lib/scavium-faucet/scavium-faucet.db \
SCAVIUM_FAUCET_ENV_FILE=/etc/scavium-faucet/scavium-faucet.env \
SCAVIUM_FAUCET_RESTORE_CONFIG=yes \
SCAVIUM_FAUCET_RESTORE_CONFIRM=yes \
scripts/scavium-faucet-restore.sh --execute
```

6. Start the service: `sudo systemctl start scavium-faucet`.
7. Verify `systemctl status`, `/health`, `/ready`, `/api/v1/admin/runtime`, and `/api/v1/admin/wallet`.
8. Review recent queue entries and audit logs before leaving maintenance mode.

The restore helper refuses live restore when it can detect an active systemd service. Do not bypass that guard unless you intentionally accept SQLite consistency risk for a non-production drill. When a fallback backup contains SQLite `-wal` or `-shm` companion files, restore installs them next to the target database and removes stale target companions when they are absent from the verified bundle.

### Manual wallet refill

There is no automatic treasury refill in Phase 23. Operators should refill only after confirming the signer address and balances through admin-only visibility:

```bash
curl -fsS http://127.0.0.1:18080/api/v1/admin/wallet \
  -H "Authorization: Bearer $SCAVIUM_FAUCET_ADMIN_TOKEN"
```

Refill checklist:

1. Confirm the reported signer address matches the intended faucet hot wallet.
2. Confirm `native_balance_wei`, `pending_nonce`, and each ERC20 token balance/status.
3. Confirm claim mode is appropriate. Use maintenance or pause mode if refill timing could confuse users.
4. From a separate treasury wallet, send a small test amount first.
5. Wait for confirmations and verify `/api/v1/admin/wallet` again.
6. Send the remaining planned refill amount only after the test transaction is visible.
7. Record the treasury transaction hash, amount, token, operator, and reason in the operational log.

Never paste `SCAVIUM_FAUCET_PRIVATE_KEY` into a treasury tool. The faucet hot wallet receives funds; it should not be used as a treasury source.

### Manual wallet rotation

Wallet rotation is configuration-driven and intentionally manual. No Phase 23 script moves funds or rewrites secrets automatically.

Safe rotation sequence:

1. Schedule a maintenance window and announce it if public traffic is expected.
2. Put the faucet into maintenance mode and let the worker drain safe in-flight claims.
3. Back up SQLite and configuration with `scripts/scavium-faucet-backup.sh --execute`.
4. Generate the new hot wallet outside the repository using the approved operator wallet tooling.
5. Fund the new hot wallet with a small native amount and any required ERC20 balances.
6. Update the real environment file outside Git with the new `SCAVIUM_FAUCET_PRIVATE_KEY`.
7. Restart the service and verify `/ready` plus `/api/v1/admin/wallet` show the new signer address, expected chain, balance, and pending nonce.
8. Submit one bounded test claim for the native token and, if configured, one ERC20 test claim per token.
9. Move leftover funds from the old hot wallet only after the new wallet is proven healthy.
10. Keep the old key material sealed until rollback is no longer needed, then retire it according to the operator key-retention policy.

Rollback during rotation is also manual: restore the previous environment file or backup bundle, restart the service, verify the old signer address through `/api/v1/admin/wallet`, and only then re-enable public claims.

### Deployment rollback verification checklist

After every rollback or restore, verify the full operator loop before declaring recovery complete:

```bash
systemctl status scavium-faucet --no-pager
journalctl -u scavium-faucet -n 100 --no-pager
curl -fsS http://127.0.0.1:18080/health
curl -fsS http://127.0.0.1:18080/ready
curl -fsS http://127.0.0.1:18080/api/v1/admin/runtime -H "Authorization: Bearer $SCAVIUM_FAUCET_ADMIN_TOKEN"
curl -fsS http://127.0.0.1:18080/api/v1/admin/wallet -H "Authorization: Bearer $SCAVIUM_FAUCET_ADMIN_TOKEN"
scripts/scavium-faucet-operator-smoke.sh
```

If `/ready` is degraded, keep the faucet paused/maintenance and inspect DB, queue, RPC failover selection, wallet balance, and ERC20 contract reachability before accepting public claims.

## Phase 26 public frontend smoke checks

After deploying the Phase 26 frontend, run these browser checks against the public URL:

1. Open the homepage and confirm the request form, token selector, refresh button, eligibility button, history button, privacy link, and terms link are reachable by keyboard.
2. Enter an invalid address and confirm claim, eligibility, and history actions reject it locally without making a successful claim.
3. Enter a valid address and click **Check Eligibility**. Confirm the page renders `eligible`, public reason, cooldown, default token, token status, and daily-budget information when those fields are returned by the API.
4. Click **View Address History**. Confirm an empty-state message appears for addresses with no claims and a capped recent-claims list appears for addresses with claims.
5. Submit a normal claim and confirm the existing claim result/polling flow still works.
6. If `SCAVIUM_FAUCET_EXPLORER_TX_URL` is configured, confirm explorer actions appear only after a valid `tx_hash` is returned. Remove or malform the explorer template in a staging environment and confirm the UI suppresses links.
7. Put the faucet in `paused`, `maintenance`, and `no_funds` modes and confirm the public banner disables submission without hiding read-only status/history actions.
8. Resize the viewport below 640px and confirm buttons, token details, status cards, and history entries stack without horizontal scrolling.


## Runtime policy rollback

Phase 28 runtime policy changes are admin-protected and persisted in SQLite. Inspect current overrides with:

```bash
curl -s -H "Authorization: Bearer $SCAVIUM_FAUCET_ADMIN_TOKEN" \
  http://127.0.0.1:18080/api/v1/admin/policy
```

Replace the complete override set with `PUT /api/v1/admin/policy`, using only non-secret budget/throttle fields. To roll back to environment/default configuration immediately:

```bash
curl -s -X DELETE -H "Authorization: Bearer $SCAVIUM_FAUCET_ADMIN_TOKEN" \
  http://127.0.0.1:18080/api/v1/admin/policy
```

After a change, check `/api/v1/config`, `/api/v1/tokens`, an address eligibility response, and `/api/v1/admin/audit?limit=20`. The public token catalog reflects runtime daily-budget overrides for the same claim-safe budget fields exposed by `/api/v1/config`, so it should not drift from policy enforcement after an admin update. Do not use runtime policy for secrets, RPC endpoints, token contract metadata, or signer configuration; those remain restart-managed.



## Phase 30 smoke timeout false rollback

If the migration reaches `/api/v1/admin/wallet`, the service journal records `status:200`, and the helper still rolls back with `curl: (28) Operation timed out`, the backend is running but the admin wallet smoke exceeded the curl timeout. The current helper uses `SMOKE_ADMIN_TIMEOUT_SECONDS=30` and one retry by default. Rerun the migration with the current source; if the wallet/RPC path is still slow, set a larger explicit value such as `SMOKE_ADMIN_TIMEOUT_SECONDS=45`. Keep public endpoint failures and nginx `502` responses as hard rollback signals.

## Phase 30 SQLite migration startup failure

If a Phase 30 migration causes nginx `502` responses and the service journal shows an error similar to `apply migration 004_token_claim_metadata.sql: duplicate column name: token_id`, the backend failed during SQLite startup migration. The migration runner in the current source handles partially applied `ALTER TABLE ... ADD COLUMN` migrations idempotently: it skips duplicate-column additions, applies remaining missing columns/indexes, and records the migration only after the full transaction succeeds. Rebuild the binary from this source and rerun `scripts/migrate-scavium-faucet-phase30.sh --execute`; the helper will keep taking a pre-migration backup and will roll back the previous binary if smoke still fails.

# Configuration reference

Configuration is loaded from environment variables at startup via `internal/config`. All variables listed below are active in the current runtime unless noted as _loaded but not yet enforced_.

## Variables

| Variable | Default | Effect |
|---|---|---|
| `SCAVIUM_FAUCET_BIND_ADDR` | `127.0.0.1:18080` | HTTP listen address used by `main.go` |
| `SCAVIUM_FAUCET_PUBLIC_BASE_URL` | `http://127.0.0.1:18080` | Loaded and validated; not currently surfaced by the handler |
| `SCAVIUM_FAUCET_RPC_URL` | `http://127.0.0.1:18545` | Ethereum JSON-RPC endpoint; used to create `chain.Client` at startup (non-dry-run); drives `RPCCheck` and `WalletCheck` in `/ready` |
| `SCAVIUM_FAUCET_CHAIN_ID` | `31337` | Validated against the RPC node at startup (non-dry-run); exposed by `/api/v1/config` |
| `SCAVIUM_FAUCET_NETWORK_NAME` | `scavium-dev` | Exposed by `/api/v1/status` and `/api/v1/config` |
| `SCAVIUM_FAUCET_SYMBOL` | `SCAV` | Exposed by `/api/v1/status` and `/api/v1/config` |
| `SCAVIUM_FAUCET_EXPLORER_TX_URL` | empty | Exposed by `/api/v1/config` |
| `SCAVIUM_FAUCET_AMOUNT_WEI` | `1000000000000000000` | Legacy/default native token amount; still copied into claims when no token override is configured |
| `SCAVIUM_FAUCET_TOKENS_JSON` | empty | Optional JSON array of claimable tokens. When empty, the faucet exposes one backward-compatible native token from `SYMBOL` + `AMOUNT_WEI`; when set, the public catalog is exposed by `/api/v1/tokens` and `/api/v1/faucet/tokens` |
| `SCAVIUM_FAUCET_DEFAULT_TOKEN_ID` | `native` | Token used when a claim omits `token_id`; preserves the existing claim contract |
| `SCAVIUM_FAUCET_COOLDOWN_SECONDS` | `86400` | Per-address cooldown enforced by `PersistentReadService`; exposed by `/api/v1/config` and address-status responses |
| `SCAVIUM_FAUCET_DRY_RUN` | `true` | When `true`, uses `DryRunSender` and skips RPC/wallet startup checks; exposed by `/api/v1/status` and `/api/v1/config` |
| `SCAVIUM_FAUCET_DATABASE_PATH` | `cmd/scavium-faucet/data/scavium-faucet.db` | Mandatory path to the SQLite database file used for persistence; created with parent directories if missing; migrations run automatically on open |
| `SCAVIUM_FAUCET_RATE_LIMIT_IP_PER_HOUR` | `10` | Maximum claims per source IP per hour; enforced by the persistent rate limiter on claim creation; exposed by `/api/v1/config` |
| `SCAVIUM_FAUCET_RATE_LIMIT_ADDR_PER_DAY` | `3` | Maximum claims per Ethereum address per day; enforced by the persistent rate limiter on claim creation; exposed by `/api/v1/config` |
| `SCAVIUM_FAUCET_DAILY_BUDGET_WEI` | empty | Maximum total amount distributed per UTC day; unset means unlimited |
| `SCAVIUM_FAUCET_ABUSE_ENFORCEMENT_ENABLED` | `true` | Enables progressive enforcement using recent abuse signals |
| `SCAVIUM_FAUCET_ABUSE_ENFORCEMENT_WINDOW_SECONDS` | `3600` | Lookback window for progressive abuse enforcement |
| `SCAVIUM_FAUCET_ABUSE_ENFORCEMENT_IP_THRESHOLD` | `20` | Negative signal threshold for a source IP within the enforcement window; `0` disables this scope |
| `SCAVIUM_FAUCET_ABUSE_ENFORCEMENT_ADDRESS_THRESHOLD` | `12` | Negative signal threshold for a wallet address within the enforcement window; `0` disables this scope |
| `SCAVIUM_FAUCET_ABUSE_ENFORCEMENT_FINGERPRINT_THRESHOLD` | `15` | Negative signal threshold for a browser fingerprint within the enforcement window; `0` disables this scope |
| `SCAVIUM_FAUCET_ABUSE_SIGNAL_RETENTION_DAYS` | `30` | Number of days to retain rows in `abuse_signals`; expired rows are pruned at startup; `0` disables pruning |
| `SCAVIUM_FAUCET_TRUSTED_PROXY` | empty | When set to the reverse proxy's IP, the handler extracts the real client IP from `X-Forwarded-For` / `X-Real-IP` via `internal/iputil`; used for rate limiting and logging |
| `SCAVIUM_FAUCET_CORS_ALLOWED_ORIGINS` | empty | Comma-separated exact origins allowed for public API CORS; empty disables CORS headers; wildcard `*` is not allowed |
| `SCAVIUM_FAUCET_PRIVATE_KEY` | empty | Hex-encoded signer key; required and validated at startup when `DRY_RUN=false`; used by `chain.EthSender` to sign transactions |
| `SCAVIUM_FAUCET_CAPTCHA_PROVIDER` | `disabled` | Selects the captcha backend: `disabled` (no check), `dev` (always pass), `hcaptcha`, `recaptcha`, or `turnstile`; active in claim creation when not `disabled` |
| `SCAVIUM_FAUCET_CAPTCHA_SITE_KEY` | empty | Public browser-side site key exposed by `/api/v1/config`; required for `hcaptcha`, `recaptcha`, or `turnstile` so the frontend can render the provider widget |
| `SCAVIUM_FAUCET_CAPTCHA_SECRET` | empty | Server-side secret for the chosen captcha provider; required when provider is `hcaptcha`, `recaptcha`, or `turnstile` |
| `SCAVIUM_FAUCET_CAPTCHA_VERIFY_URL` | provider default | Optional verification URL override. When empty, the runtime uses the provider default (`hcaptcha`, `recaptcha`, or Cloudflare Turnstile) |
| `SCAVIUM_FAUCET_MODE` | `active` | Operational mode reported by `/api/v1/status`; `active`, `paused`, or `maintenance` |
| `SCAVIUM_FAUCET_ADMIN_TOKEN` | empty | Bearer token for `/api/v1/admin/*` endpoints, including `/api/v1/admin/metrics`; admin API is active when non-empty and uses constant-time comparison; never logged |
| `SCAVIUM_FAUCET_WORKER_ENABLED` | `true` | Enables the background worker that processes the SQLite claim queue; default is enabled |
| `SCAVIUM_FAUCET_WORKER_POLL_SECONDS` | `5` | Worker polling interval in seconds |
| `SCAVIUM_FAUCET_WATCHER_ENABLED` | `false` (dry-run), auto `true` (non-dry-run) | Enables the background watcher that polls for on-chain confirmations; automatically enabled in production (`DRY_RUN=false`) unless explicitly set |
| `SCAVIUM_FAUCET_WATCHER_POLL_SECONDS` | `15` | Watcher polling interval in seconds |
| `SCAVIUM_FAUCET_MIN_CONFIRMATIONS` | `1` | Minimum on-chain confirmations required before a claim is marked `confirmed` |

## Validation rules

`Config.Validate()` enforces:

- bind address must be non-empty
- public base URL must be non-empty
- RPC URL must be non-empty
- chain ID must be positive
- network name must be non-empty
- symbol must be non-empty
- amount wei must be positive
- token configuration must include unique ids, supported types (`native` or `erc20`), positive amounts, and ERC20 contract addresses for `erc20` tokens
- cooldown seconds must be zero or positive
- worker poll seconds must be positive
- watcher poll seconds must be positive
- captcha provider must be one of `disabled`, `dev`, `hcaptcha`, `recaptcha`, or `turnstile`
- abuse signal retention days must be zero or positive
- `hcaptcha`, `recaptcha`, and `turnstile` require `SCAVIUM_FAUCET_CAPTCHA_SITE_KEY` and `SCAVIUM_FAUCET_CAPTCHA_SECRET`

Private key and admin token are not validated by `Config.Validate()`. A missing private key causes a startup error when `DRY_RUN=false`.

## Example environment

```ini
SCAVIUM_FAUCET_BIND_ADDR=127.0.0.1:18080
SCAVIUM_FAUCET_PUBLIC_BASE_URL=https://faucet.example.test
SCAVIUM_FAUCET_RPC_URL=http://127.0.0.1:18545
SCAVIUM_FAUCET_CHAIN_ID=1337
SCAVIUM_FAUCET_NETWORK_NAME=scavium-testnet
SCAVIUM_FAUCET_SYMBOL=SCAV
SCAVIUM_FAUCET_EXPLORER_TX_URL=https://explorer.example.test/tx/{txHash}

SCAVIUM_FAUCET_DATABASE_PATH=/var/lib/scavium-faucet/scavium-faucet.db
SCAVIUM_FAUCET_AMOUNT_WEI=1000000000000000000
# Optional multi-token configuration; omit to keep single native-token behavior.
# Keep the JSON value on one line in systemd environment files.
# SCAVIUM_FAUCET_DEFAULT_TOKEN_ID=native
# SCAVIUM_FAUCET_TOKENS_JSON=[{"id":"native","symbol":"SCAV","type":"native","decimals":18,"amount_wei":"1000000000000000000","daily_budget_wei":"100000000000000000000"},{"id":"scat","symbol":"SCAT","type":"erc20","address":"0x1111111111111111111111111111111111111111","decimals":18,"amount_wei":"25000000000000000000","daily_budget_wei":"2500000000000000000000"}]
SCAVIUM_FAUCET_COOLDOWN_SECONDS=86400
SCAVIUM_FAUCET_RATE_LIMIT_IP_PER_HOUR=10
SCAVIUM_FAUCET_RATE_LIMIT_ADDR_PER_DAY=3
SCAVIUM_FAUCET_DAILY_BUDGET_WEI=100000000000000000000
SCAVIUM_FAUCET_DRY_RUN=false

SCAVIUM_FAUCET_PRIVATE_KEY=replace-with-actual-hex-key

SCAVIUM_FAUCET_WORKER_ENABLED=true
SCAVIUM_FAUCET_WORKER_POLL_SECONDS=5
SCAVIUM_FAUCET_WATCHER_ENABLED=true
SCAVIUM_FAUCET_WATCHER_POLL_SECONDS=15
SCAVIUM_FAUCET_MIN_CONFIRMATIONS=1

SCAVIUM_FAUCET_TRUSTED_PROXY=127.0.0.1
SCAVIUM_FAUCET_CORS_ALLOWED_ORIGINS=https://faucet.example.test,https://wallet.example.test
SCAVIUM_FAUCET_MODE=active
SCAVIUM_FAUCET_ADMIN_TOKEN=replace-me

# Optional captcha (disabled by default)
# SCAVIUM_FAUCET_CAPTCHA_PROVIDER=turnstile
# SCAVIUM_FAUCET_CAPTCHA_SITE_KEY=replace-with-public-site-key
# SCAVIUM_FAUCET_CAPTCHA_SECRET=replace-with-captcha-secret
# Optional override; omitted uses the provider default verify endpoint.
# SCAVIUM_FAUCET_CAPTCHA_VERIFY_URL=https://challenges.cloudflare.com/turnstile/v0/siteverify
```


## Testnet token registration

Phase 17 token registration is intentionally configuration-driven. To add an ERC20 testnet asset, deploy or identify the ERC20 contract on the same chain configured by `SCAVIUM_FAUCET_CHAIN_ID`, then add an entry to `SCAVIUM_FAUCET_TOKENS_JSON` and restart the service. The public token catalog can be verified after restart with `GET /api/v1/tokens` or `GET /api/v1/faucet/tokens`.

Minimum ERC20 entry:

```json
{
  "id": "scat",
  "symbol": "SCAT",
  "type": "erc20",
  "address": "0x1111111111111111111111111111111111111111",
  "decimals": 18,
  "amount_wei": "25000000000000000000"
}
```

Operational requirements before enabling an ERC20 token:

- token id is stable, unique, and matches the `token_id` clients will submit
- contract address exists on the configured testnet chain
- decimals match the deployed ERC20 contract
- amount and budget values are expressed in base units, not display units
- faucet signer has enough native SCAV for gas and enough ERC20 balance for claims
- `SCAVIUM_FAUCET_DEFAULT_TOKEN_ID` points to a configured token

See [token-registration.md](token-registration.md) for the full testnet operator checklist and validation commands.

## Practical notes

- Keep secrets in an external environment file or service manager, not in the repository.
- Set `SCAVIUM_FAUCET_BIND_ADDR` to loopback and terminate TLS in a reverse proxy.
- Treat `SCAVIUM_FAUCET_ADMIN_TOKEN`, `SCAVIUM_FAUCET_PRIVATE_KEY`, and `SCAVIUM_FAUCET_CAPTCHA_SECRET` as secrets; none are logged by the binary. `SCAVIUM_FAUCET_CAPTCHA_SITE_KEY` is intentionally public and is surfaced to the frontend.
- No extra environment variable is required for Phase 16 observability. Request correlation, structured logs, health/readiness enrichment, and process-local metrics are active in the binary. `/api/v1/admin/metrics` is available only when `SCAVIUM_FAUCET_ADMIN_TOKEN` is set.
- Set `SCAVIUM_FAUCET_TRUSTED_PROXY` to the loopback or reverse proxy address so that IP-based rate limiting uses the real client IP rather than `127.0.0.1`.
- Set `SCAVIUM_FAUCET_CORS_ALLOWED_ORIGINS` only to exact public origins that should call the API from browsers. Leave it empty to disable CORS headers; `*` is rejected.
- `SCAVIUM_FAUCET_DAILY_BUDGET_WEI` is enforced against queued, sent, and confirmed claims and resets at UTC midnight.
- `SCAVIUM_FAUCET_ABUSE_ENFORCEMENT_*` is intentionally conservative: it reads only negative abuse signals from the configured lookback window and rejects new claims only when a configured threshold is reached. Set a threshold to `0` to disable that specific scope while keeping the rest of the enforcement layer active.
- In dry-run mode (`DRY_RUN=true`), the `PRIVATE_KEY` is not required; `DryRunSender` is used and no on-chain transactions are submitted.

## Phase 17.2 closure note

The Phase 17.2 token registration contract is configuration-driven. `SCAVIUM_FAUCET_TOKENS_JSON` and `SCAVIUM_FAUCET_DEFAULT_TOKEN_ID` are loaded at startup, validated before serving traffic, and then exposed through the public token catalog endpoints. Changing token definitions requires a service restart in this phase. Runtime mutation, hot reload, admin-driven token creation, and database-backed token catalogs are intentionally deferred to later phases.



## Phase 17.3.1 validation note

The configuration loader still validates token definitions at startup, and Phase 17.3.1 adds a second defensive validation boundary at claim time. Runtime claims only proceed when the selected token id resolves to executable metadata: valid token type, positive amount, non-negative decimals, and an ERC20 contract address when `type` is `erc20`. Invalid token selections are rejected with the existing claim error contract instead of reaching persistence, queue, or sender execution.

## Token-aware enforcement scope

As of Phase 17.3.2, claim-time cooldown and rate-limit checks are scoped by the resolved token id. The configured token catalog therefore defines not only transfer metadata, but also the unit of eligibility for cooldown and rate limiting. Keep token ids stable once public clients depend on them; changing a token id creates a new enforcement scope.

Daily budgets remain configured per token through each token's `daily_budget_wei` value, with the legacy `SCAVIUM_FAUCET_DAILY_BUDGET_WEI` still serving the native/default compatibility path when token-specific budget metadata is not present.


## Phase 17.3 closure note

Phase 17.3 closes the configuration-driven token claim path without adding runtime configuration mutation. Token ids loaded from `SCAVIUM_FAUCET_TOKENS_JSON` now define transfer metadata, claim validation eligibility, cooldown/rate-limit scope, daily budget scope, and token-scoped runtime metrics. Keep token ids stable once published because changing an id creates a new claim/enforcement/metrics scope.

Changing token definitions still requires editing environment configuration and restarting the service. Hot reload, admin-driven token mutation, database-backed catalogs, and durable per-token analytics remain deferred.

## Frontend token selector behavior

The Phase 17.4.1 frontend does not introduce new environment variables. It derives its browser-visible token selector from the same public catalog generated from `SCAVIUM_FAUCET_TOKENS_JSON` and the configured default token.

Operational implications:

- Changing token registration still requires editing environment configuration and restarting the service.
- The frontend selector reflects whatever `GET /api/v1/tokens` returns after restart.
- If no valid token catalog can be loaded by the browser, the selector remains hidden and legacy/default-token claim behavior is preserved.
- Token labels are rendered from public metadata only: `id`, `symbol`, `type`, `decimals`, and `amount_wei`.
- Phase 17.4.2 additionally renders loading/fallback states and selected-token detail cards from that same metadata.
- Phase 17.4.3 additionally formats claim result summaries from returned claim metadata (`token_id`, `token_symbol`, `token_type`, `token_decimals`, and `amount_wei`) without introducing a frontend-specific configuration source.
- The frontend does not require a separate token-selector or claim-result configuration flag; catalog availability and claim responses remain the source of truth.

## Phase 17.4 closure note

Phase 17.4 closes the frontend-facing token selection behavior without adding frontend-specific configuration. The embedded UI continues to derive selector options from `GET /api/v1/tokens`, which is generated from `SCAVIUM_FAUCET_TOKENS_JSON` and the configured default token. Token configuration changes therefore still require environment updates and service restart before the browser-visible catalog and claim-result presentation reflect the new metadata.

## Phase 17 closure note

Phase 17 closes token support around startup-loaded configuration. `SCAVIUM_FAUCET_TOKENS_JSON` and `SCAVIUM_FAUCET_DEFAULT_TOKEN_ID` define the native/ERC20 faucet catalog, default fallback behavior, claim-time token validation, token-scoped enforcement, token-aware metrics, and the browser-visible selector metadata. Changing token definitions still requires updating environment configuration and restarting the service; runtime mutation and database-backed catalogs remain outside this phase.

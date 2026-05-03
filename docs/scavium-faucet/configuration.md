# Configuration Reference

All configuration is loaded from the process environment at startup.
No configuration file is read — use an `EnvironmentFile` in the systemd unit to pass secrets.

---

## Core

| Variable | Default | Description |
|---|---|---|
| `SCAVIUM_FAUCET_BIND_ADDR` | `127.0.0.1:18080` | TCP address the HTTP server listens on. Keep on loopback; let nginx terminate TLS. |
| `SCAVIUM_FAUCET_PUBLIC_BASE_URL` | `http://127.0.0.1:18080` | Public HTTPS URL of the faucet (used in responses and CORS). |
| `SCAVIUM_FAUCET_MODE` | `active` | Operational mode: `active`, `paused`, or `maintenance`. |
| `SCAVIUM_FAUCET_DRY_RUN` | `true` | When `true`, transactions are validated but not broadcast. Safe default for development. |

---

## Network

| Variable | Default | Description |
|---|---|---|
| `SCAVIUM_FAUCET_RPC_URL` | `http://127.0.0.1:18545` | Besu JSON-RPC endpoint. |
| `SCAVIUM_FAUCET_CHAIN_ID` | `31337` | EVM chain ID. Must match the connected network. |
| `SCAVIUM_FAUCET_NETWORK_NAME` | `scavium-dev` | Human-readable network name shown in the UI and API. |
| `SCAVIUM_FAUCET_SYMBOL` | `SCAV` | Token symbol shown in the UI and API. |
| `SCAVIUM_FAUCET_EXPLORER_TX_URL` | _(empty)_ | URL prefix for transaction links, e.g. `https://explorer.scavium.io/tx/`. |

---

## Wallet & transactions

| Variable | Default | Description |
|---|---|---|
| `SCAVIUM_FAUCET_PRIVATE_KEY` | _(required in production)_ | Hex-encoded private key of the faucet wallet. Never commit. Never log. |
| `SCAVIUM_FAUCET_AMOUNT_WEI` | `1000000000000000000` (1 SCAV) | Amount sent per claim, in wei. |

---

## Rate limits & cooldowns

| Variable | Default | Description |
|---|---|---|
| `SCAVIUM_FAUCET_COOLDOWN_SECONDS` | `86400` (24 h) | Minimum time between claims from the same address. |
| `SCAVIUM_FAUCET_RATE_LIMIT_IP_PER_HOUR` | `10` | Maximum claim attempts from the same IP per hour. |
| `SCAVIUM_FAUCET_RATE_LIMIT_ADDR_PER_DAY` | `3` | Maximum successful claims from the same address per day. |
| `SCAVIUM_FAUCET_DAILY_BUDGET_WEI` | _(unlimited)_ | Total wei disbursable per day. Faucet auto-pauses when reached. |

---

## Captcha

| Variable | Default | Description |
|---|---|---|
| `SCAVIUM_FAUCET_CAPTCHA_PROVIDER` | `disabled` | Provider: `disabled`, `dev`, `hcaptcha`, `recaptcha`, `turnstile`. |
| `SCAVIUM_FAUCET_CAPTCHA_SECRET` | _(empty)_ | Server-side secret for the chosen captcha provider. Never log. |
| `SCAVIUM_FAUCET_CAPTCHA_VERIFY_URL` | _(provider default)_ | Override the captcha verification URL (useful for testing). |

`dev` provider accepts any non-empty token — use it in CI and local dev only.

---

## Security

| Variable | Default | Description |
|---|---|---|
| `SCAVIUM_FAUCET_TRUSTED_PROXY` | _(empty)_ | CIDR or IP of the trusted reverse proxy. Required for correct real-IP extraction when behind nginx. Example: `127.0.0.1`. |
| `SCAVIUM_FAUCET_ADMIN_TOKEN` | _(empty)_ | Bearer token required for all `/api/v1/admin/*` endpoints. If empty, admin API is disabled. Never log. |

---

## Example EnvironmentFile

```ini
# /etc/scavium-faucet/env  (chmod 640, owner root:scavium-faucet)

SCAVIUM_FAUCET_BIND_ADDR=127.0.0.1:18080
SCAVIUM_FAUCET_PUBLIC_BASE_URL=https://faucet.scavium.io
SCAVIUM_FAUCET_RPC_URL=http://127.0.0.1:18545
SCAVIUM_FAUCET_CHAIN_ID=1337
SCAVIUM_FAUCET_NETWORK_NAME=scavium-testnet
SCAVIUM_FAUCET_SYMBOL=SCAV
SCAVIUM_FAUCET_EXPLORER_TX_URL=https://explorer.scavium.io/tx/

SCAVIUM_FAUCET_PRIVATE_KEY=<hex-encoded-key>
SCAVIUM_FAUCET_AMOUNT_WEI=1000000000000000000
SCAVIUM_FAUCET_COOLDOWN_SECONDS=86400
SCAVIUM_FAUCET_RATE_LIMIT_IP_PER_HOUR=5
SCAVIUM_FAUCET_RATE_LIMIT_ADDR_PER_DAY=1
SCAVIUM_FAUCET_DAILY_BUDGET_WEI=100000000000000000000

SCAVIUM_FAUCET_CAPTCHA_PROVIDER=turnstile
SCAVIUM_FAUCET_CAPTCHA_SECRET=<turnstile-secret>

SCAVIUM_FAUCET_TRUSTED_PROXY=127.0.0.1
SCAVIUM_FAUCET_ADMIN_TOKEN=<random-high-entropy-token>

SCAVIUM_FAUCET_DRY_RUN=false
SCAVIUM_FAUCET_MODE=active
```

> **Important:** `chmod 640` and set ownership to `root:scavium-faucet` so only the service user can read the file.

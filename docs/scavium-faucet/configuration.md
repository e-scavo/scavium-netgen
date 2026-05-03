# Configuration reference

Configuration is loaded from environment variables at startup via `internal/config`. The table below distinguishes between values that affect the shipped binary today and values that are loaded for later stages but not yet wired into runtime behavior.

## Variables

| Variable | Default | Current effect |
|---|---|---|
| `SCAVIUM_FAUCET_BIND_ADDR` | `127.0.0.1:18080` | Used by `main.go` as the listen address |
| `SCAVIUM_FAUCET_PUBLIC_BASE_URL` | `http://127.0.0.1:18080` | Loaded and validated; not currently surfaced by the handler |
| `SCAVIUM_FAUCET_RPC_URL` | `http://127.0.0.1:18545` | Loaded and validated; not actively used because readiness checks are still stubs |
| `SCAVIUM_FAUCET_CHAIN_ID` | `31337` | Exposed by `/api/v1/config` |
| `SCAVIUM_FAUCET_NETWORK_NAME` | `scavium-dev` | Exposed by `/api/v1/status` and `/api/v1/config` |
| `SCAVIUM_FAUCET_SYMBOL` | `SCAV` | Exposed by `/api/v1/status` and `/api/v1/config` |
| `SCAVIUM_FAUCET_EXPLORER_TX_URL` | empty | Exposed by `/api/v1/config` |
| `SCAVIUM_FAUCET_AMOUNT_WEI` | `1000000000000000000` | Exposed by `/api/v1/config` and copied into created claims |
| `SCAVIUM_FAUCET_COOLDOWN_SECONDS` | `86400` | Exposed by `/api/v1/config` and address-status responses |
| `SCAVIUM_FAUCET_DRY_RUN` | `true` | Exposed by `/api/v1/status` and `/api/v1/config` |
| `SCAVIUM_FAUCET_RATE_LIMIT_IP_PER_HOUR` | `10` | Exposed by `/api/v1/config` and address-status responses; not enforced yet |
| `SCAVIUM_FAUCET_RATE_LIMIT_ADDR_PER_DAY` | `3` | Exposed by `/api/v1/config` and address-status responses; not enforced yet |
| `SCAVIUM_FAUCET_DAILY_BUDGET_WEI` | empty | Loaded but not enforced yet |
| `SCAVIUM_FAUCET_TRUSTED_PROXY` | empty | Loaded; `internal/iputil` exists, but the HTTP layer does not use it yet |
| `SCAVIUM_FAUCET_PRIVATE_KEY` | empty | Loaded for future send/signing work; not used by the shipped binary |
| `SCAVIUM_FAUCET_CAPTCHA_PROVIDER` | `disabled` | Loaded only; captcha verification is not wired into the public claim endpoint yet |
| `SCAVIUM_FAUCET_CAPTCHA_SECRET` | empty | Loaded only; not used today |
| `SCAVIUM_FAUCET_CAPTCHA_VERIFY_URL` | empty | Loaded only; not used today |
| `SCAVIUM_FAUCET_MODE` | `active` | Loaded only; the current public status endpoint still reports `active` from the in-memory read service |
| `SCAVIUM_FAUCET_ADMIN_TOKEN` | empty | Loaded by config, but `app.New` does not pass it into `httpapi.NewHandler`, so admin endpoints remain disabled in the shipped binary |

## Validation rules

`Config.Validate()` currently enforces:

- bind address must be non-empty
- public base URL must be non-empty
- RPC URL must be non-empty
- chain ID must be positive
- network name must be non-empty
- symbol must be non-empty
- amount wei must be positive
- cooldown seconds must be zero or positive

Notably, the current validator does **not** require a private key or admin token.

## Example environment

```ini
SCAVIUM_FAUCET_BIND_ADDR=127.0.0.1:18080
SCAVIUM_FAUCET_PUBLIC_BASE_URL=https://faucet.example.test
SCAVIUM_FAUCET_RPC_URL=http://127.0.0.1:18545
SCAVIUM_FAUCET_CHAIN_ID=1337
SCAVIUM_FAUCET_NETWORK_NAME=scavium-testnet
SCAVIUM_FAUCET_SYMBOL=SCAV
SCAVIUM_FAUCET_EXPLORER_TX_URL=https://explorer.example.test/tx/{txHash}

SCAVIUM_FAUCET_AMOUNT_WEI=1000000000000000000
SCAVIUM_FAUCET_COOLDOWN_SECONDS=86400
SCAVIUM_FAUCET_RATE_LIMIT_IP_PER_HOUR=10
SCAVIUM_FAUCET_RATE_LIMIT_ADDR_PER_DAY=3
SCAVIUM_FAUCET_DRY_RUN=true

SCAVIUM_FAUCET_TRUSTED_PROXY=127.0.0.1
SCAVIUM_FAUCET_MODE=active
SCAVIUM_FAUCET_ADMIN_TOKEN=replace-me
```

## Practical notes

- Keep secrets in an external environment file or service manager, not in the repository.
- Set `SCAVIUM_FAUCET_BIND_ADDR` to loopback and terminate TLS in a reverse proxy.
- Treat `SCAVIUM_FAUCET_ADMIN_TOKEN`, `SCAVIUM_FAUCET_PRIVATE_KEY`, and captcha secrets as sensitive even though the current binary does not use all of them yet.

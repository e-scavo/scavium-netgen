# Testnet token registration guide

Phase 17.2.2 documents the operator path for registering native and ERC20 testnet assets in the public faucet without changing the claim API contract. Token registration is configuration-driven: the faucet reads the token catalog at startup from `SCAVIUM_FAUCET_TOKENS_JSON`, validates it through `internal/config`, exposes claim-safe metadata through the public token catalog endpoints, and keeps `POST /api/v1/claim` backward-compatible through `SCAVIUM_FAUCET_DEFAULT_TOKEN_ID`.

This guide is intentionally operational. It does not introduce runtime admin mutation, database-driven token management, or a frontend selector. Those remain later Phase 17/18 concerns.

## Registration model

A registered faucet token is one entry in `SCAVIUM_FAUCET_TOKENS_JSON`.

| Field | Required | Notes |
|---|---:|---|
| `id` | yes | Stable client-facing identifier used as `token_id` in claims. Use lowercase, deployment-safe ids such as `native`, `scav`, or `scat`. |
| `symbol` | yes | Public display symbol returned by `GET /api/v1/tokens`. |
| `type` | yes | `native` for the chain gas token or `erc20` for ERC20 transfers. |
| `address` | ERC20 only | ERC20 contract address. Native tokens must omit it or leave it empty. |
| `decimals` | recommended | Public token decimals. Use the contract value for ERC20 tokens. Defaults to 18 when omitted or zero. |
| `amount_wei` | yes | Claim amount in base units. For an 18-decimal token, `1000000000000000000` means `1.0`. |
| `daily_budget_wei` | optional | Optional token-specific UTC-day distribution cap in base units. When omitted, the global `SCAVIUM_FAUCET_DAILY_BUDGET_WEI` fallback behavior remains available. |

The default token is selected by `SCAVIUM_FAUCET_DEFAULT_TOKEN_ID`. If a client omits `token_id`, the claim uses that default. This preserves the original faucet contract for existing clients.

## Example — native-only baseline

Use this mode when the faucet distributes only the SCAVIUM native testnet token:

```ini
SCAVIUM_FAUCET_SYMBOL=SCAV
SCAVIUM_FAUCET_AMOUNT_WEI=1000000000000000000
# SCAVIUM_FAUCET_DEFAULT_TOKEN_ID=native
# SCAVIUM_FAUCET_TOKENS_JSON=
```

With no token JSON configured, the runtime exposes a single backward-compatible native token derived from `SCAVIUM_FAUCET_SYMBOL` and `SCAVIUM_FAUCET_AMOUNT_WEI`.

## Example — native + ERC20 testnet token

Use one compact JSON line in the environment file. Do not split the value across multiple lines unless the service manager explicitly supports multiline environment values.

```ini
SCAVIUM_FAUCET_DEFAULT_TOKEN_ID=native
SCAVIUM_FAUCET_TOKENS_JSON=[{"id":"native","symbol":"SCAV","type":"native","decimals":18,"amount_wei":"1000000000000000000","daily_budget_wei":"100000000000000000000"},{"id":"scat","symbol":"SCAT","type":"erc20","address":"0x1111111111111111111111111111111111111111","decimals":18,"amount_wei":"25000000000000000000","daily_budget_wei":"2500000000000000000000"}]
```

Before enabling a real ERC20 entry, replace the placeholder address with the deployed token contract address on the same chain configured by `SCAVIUM_FAUCET_CHAIN_ID` and `SCAVIUM_FAUCET_RPC_URL`.

## Operator checklist

Before restart:

- [ ] Confirm the ERC20 contract is deployed on the configured SCAVIUM testnet chain.
- [ ] Confirm `decimals` from the token contract and convert `amount_wei` / `daily_budget_wei` using base units.
- [ ] Confirm the faucet signer wallet holds enough native SCAV for gas.
- [ ] Confirm the faucet signer wallet holds enough ERC20 balance for the configured claim amount and expected daily volume.
- [ ] Use unique token ids; do not change an existing id once clients or persisted claims depend on it.
- [ ] Keep `SCAVIUM_FAUCET_DEFAULT_TOKEN_ID` pointed at a configured token.
- [ ] Keep `SCAVIUM_FAUCET_PRIVATE_KEY`, captcha secrets, and admin tokens out of documentation and git.

After restart:

```bash
curl -sS https://faucet.testnet.scavium.network/api/v1/tokens
curl -sS https://faucet.testnet.scavium.network/api/v1/faucet/tokens
```

Expected shape:

```json
{
  "tokens": [
    {
      "id": "native",
      "symbol": "SCAV",
      "type": "native",
      "decimals": 18,
      "amount_wei": "1000000000000000000"
    },
    {
      "id": "scat",
      "symbol": "SCAT",
      "type": "erc20",
      "address": "0x1111111111111111111111111111111111111111",
      "decimals": 18,
      "amount_wei": "25000000000000000000"
    }
  ]
}
```

Then submit a test claim with an explicit token id:

```bash
curl -sS -X POST https://faucet.testnet.scavium.network/api/v1/claim \\
  -H 'Content-Type: application/json' \\
  -H 'Idempotency-Key: test-scat-claim-001' \\
  -d '{"address":"0x52908400098527886E0F7030069857D2E4169EE7","token_id":"scat","captcha_token":"<provider-token>"}'
```

For local or staging validation with the dev captcha provider, use the configured dev token according to the active captcha policy. Never enable dev captcha on the public production faucet.

## Troubleshooting

| Symptom | Likely cause | Action |
|---|---|---|
| Startup fails with duplicate token id | Two entries share the same `id` | Rename or remove one token entry and restart. |
| Startup fails because default token is not configured | `SCAVIUM_FAUCET_DEFAULT_TOKEN_ID` does not match any normalized token id | Point the default to an existing token id. |
| Startup fails for ERC20 address | ERC20 token entry has an empty or invalid `address` | Use the deployed contract address on the configured testnet chain. |
| `GET /api/v1/tokens` shows only native token | `SCAVIUM_FAUCET_TOKENS_JSON` is empty or not loaded by systemd | Check the environment file, daemon reload, and service restart. |
| ERC20 claim is queued but send fails | Faucet signer lacks ERC20 balance, gas, or the contract address is wrong | Check wallet balances, RPC chain id, token address, and worker logs. |
| Existing clients still receive native token | Expected behavior when `token_id` is omitted | Use explicit `token_id` for ERC20 claims; keep default native for backward compatibility. |

## Phase boundary

Phase 17.2 is closed for the current public testnet scope. Token registration is intentionally configuration-driven, loaded at startup, validated by runtime config parsing, and exposed through the public catalog endpoints for operator and frontend discovery.

The closure does not add frontend selection, runtime admin token creation, dynamic token mutation, hot reload, or database-backed token catalogs. Those remain explicitly outside this phase so the public claim contract, deployment model, and SQLite claim history stay stable while later phases harden token-aware validation and operator controls.

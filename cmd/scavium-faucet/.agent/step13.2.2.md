# Step 13.2.2 — Configuration documentation alignment

Modify:
docs/scavium-faucet/configuration.md

Add / update:

## New environment variables

- SCAVIUM_FAUCET_CORS_ALLOWED_ORIGINS
  - comma-separated exact origins
  - empty = disabled
  - "*" not allowed

- SCAVIUM_FAUCET_DAILY_BUDGET_WEI
  - max total distributed per UTC day
  - unset = unlimited

## Clarify existing:

- WORKER_ENABLED default true
- WATCHER auto-enabled in production
- DATABASE_PATH mandatory for persistence

## Add note:

Daily budget:
- enforced on queued + sent + confirmed
- reset at UTC midnight

Keep format intact.
Do not remove existing content.

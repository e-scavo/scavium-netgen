# Step 13.1.0 — Production hardening audit plan

## Recommended executor

Copilot Chat in VSCode.

## Goal

Read the current code and produce a hardening implementation plan. Do not edit files.

## Files to inspect

- `cmd/scavium-faucet/internal/httpapi/handler.go`
- `cmd/scavium-faucet/internal/httpapi/handler_test.go`
- `cmd/scavium-faucet/internal/faucet/persistent_service.go`
- `cmd/scavium-faucet/internal/faucet/persistent_service_test.go`
- `cmd/scavium-faucet/internal/domain/`
- `cmd/scavium-faucet/internal/config/config.go`
- `cmd/scavium-faucet/internal/config/config_test.go`
- `cmd/scavium-faucet/internal/store/sqlite/store.go`
- `cmd/scavium-faucet/internal/store/sqlite/store_test.go`
- `cmd/scavium-faucet/internal/abuse/`
- `cmd/scavium-faucet/internal/captcha/`
- `cmd/scavium-faucet/internal/app/app.go`
- current docs only if needed to identify known gaps

## Required checks

1. Identify all current service errors from claim creation.
2. Identify how HTTP maps each error.
3. Check whether CORS exists.
4. Check whether `DailyBudgetWei` is enforced.
5. Check whether request logging or minimal metrics exist.
6. Identify minimal tests needed.

## Required output

- Files read.
- Confirmed gaps.
- Proposed files to modify.
- Proposed exact order for `13.1.1` through `13.1.4`.
- Risks and what not to touch.

## Hard constraints

- Do not modify files.
- Do not read or use `cmd/scavium-faucet-v0`.
- Do not propose VPS/nginx/systemd/certbot work.

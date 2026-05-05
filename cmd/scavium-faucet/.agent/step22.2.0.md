# Step 22.2.0 — Wallet and Nonce Runtime Visibility

## Goal

Expose admin-safe wallet balance and nonce visibility for operators.

## Scope

- Add runtime/admin visibility fields only behind admin auth.
- Do not expose private keys or sensitive wallet internals.
- Include native and configured token balances only if feasible and safe.
- Add tests and docs.

## Validation

```bash
gofmt -w <go-files-changed>
go test ./cmd/scavium-faucet/internal/chain/... ./cmd/scavium-faucet/internal/httpapi/... ./cmd/scavium-faucet/internal/app/... -count=1 -timeout 300s
go test ./... -timeout 300s
make build -B
```

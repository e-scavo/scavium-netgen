# Step 3.1.0 — RPC, signer y sender abstractions

## Objetivo

Preparar blockchain real detrás de interfaces testeables.

## Implementar

- paquete `internal/chain`
- interfaces para:
  - chain ID
  - balance
  - nonce
  - gas price/fee policy mínima
  - send transaction
  - receipt
- signer cargado desde private key/keystore según config, con soporte inicial a private key solo para dev si se decide
- dry-run sender
- mocks/fakes para tests
- validación de chain ID

## Seguridad

No loguear private key. No agregar claves reales.

## Validación

```bash
gofmt -w cmd/scavium-faucet
go test ./...
go build ./cmd/scavium-faucet
```

## Git

```bash
git checkout main
git pull --ff-only
git checkout -b faucet/step3.1.0-chain-abstractions
git add cmd/scavium-faucet
git commit -m "faucet: step 3.1.0 add chain abstractions"
git checkout main
git merge --no-ff faucet/step3.1.0-chain-abstractions
git branch -d faucet/step3.1.0-chain-abstractions
```

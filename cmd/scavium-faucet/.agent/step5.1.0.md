# Step 5.1.0 — Integración SCAVIUM Wallet

## Objetivo

Completar endpoints y flujo para que SCAVIUM Wallet consuma el faucet directamente por HTTPS.

## Implementar

- endpoints wallet-friendly definitivos:
  - `GET /api/v1/faucet/status`
  - `GET /api/v1/faucet/config`
  - `GET /api/v1/faucet/address/{address}/eligibility`
  - `POST /api/v1/faucet/claim`
  - `GET /api/v1/faucet/claim/{id}`
- challenge/firma opcional para probar control de address
- respuesta compacta para app: elegibilidad, cooldown restante, límites, tracking ID, estado, txHash
- tests de compatibilidad con endpoints públicos existentes

## Criterio

La wallet no usa RPC directo para pedir fondos. Siempre usa API del faucet.

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
git checkout -b faucet/step5.1.0-wallet-api
git add cmd/scavium-faucet
git commit -m "faucet: step 5.1.0 add wallet integration api"
git checkout main
git merge --no-ff faucet/step5.1.0-wallet-api
git branch -d faucet/step5.1.0-wallet-api
```
